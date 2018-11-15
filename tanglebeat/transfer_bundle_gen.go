package main

import (
	"errors"
	"fmt"
	"github.com/iotaledger/iota.go/address"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat1/bundle_source"
	"github.com/lunfardo314/tanglebeat1/lib"
	"github.com/op/go-logging"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"time"
)

// generates sequence of transfers of full balance from address idx to idx+1
// next bundle will be only generated upon current confirmed
// it is sequence of transfers along addresses with 0,1,2,3 ..indices of the same seed

type transferBundleGenerator struct {
	name          string
	params        *senderParamsYAML
	seed          trinary.Trytes
	securityLevel int
	txTag         trinary.Trytes
	index         uint64
	iotaAPI       *api.API
	iotaAPIgTTA   *api.API
	iotaAPIaTT    *api.API
	log           *logging.Logger
	chanOut       bundle_source.BundleSourceChan
}

const UID_LEN = 12

func NewTransferBundleGenerator(name string, params *senderParamsYAML, logger *logging.Logger) (*bundle_source.BundleSourceChan, error) {
	genState, err := initTransferBundleGenerator(name, params, logger)
	if err != nil {
		return nil, err
	}
	go genState.runGenerator()
	return &genState.chanOut, nil
}

func initTransferBundleGenerator(name string, params *senderParamsYAML, logger *logging.Logger) (*transferBundleGenerator, error) {
	var err error
	var ret = transferBundleGenerator{
		name:          name,
		params:        params,
		securityLevel: 2,
		log:           logger,
		chanOut:       make(bundle_source.BundleSourceChan),
	}
	// TODO specify timeout
	ret.iotaAPI, err = api.ComposeAPI(
		api.HTTPClientSettings{URI: params.IOTANode},
	)
	if err != nil {
		return nil, err
	}
	// Timeout: time.Duration(params.TimeoutAPI) * time.Second,

	AEC.registerAPI(ret.iotaAPI, params.IOTANode)

	ret.iotaAPIgTTA, err = api.ComposeAPI(
		api.HTTPClientSettings{URI: params.IOTANodeTipsel},
	)
	//		Timeout: time.Duration(params.TimeoutTipsel) * time.Second,
	if err != nil {
		return nil, err
	}

	AEC.registerAPI(ret.iotaAPIgTTA, params.IOTANodeTipsel)

	ret.iotaAPIaTT, err = api.ComposeAPI(
		api.HTTPClientSettings{URI: params.IOTANodePoW},
	)
	//		Timeout: time.Duration(params.TimeoutPoW) * time.Second,
	if err != nil {
		return nil, err
	}

	AEC.registerAPI(ret.iotaAPIaTT, params.IOTANodePoW)

	ret.seed = trinary.Trytes(params.Seed)
	err = trinary.ValidTrytes(ret.seed)
	if err != nil {
		return nil, err
	}

	ret.txTag = trinary.Trytes(params.TxTag)
	err = trinary.ValidTrytes(ret.txTag)
	if err != nil {
		return nil, err
	}
	// load index0
	fname := ret.getLastIndexFname()
	var idx uint64
	var _idx int
	b, _ := ioutil.ReadFile(fname)
	_idx, err = strconv.Atoi(string(b))
	if err != nil {
		idx = 0
		ret.log.Infof("Starting from index 0")
	} else {
		idx = uint64(_idx)
		ret.log.Infof("Last idx %v have been read from %v", ret.index, fname)
	}

	if idx > params.Index0 {
		ret.index = idx
	} else {
		ret.index = params.Index0
	}

	ret.log.Infof("Created transfer bundle generator ('traveling iota') transfer generator instance with UID = %v", params.GetUID())
	return &ret, nil
}

// main generating loop
func (gen *transferBundleGenerator) runGenerator() {
	var addr trinary.Hash
	var spent bool
	var balance uint64
	var err error
	var bundleData *bundle_source.FirstBundleData
	var isNew bool
	// error count will be used to stop producing bundles if exceeds SeqRestartAfterErr
	var errorCount uint64

	defer gen.log.Debugf("Leaving Transfer BundleTrytes Generator routine")

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
			var b *api.Balances
			if b, err = gen.getBalanceAddr(trinary.Hashes{addr}); err == nil {
				balance = b.Balances[0]
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
			gen.log.Infof("Transfer Bundles: '%v' index = %v is 'used'. Moving to the next",
				gen.name, gen.index)
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
			// have to parse first transaction to get the bundle hash
			tx0, err := transaction.AsTransactionObject(bundleData.BundleTrytes[0])
			if err != nil {
				gen.log.Errorf("Transfer Bundles: AsTransactionObject returned: %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			// ---------------------- send bundle to confirm
			bundleData.Addr = addr
			bundleData.Index = gen.index
			bundleData.IsNew = isNew
			bundleData.BundleHash = tx0.Bundle
			gen.chanOut <- bundleData /// here blocks until picked up in the sequence
			// ---------------------- send bundle to confirm

			errorCount = 0 // so far everything seems to be ok --> reset error count

			gen.log.Debugf("Transfer Bundles: send bundle to confirmer and wait until bundle hash %v confirmed. idx = %v",
				bundleData.BundleHash, gen.index)

			// wait until any transaction with the bundle hash becomes confirmed
			// note, that during rettach transaction can change but the bundle hash remains the same
			// returns 0 if error count didn't exceed limit
			errorCount = gen.waitUntilBundleConfirmed(bundleData.BundleHash)
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
				gen.log.Debugf("Transfer Bundles: moving '%v' to the next index -> %v", gen.name, gen.index)
			}
		}
	}
}

// returns 0 if error count doesn't reach limit

const sleepEveryLoop = 5 * time.Second

func (gen *transferBundleGenerator) waitUntilBundleConfirmed(bundleHash trinary.Hash) uint64 {
	gen.log.Debugf("waitUntilBundleConfirmed: start waiting for the bundle to be confirmed")

	startWaiting := time.Now()
	count := 0
	var sinceWaiting time.Duration
	var errorCount uint64

	for confirmed := false; !confirmed; count++ {
		if gen.params.SeqRestartAfterErr > 0 && errorCount >= gen.params.SeqRestartAfterErr {
			return errorCount
		}
		time.Sleep(sleepEveryLoop)
		sinceWaiting = time.Since(startWaiting)
		if count%5 == 0 {
			gen.log.Debugf("waitUntilBundleConfirmed: time since waiting: %v", sinceWaiting)
		}
		txHashes, err := gen.iotaAPI.FindTransactions(api.FindTransactionsQuery{
			Bundles: trinary.Hashes{bundleHash},
		})
		if err != nil {
			gen.log.Errorf("waitUntilBundleConfirmed: FindTransactions returned: %v. Time since waiting: %v",
				err, sinceWaiting)
			AEC.IncErrorCount(gen.iotaAPI)
			errorCount += 1
			continue
		}

		states, err := gen.iotaAPI.GetLatestInclusion(txHashes)
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

func (gen *transferBundleGenerator) isSpentAddr(address trinary.Hash) (bool, error) {
	if spent, err := gen.iotaAPI.WereAddressesSpentFrom(address); err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return false, err
	} else {
		return spent[0], nil
	}
}

func (gen *transferBundleGenerator) getBalanceAddr(addresses trinary.Hashes) (*api.Balances, error) {
	if balances, err := gen.iotaAPI.GetBalances(addresses, 100); err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	} else {
		return balances, nil
	}
}

func (gen *transferBundleGenerator) getAddress(index uint64) (trinary.Hash, error) {
	return address.GenerateAddress(gen.seed, index, securityLevel)
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

func (gen *transferBundleGenerator) sendBalance(fromAddr, toAddr trinary.Trytes, balance uint64,
	seed trinary.Trytes, fromIndex uint64) (*bundle_source.FirstBundleData, error) {
	// fromIndex is required to calculate inputs, cant specifiy inputs explicitely to PrepareTransfers
	ret := &bundle_source.FirstBundleData{
		NumAttach: 1,
		StartTime: lib.UnixMs(time.Now()),
	}
	//------ prepare transfer
	transfers := bundle.Transfers{{
		Address: toAddr,
		Value:   balance,
		Tag:     gen.txTag,
	}}
	inputs := []api.Input{{
		Address:  fromAddr,
		Security: securityLevel,
		KeyIndex: fromIndex,
		Balance:  balance,
	}}
	ts := lib.UnixMs(time.Now())
	prepTransferOptions := api.PrepareTransfersOptions{
		Inputs:    inputs,
		Timestamp: &ts,
	}
	// signing is here
	bundleRet, err := gen.iotaAPI.PrepareTransfers(seed, transfers, prepTransferOptions)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	//----- end prepare transfer

	//------ find two transactions to approve
	st := lib.UnixMs(time.Now())
	gttaResp, err := gen.iotaAPIgTTA.GetTransactionsToApprove(3)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPIgTTA)
		return nil, err
	}
	ret.TotalDurationTipselMs = lib.UnixMs(time.Now()) - st

	st = lib.UnixMs(time.Now())

	// do POW by calling attachToTangle
	bundleRet, err = gen.iotaAPIaTT.AttachToTangle(
		gttaResp.TrunkTransaction,
		gttaResp.BranchTransaction,
		14,
		bundleRet,
	)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPIaTT)
		return nil, err
	}

	ret.TotalDurationPoWMs = lib.UnixMs(time.Now()) - st
	// breatcast bundle
	_, err = gen.iotaAPI.BroadcastTransactions(bundleRet...)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	_, err = gen.iotaAPI.StoreTransactions(bundleRet...)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	ret.BundleTrytes = bundleRet
	return ret, nil
}

func (gen *transferBundleGenerator) sendToNext(addr trinary.Hash) (*bundle_source.FirstBundleData, error) {
	nextAddr, err := gen.getAddress(gen.index + 1)
	if err != nil {
		return nil, err
	}
	gen.log.Debugf("Inside sendToNext with tag = '%v'. idx=%v. %v --> %v", gen.txTag, gen.index, addr, nextAddr)

	gbResp, err := gen.iotaAPI.GetBalances(trinary.Hashes{addr}, 100)
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

func (gen *transferBundleGenerator) findBundleToConfirm(addr trinary.Hash) (*bundle_source.FirstBundleData, error) {
	// find all transactions of the address
	txs, err := gen.findTransactionObjects(
		api.FindTransactionsQuery{
			Addresses: trinary.Hashes{addr},
		},
	)
	if err != nil {
		return nil, err
	}
	// filter out spending transactions, collect set of bundles of those transactions
	// note that bundle hashes can be more than one in rare cases
	var spendingBundleHashes trinary.Hashes
	for _, tx := range txs {
		if tx.Value < 0 && !lib.TrytesInSet(tx.Bundle, spendingBundleHashes) {
			spendingBundleHashes = append(spendingBundleHashes, tx.Bundle)
		}
	}
	if len(spendingBundleHashes) == 0 {
		return nil, nil // no error, empty bundle
	}

	//find all transactions, belonging to spending bundles
	txs, err = gen.findTransactionObjects(
		api.FindTransactionsQuery{
			Bundles: spendingBundleHashes,
		},
	)

	// collect all tails
	var tails []*transaction.Transaction
	for _, tx := range txs {
		if transaction.IsTailTransaction(&tx) {
			tails = append(tails, &tx)
		}
	}
	// collect hashes of tails
	var tailHashes trinary.Hashes
	for _, tail := range tails {
		tailHashes = append(tailHashes, tail.Hash)
	}
	// check if ANY of tails are confirmed
	var confirmed bool
	confirmed, err = gen.isAnyConfirmed(tailHashes)
	if err != nil {
		return nil, err
	}
	if confirmed {
		return nil, nil // already confirmed
	}

	// none is confirmed hence no matter which one
	// select the oldest one by Timestamp
	// TODO  use attachmentTimestamp instead
	maxTime := tails[0].Timestamp
	minTime := tails[0].Timestamp
	maxTail := tails[0]
	for _, tail := range tails {
		if tail.Timestamp > maxTime {
			maxTime = tail.Timestamp
			maxTail = tail
		}
		if tail.Timestamp < minTime {
			minTime = tail.Timestamp
		}
	}
	// collect the bundle by maxTail
	txSet := extractBundleTxByTail(maxTail, txs)

	txSet, err = lib.CheckAndSortTxSetAsBundle(txSet)

	if err != nil {
		// report but don't rise the error about bundle inconsistency,
		// because it come from the node
		return nil, errors.New(fmt.Sprintf("Inconsistency of a spending bundle in addr = %v: %v", addr, err))
	}
	var bundleTrytes []trinary.Trytes
	bundleTrytes, err = lib.TransactionSetToBundleTrytes(txSet)
	if err != nil {
		return nil, err
	}
	ret := &bundle_source.FirstBundleData{
		BundleTrytes: bundleTrytes,
		NumAttach:    uint64(len(tails)),
		StartTime:    minTime,
	}
	return ret, nil
}

// give the tail and transaction set, filers out from the set the bundle of that tail
// checks consistency of the bundle. Sorts it by index

func extractBundleTxByTail(tail *transaction.Transaction, allTx []transaction.Transaction) []*transaction.Transaction {
	lib.Assert(tail.CurrentIndex == 0, "tail.CurrentIndex == 0", nil)
	// first in a bundle is tail tx
	var ret []*transaction.Transaction
	tx := tail
	count := 0
	// counting and capping steps to avoid eternal loops along trunk (impossible, I know)
	for {
		if count >= len(allTx) {
			return ret
		}
		ret = append(ret, tx)
		var ok bool
		tx, ok = lib.FindTxByHash(tx.TrunkTransaction, allTx)
		if !ok {
			return ret
		}
		count++
	}
	return ret
}

func (gen *transferBundleGenerator) isAnyConfirmed(txHashes trinary.Hashes) (bool, error) {
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

func (gen *transferBundleGenerator) findTransactionObjects(query api.FindTransactionsQuery) (transaction.Transactions, error) {
	// TODO tx cache
	ftHashes, err := gen.iotaAPI.FindTransactions(query)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	rawTrytes, err := gen.iotaAPI.GetTrytes(ftHashes...)
	if err != nil {
		AEC.IncErrorCount(gen.iotaAPI)
		return nil, err
	}
	return transaction.AsTransactionObjects(rawTrytes, nil)
}
