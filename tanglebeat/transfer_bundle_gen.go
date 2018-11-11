package main

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/bundle_source"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

// generates sequence of transfers of full balance from address idx to idx+1
// next bundle will be only generated upon current confirmed
// it is sequence of transfers along addresses with 0,1,2,3 ..indices of the same seed

type transferBundleGenerator struct {
	params        *senderParamsYAML
	seed          giota.Trytes
	securityLevel int
	txTag         giota.Trytes
	index         int
	iotaAPI       *giota.API
	iotaAPIgTTA   *giota.API
	iotaAPIaTT    *giota.API
	log           *logging.Logger
	chanOut       bundle_source.BundleSourceChan
}

const UID_LEN = 12

func NewTransferBundleGenerator(params *senderParamsYAML, logger *logging.Logger) (*bundle_source.BundleSourceChan, error) {
	genState, err := initTransferBundleGenerator(params, logger)
	if err != nil {
		return nil, err
	}
	go genState.runGenerator()
	return &genState.chanOut, nil
}

func initTransferBundleGenerator(params *senderParamsYAML, logger *logging.Logger) (*transferBundleGenerator, error) {
	var err error
	var ret = transferBundleGenerator{
		params:        params,
		securityLevel: 2,
		log:           logger,
		chanOut:       make(bundle_source.BundleSourceChan),
	}
	ret.iotaAPI = giota.NewAPI(
		params.IOTANode,
		&http.Client{
			Timeout: time.Duration(params.TimeoutAPI) * time.Second,
		},
	)
	AEC.registerAPI(ret.iotaAPI, params.IOTANode)

	ret.iotaAPIgTTA = giota.NewAPI(
		params.IOTANodeTipsel,
		&http.Client{
			Timeout: time.Duration(params.TimeoutTipsel) * time.Second,
		},
	)
	AEC.registerAPI(ret.iotaAPIgTTA, params.IOTANodeTipsel)

	ret.iotaAPIaTT = giota.NewAPI(
		params.IOTANodePoW,
		&http.Client{
			Timeout: time.Duration(params.TimeoutPoW) * time.Second,
		},
	)
	AEC.registerAPI(ret.iotaAPIaTT, params.IOTANodePoW)

	ret.seed, err = giota.ToTrytes(params.Seed)
	if err != nil {
		return nil, err
	}

	ret.txTag, err = giota.ToTrytes(params.TxTag)
	if err != nil {
		return nil, err
	}
	// load index0
	fname := ret.getLastIndexFname()
	var idx int
	b, _ := ioutil.ReadFile(fname)
	idx, err = strconv.Atoi(string(b))
	if err != nil {
		idx = 0
		ret.log.Infof("Starting from index 0")
	} else {
		ret.log.Infof("Last idx %v have been read from %v", ret.index, fname)
	}

	ret.index = lib.Max(idx, params.Index0)

	ret.log.Infof("Created transfer bundle generator ('traveling iota') transfer generator instance with UID = %v", params.GetUID())
	return &ret, nil
}

// main generating loop
func (gen *transferBundleGenerator) runGenerator() {
	var addr giota.Address
	var spent bool
	var balance int64
	var err error
	var bundleData *bundle_source.FirstBundleData
	var isNew bool
	// error count will be used to stop producing bundles if exceeds SeqRestartAfterErr
	var errorCount int

	defer gen.log.Debugf("Leaving Transfer Bundle Generator routine")

	addr = ""
	balanceZeroWaitSec := 2
	for {
		if gen.params.SeqRestartAfterErr > 0 && errorCount >= gen.params.SeqRestartAfterErr {
			gen.log.Errorf("TransferBundles @ index = %v: error count limit of %d API errors exceed. Closing output channel",
				gen.index, gen.params.SeqRestartAfterErr)
			close(gen.chanOut)
			return //>>>>>>>>>>>>>>>>> leaving loop
		}
		if addr == "" {
			addr, err = gen.getAddress(gen.index)
			if err != nil {
				gen.log.Errorf("Can't get address index = %v: %v", gen.index, err)
				time.Sleep(5 * time.Second)
				continue
			}
		}
		if spent, err = gen.isSpentAddr(addr); err == nil {
			var b []int64
			if b, err = gen.getBalanceAddr([]giota.Address{addr}); err == nil {
				balance = b[0]
			}
		}
		if err != nil {
			gen.log.Errorf("idx = %v: %v", gen.index, err)
			time.Sleep(5 * time.Second)
			errorCount += 1
			continue
		}
		switch {
		case balance == 0 && spent:
			// used, just go to next
			if err = gen.saveIndex(); err != nil {
				gen.log.Errorf("Transfer Bundles: saveIndex: %v", err)
				time.Sleep(5 * time.Second)
				errorCount += 1
				continue
			}
			gen.log.Infof("Transfer Bundles: index = %v is 'used'. Moving to the next", gen.index)
			gen.index += 1
			addr = ""

		case balance == 0 && !spent:
			// nogo
			// loops until balance != 0
			gen.log.Infof("Transfer Bundles: index = %v, balance == 0, not spent. Wait %v sec for balance to become non zero",
				gen.index, balanceZeroWaitSec)
			time.Sleep(time.Duration(balanceZeroWaitSec) * time.Second)
			balanceZeroWaitSec = lib.Min(balanceZeroWaitSec+2, 15)

		case balance != 0:
			bundleData, err = gen.findBundleToConfirm(addr)
			if err != nil {
				gen.log.Errorf("Transfer Bundles: findBundleToConfirm returned: %v", err)
				time.Sleep(5 * time.Second)
				errorCount += 1
				continue
			}
			if bundleData == nil {
				// didn't find any ready to confirm, initialize new transfer
				bundleData, err = gen.sendToNext(addr)
				if err != nil {
					gen.log.Errorf("Transfer Bundles: sendToNext returned: %v", err)
					time.Sleep(5 * time.Second)
					errorCount += 1
					continue
				}
				isNew = true
				gen.log.Debugf("Transfer Bundles: Initialized new transfer idx=%v->%v, %v->",
					gen.index, gen.index+1, addr)
			} else {
				isNew = false
				gen.log.Debugf("Transfer Bundles: Found existing transfer to confirm idx=%v->%v, %v->",
					gen.index, gen.index+1, addr)
			}
			if bundleData == nil {
				gen.log.Errorf("Transfer Bundles: internal inconsistency. Wait 5 sec. idx = %v", gen.index)
				time.Sleep(5 * time.Second)
				errorCount += 1
				continue
			}
			// ---------------------- send bundle to confirm
			bundleData.Addr = addr
			bundleData.Index = gen.index
			bundleData.IsNew = isNew
			gen.chanOut <- bundleData /// here blocks until picked up in the sequence
			// ---------------------- send bundle to confirm

			errorCount = 0 // so far everything seems to be ok --> reset error count

			bhash := bundleData.Bundle.Hash()
			gen.log.Debugf("Transfer Bundles: send bundle to confirmer and wait until bundle hash %v confirmed. idx = %v", bhash, gen.index)

			// wait until any transaction with the bundle hash becomes confirmed
			// returns 0 if error count didn't exceed limit
			errorCount = gen.waitUntilBundleConfirmed(bhash)
			if errorCount == 0 {
				gen.log.Debugf("Transfer Bundles: bundle confirmed. idx = %v", gen.index)
				err = gen.saveIndex()
				if err != nil {
					gen.log.Errorf("Transfer Bundles: saveIndex: %v", err)
					time.Sleep(5 * time.Second)
				}
				// moving to next even if balance is still not zero (sometimes happens)
				// in latter case iotas will be left behind
				gen.index += 1
				addr = ""
				gen.log.Debugf("Transfer Bundles: moving to the next index -> %v", gen.index)
			}
		}
	}
}

// returns 0 if error count doesn't reach limit
func (gen *transferBundleGenerator) waitUntilBundleConfirmed(bundleHash giota.Trytes) int {
	gen.log.Debugf("waitUntilBundleConfirmed: start waiting for the bundle to be confirmed")

	startWaiting := time.Now()
	count := 0
	var sinceWaiting time.Duration
	errorCount := 0

	for confirmed := false; !confirmed; count++ {
		if errorCount >= gen.params.SeqRestartAfterErr {
			return errorCount
		}
		time.Sleep(2 * time.Second)
		sinceWaiting = time.Since(startWaiting)
		if count%5 == 0 {
			gen.log.Debugf("waitUntilBundleConfirmed: time since waiting: %v", sinceWaiting)
		}
		ftResp, err := gen.iotaAPI.FindTransactions(&giota.FindTransactionsRequest{
			Bundles: []giota.Trytes{bundleHash},
		})
		if err != nil {
			gen.log.Errorf("waitUntilBundleConfirmed: FindTransactions returned: %v. Time since waiting: %v",
				err, sinceWaiting)
			AEC.IncErrorCount(gen.iotaAPI)
			errorCount += 1
			continue
		}

		states, err := gen.iotaAPI.GetLatestInclusion(ftResp.Hashes)
		if err != nil {
			gen.log.Errorf("waitUntilBundleConfirmed: GetLatestInclusion returned: %v. Time since waiting: %v",
				err, sinceWaiting)
			AEC.IncErrorCount(gen.iotaAPI)
			errorCount += 1
			continue
		}
		for _, conf := range states {
			if conf {
				confirmed = true
			}
		}
		errorCount = 0 // reset error count
	}
	return 0
}

func (gen *transferBundleGenerator) isSpentAddr(address giota.Address) (bool, error) {
	if resp, err := gen.iotaAPI.WereAddressesSpentFrom([]giota.Address{address}); err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return false, err
	} else {
		return resp.States[0], nil
	}
}

func (gen *transferBundleGenerator) getBalanceAddr(addresses []giota.Address) ([]int64, error) {
	if gbResp, err := gen.iotaAPI.GetBalances(addresses, 100); err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	} else {
		return gbResp.Balances, nil
	}
}

func (gen *transferBundleGenerator) getAddress(index int) (giota.Address, error) {
	return giota.NewAddress(gen.seed, index, gen.securityLevel)
}

func (gen *transferBundleGenerator) getLastIndexFname() string {
	return path.Join(Config.siteDataDir, Config.Logging.WorkingSubdir, gen.params.GetUID())
}

func (gen *transferBundleGenerator) saveIndex() error {
	fname := gen.getLastIndexFname()
	fout, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer fout.Close()
	if _, err = fout.WriteString(fmt.Sprintf("%v", gen.index)); err == nil {
		gen.log.Debugf("Last idx %v saved to %v", gen.index, fname)
	}
	return err
}

func (gen *transferBundleGenerator) sendBalance(fromAddr, toAddr giota.Address, balance int64,
	seed giota.Trytes, fromIndex int) (*bundle_source.FirstBundleData, error) {
	// fromIndex is required to calculate inputs, cant specifiy inputs explicitely to PrepareTransfers
	ret := &bundle_source.FirstBundleData{
		NumAttach: 1,
		StartTime: time.Now(),
	}
	transfers := []giota.Transfer{
		{Address: toAddr,
			Value: balance,
			Tag:   gen.txTag,
		},
	}
	inputs := []giota.AddressInfo{
		{Seed: gen.seed, Index: fromIndex, Security: gen.securityLevel},
	}
	bundle, err := giota.PrepareTransfers(
		gen.iotaAPI,
		gen.seed,
		transfers,
		inputs,
		giota.Address(""),
		gen.securityLevel,
	)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	st := lib.UnixMs(time.Now())
	gttaResp, err := gen.iotaAPIgTTA.GetTransactionsToApprove(3, 100, giota.Trytes(""))
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPIgTTA)
		return nil, err
	}
	ret.TotalDurationTipselMs = lib.UnixMs(time.Now()) - st

	st = lib.UnixMs(time.Now())
	attResp, err := gen.iotaAPIaTT.AttachToTangle(&giota.AttachToTangleRequest{
		TrunkTransaction:   gttaResp.TrunkTransaction,
		BranchTransaction:  gttaResp.BranchTransaction,
		Trytes:             bundle,
		MinWeightMagnitude: 14,
	})
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPIaTT)
		return nil, err
	}

	ret.TotalDurationPoWMs = lib.UnixMs(time.Now()) - st

	err = gen.iotaAPI.BroadcastTransactions(attResp.Trytes)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	err = gen.iotaAPI.StoreTransactions(attResp.Trytes)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	ret.Bundle = attResp.Trytes
	return ret, nil
}

func (gen *transferBundleGenerator) sendToNext(addr giota.Address) (*bundle_source.FirstBundleData, error) {
	nextAddr, err := gen.getAddress(gen.index + 1)
	if err != nil {
		return nil, err
	}
	gen.log.Debugf("Inside sendToNext with tag = '%v'. idx=%v. %v --> %v", gen.txTag, gen.index, addr, nextAddr)

	gbResp, err := gen.iotaAPI.GetBalances([]giota.Address{addr}, 100)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	balance := gbResp.Balances[0]
	if balance == 0 {
		return nil, errors.New(fmt.Sprintf("Address %v has 0 balance, can't sent to the next.", addr))
	} else {
		gen.log.Infof("Sending %v i, idx = %v. %v --> %v",
			balance, gen.index, addr, nextAddr)
	}
	ret, err := gen.sendBalance(addr, nextAddr, balance, gen.seed, gen.index)
	if err != nil {
		return nil, err
	}
	// wait until address will acquire isSpent status
	spent, err := gen.isSpentAddr(addr)
	timeout := 10
	for count := 0; !spent && err == nil; count++ {
		if count > timeout {
			return nil, errors.New("!!!!!!!! Didn't get 'spent' state in 10 seconds")
		}
		time.Sleep(1 * time.Second)
		spent, err = gen.isSpentAddr(addr)
	}
	return ret, err
}

func (gen *transferBundleGenerator) findBundleToConfirm(addr giota.Address) (*bundle_source.FirstBundleData, error) {
	// find all transactions of the address
	ftResp, err := gen.findTrytes(
		&giota.FindTransactionsRequest{
			Addresses: []giota.Address{addr},
		},
	)
	if err != nil {
		return nil, err
	}
	// filter out spending transactions, collect set of bundles of those transactions
	// note that bundle hashes can be more than one in rare cases
	var spendingBundleHashes []giota.Trytes
	for _, tx := range ftResp.Trytes {
		if tx.Value < 0 && !lib.TrytesInSet(tx.Bundle, spendingBundleHashes) {
			spendingBundleHashes = append(spendingBundleHashes, tx.Bundle)
		}
	}
	if len(spendingBundleHashes) == 0 {
		return nil, nil // no error, empty bundle
	}

	//find all transactions, belonging to spending bundles
	ftResp, err = gen.findTrytes(
		&giota.FindTransactionsRequest{
			Bundles: spendingBundleHashes,
		},
	)
	spendingBundlesTx := ftResp.Trytes

	// collect all tails
	var tails []giota.Transaction
	for _, tx := range spendingBundlesTx {
		if tx.CurrentIndex == 0 {
			tails = append(tails, tx)
		}
	}
	// collect hashes of tails
	var tailHashes []giota.Trytes
	for _, tail := range tails {
		tailHashes = append(tailHashes, tail.Hash())
	}
	// check is ANY of tails are confirmed
	var confirmed bool
	confirmed, err = gen.isAnyConfirmed(tailHashes)
	switch {
	case err != nil:
		return nil, err
	case confirmed:
		return nil, nil // already confirmed
	}

	// none is confirmed hence no matter which one
	// select the oldest one by Timestamp
	// TODO  use attachmentTimestamp instead
	maxTime := tails[0].Timestamp
	minTime := tails[0].Timestamp
	maxTail := tails[0]
	for _, tail := range tails {
		if tail.Timestamp.After(maxTime) {
			maxTime = tail.Timestamp
			maxTail = tail
		}
		if tail.Timestamp.Before(minTime) {
			minTime = tail.Timestamp
		}
	}
	// collect the bundle by maxTail
	bundleTx := extractBundleTxByTail(&maxTail, spendingBundlesTx)
	bundleTx, err = lib.CheckAndSortBundle(bundleTx)

	if err != nil {
		// report but don't rise the error about bundle inconsistency,
		// because it come from the node
		return nil, errors.New(fmt.Sprintf("Inconsistency of a spending bundle in addr = %v: %v", addr, err))
	}
	ret := &bundle_source.FirstBundleData{
		Bundle:    bundleTx,
		NumAttach: int64(len(tails)),
		StartTime: minTime,
	}
	return ret, nil
}

// give the tail and transaction set, filers out from the set the bundle of that tail
// checks consistency of the bundle. Sorts it by index

func extractBundleTxByTail(tail *giota.Transaction, allTx []giota.Transaction) []giota.Transaction {
	lib.Assert(tail.CurrentIndex == 0, "tail.CurrentIndex == 0", nil)
	// first in a bundle is tail tx
	var ret []giota.Transaction
	tx := tail
	count := 0
	// counting and capping steps to avoid eternal loops along trunk (impossible, I know)
	for {
		if count >= len(allTx) {
			return ret
		}
		ret = append(ret, *tx)
		var ok bool
		tx, ok = lib.FindTxByHash(tx.TrunkTransaction, allTx)
		if !ok {
			return ret
		}
		count++
	}
	return ret
}

func (gen *transferBundleGenerator) isAnyConfirmed(txHashes []giota.Trytes) (bool, error) {
	incl, err := gen.iotaAPI.GetLatestInclusion(txHashes)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return false, err
	}
	for _, ok := range incl {
		if ok {
			return true, nil
		}
	}
	return false, nil
}

func (gen *transferBundleGenerator) findTrytes(txReq *giota.FindTransactionsRequest) (*giota.GetTrytesResponse, error) {
	// TODO tx cache
	ftResp, err := gen.iotaAPI.FindTransactions(txReq)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	gtResp, err := gen.iotaAPI.GetTrytes(ftResp.Hashes)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	return gtResp, nil
}
