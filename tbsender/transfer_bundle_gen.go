package main

import (
	"errors"
	"fmt"
	. "github.com/iotaledger/iota.go/address"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/bundle"
	. "github.com/iotaledger/iota.go/consts"
	. "github.com/iotaledger/iota.go/guards/validators"
	. "github.com/iotaledger/iota.go/transaction"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/bundle_source"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/multiapi"
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
	seed          Trytes
	securityLevel int
	txTag         Trytes
	index         uint64

	iotaMultiAPI     multiapi.MultiAPI
	iotaMultiAPIgTTA multiapi.MultiAPI
	iotaMultiAPIaTT  multiapi.MultiAPI

	log     *logging.Logger
	chanOut bundle_source.BundleSourceChan
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
	// default multi API
	ret.iotaMultiAPI, err = multiapi.New(params.IOTANode, params.TimeoutAPI)
	if err != nil {
		return nil, err
	}

	// multi APi for tipsel
	ret.iotaMultiAPIgTTA, err = multiapi.New(params.IOTANodeTipsel, params.TimeoutTipsel)
	if err != nil {
		return nil, err
	}

	ret.iotaMultiAPIaTT, err = multiapi.New([]string{params.IOTANodePoW}, params.TimeoutPoW)
	if err != nil {
		return nil, err
	}

	ret.seed = Trytes(params.Seed)
	ret.txTag = Pad(Trytes(params.TxTag), TagTrinarySize/3)
	err = Validate(ValidateSeed(ret.seed), ValidateTags(ret.txTag))
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
		ret.log.Infof("%v: Starting from index = 0", name)
	} else {
		idx = uint64(_idx)
		ret.log.Infof("%v: last idx %v have been read from %v", name, idx, fname)
	}

	if idx > params.Index0 {
		ret.index = idx
	} else {
		ret.index = params.Index0
	}

	ret.log.Infof("%v: created transfer bundle generator ('traveling iota')", name)
	return &ret, nil
}

func (gen *transferBundleGenerator) CheckBalance(addr Hash) (bool, uint64, error) {
	spent, err := gen.isSpentAddr(addr)
	if err != nil {
		return false, 0, err
	}
	var b *Balances
	b, err = gen.getBalanceAddr(Hashes{addr})
	if err != nil {
		return false, 0, err
	}
	return spent, b.Balances[0], nil
}

// main generating loop
func (gen *transferBundleGenerator) runGenerator() {
	var addr Hash
	var spent bool
	var balance uint64
	var err error
	var bundleData *bundle_source.FirstBundleData
	var isNew bool
	// error count will be used to stop producing bundles if exceeds SeqRestartAfterErr
	var errorCount uint64

	defer gen.log.Debugf("%v: leaving Transfer BundleTrytes Generator routine", gen.name)

	addr = ""
	balanceZeroWaitSec := 2

	for {
		if addr == "" {
			addr, err = gen.getAddress(gen.index)
			if err != nil {
				gen.log.Panicf("%v: can't get address index = %v: %v", gen.name, gen.index, err)
				continue
			}
		}
		spent, balance, err = gen.CheckBalance(addr)
		if err != nil {
			gen.log.Errorf("%v[%v]: CheckBalance returned %v", gen.name, gen.index, err)
			time.Sleep(5 * time.Second)
			errorCount += 1
			continue
		}
		switch {
		case balance == 0 && spent:
			// used, just go to next
			gen.saveIndex()
			gen.log.Infof("Transfer Bundles: '%v' index = %v is 'used'. Moving to the next", gen.name, gen.index)
			gen.index += 1
			addr = ""
			continue

		case balance == 0 && !spent:
			// nogo
			// will loop until balance != 0
			gen.log.Infof("Transfer Bundles: '%v' index = %v, balance == 0, not spent. Wait %v sec for balance to become non zero",
				gen.name, gen.index, balanceZeroWaitSec)
			time.Sleep(time.Duration(balanceZeroWaitSec) * time.Second)
			balanceZeroWaitSec = lib.Min(balanceZeroWaitSec+2, 15)
			continue

		case balance != 0:
			bundleData, err = gen.findBundleToConfirm(addr)
			if err != nil {
				gen.log.Errorf("Transfer Bundles '%v': findBundleToConfirm returned: %v", gen.name, err)
				time.Sleep(5 * time.Second)
				errorCount += 1
				continue
			}
			if bundleData == nil {
				// didn't find any ready to confirm, initialize new transfer
				bundleData, err = gen.sendToNext(addr)
				if err != nil {
					gen.log.Errorf("Transfer Bundles: '%v' sendToNext returned: %v", gen.name, err)
					time.Sleep(5 * time.Second)
					errorCount += 1
					continue
				}
				isNew = true
				gen.log.Debugf("Transfer Bundles: '%v' Initialized new transfer idx=%v->%v, %v->",
					gen.name, gen.index, gen.index+1, addr)
			} else {
				isNew = false
				gen.log.Debugf("Transfer Bundles '%v': Found existing transfer to confirm idx=%v->%v, %v->",
					gen.name, gen.index, gen.index+1, addr)
			}
			if bundleData == nil {
				gen.log.Errorf("Transfer Bundles '%v': internal inconsistency. Wait 5 sec. idx = %v",
					gen.name, gen.index)
				time.Sleep(5 * time.Second)
				errorCount += 1
				continue
			}
			// have to parse first transaction to get the bundle hash
			tail, err := lib.TailFromBundleTrytes(bundleData.BundleTrytes)
			if err != nil {
				gen.log.Errorf("Transfer Bundles: '%v' AsTransactionObject returned: %v", gen.name, err)
				time.Sleep(5 * time.Second)
				continue
			}
			bundleHash := tail.Bundle
			gen.log.Debugf("Transfer Bundles: '%v': Sending bundle to confirm. bundle hash: %v tail: %v",
				gen.name, bundleHash, tail.Hash)
			// ---------------------- send bundle to confirm
			// start stopwatch for the bundle. The creation of the originating bundle
			// won't be included into overall duration
			confirmer.StartStopwatch(bundleHash)

			bundleData.Addr = addr
			bundleData.Index = gen.index
			bundleData.IsNew = isNew
			gen.chanOut <- bundleData /// here blocks until picked up in the sequence
			// ---------------------- send bundle to confirm
			gen.log.Debugf("Transfer Bundles: '%v'[%v] just sent bundle hash = %v to confirmer and wait until transfer confirmed",
				gen.name, gen.index, bundleHash)

			errorCount = 0 // so far everything seems to be ok --> reset error count

			// wait until any transaction with the bundle hash becomes confirmed
			// note, that during rettach transaction can change but the bundle hash remains the same
			// returns 0 if error count didn't exceed limit
			gen.waitUntilBundleConfirmed(bundleHash)

			gen.log.Debugf("Transfer Bundles: '%v'[%v] bundle %v confirmed", gen.name, gen.index, bundleHash)
			gen.saveIndex()
			// moving to next even if balance is still not zero (sometimes happens)
			// in latter case iotas will be left behind
			gen.index += 1
			addr = ""
			gen.log.Debugf("Transfer Bundles: moving '%v' to the next index -> %v", gen.name, gen.index)
		}
	}
}

// returns 0 if error count doesn't reach limit

func (gen *transferBundleGenerator) waitUntilBundleConfirmed(bundleHash Hash) {
	gen.log.Debugf("waitUntilBundleConfirmed: '%v' start waiting for the bundle to be confirmed", gen.name)
	defer gen.log.Debugf("waitUntilBundleConfirmed: '%v' %v left", gen.name, bundleHash)

	confirmer.WaitfForConfirmation(bundleHash, gen.iotaMultiAPI, gen.log, AEC)
}

//const sleepEveryLoop = 5 * time.Second
//
//// loops until confirmation or until failure
//// returns only upon success
//func (gen *transferBundleGenerator) waitUntilBundleConfirmed(bundleHash Hash) {
//	gen.log.Debugf("waitUntilBundleConfirmed: '%v' start waiting for the bundle to be confirmed", gen.name)
//	defer gen.log.Debugf("waitUntilBundleConfirmed: '%v' %v left", gen.name, bundleHash)
//
//	startWaiting := time.Now()
//	var sinceWaiting time.Duration
//
//	for count := 0; ; count++ {
//		time.Sleep(sleepEveryLoop)
//		sinceWaiting = time.Since(startWaiting)
//		if count%5 == 0 {
//			gen.log.Debugf("'%v': waitUntilBundleConfirmed: %v Time since waiting: %v", gen.name, bundleHash, sinceWaiting)
//		}
//		//confirmed, err := lib.IsBundleHashConfirmed(bundleHash, gen.iotaAPI)
//		var retMapi multiapi.MultiCallRet
//		confirmed, err := lib.IsBundleHashConfirmedMulti(bundleHash, gen.iotaMultiAPI, &retMapi)
//
//		if AEC.CheckError(retMapi.Endpoint, err) {
//			gen.log.Errorf("'%v' waitUntilBundleConfirmed: FindTransactions returned: %v. Time since waiting: %v",
//				gen.name, err, sinceWaiting)
//		} else {
//			if confirmed {
//				return
//			}
//		}
//	}
//}

func (gen *transferBundleGenerator) isSpentAddr(address Hash) (bool, error) {
	var apiret multiapi.MultiCallRet
	spent, err := gen.iotaMultiAPI.WereAddressesSpentFrom(address, &apiret)

	if AEC.CheckError(apiret.Endpoint, err) {
		return false, err
	} else {
		return spent[0], nil
	}
}

func (gen *transferBundleGenerator) getBalanceAddr(addresses Hashes) (*Balances, error) {
	var apiret multiapi.MultiCallRet
	balances, err := gen.iotaMultiAPI.GetBalances(addresses, 100, &apiret)

	if AEC.CheckError(apiret.Endpoint, err) {
		return nil, err
	} else {
		return balances, nil
	}
}

func (gen *transferBundleGenerator) getAddress(index uint64) (Hash, error) {
	return GenerateAddress(gen.seed, index, securityLevel)
}

func (gen *transferBundleGenerator) getLastIndexFname() string {
	return path.Join(Config.siteDataDir, Config.Logging.WorkingSubdir, gen.params.GetUID())
}

func (gen *transferBundleGenerator) saveIndex() bool {
	fname := gen.getLastIndexFname()
	fout, err := os.OpenFile(fname, os.O_CREATE|os.O_WRONLY, 0666)
	if err == nil {
		defer fout.Close()
		_, err = fout.WriteString(fmt.Sprintf("%v", gen.index))
	}
	if err == nil {
		gen.log.Debugf("Last idx %v saved to %v", gen.index, fname)
	} else {
		gen.log.Errorf("Transfer Bundles: '%v' saveIndex: %v", gen.name, err)
		time.Sleep(5 * time.Second)
	}
	return err == nil
}

func (gen *transferBundleGenerator) sendBalance(fromAddr, toAddr Trytes, balance uint64,
	seed Trytes, fromIndex uint64) (*bundle_source.FirstBundleData, error) {

	ret := &bundle_source.FirstBundleData{
		NumAttach: 1,
	}
	//------ prepare transfer
	transfers := Transfers{{
		Address: toAddr,
		Value:   balance,
		Tag:     gen.txTag,
	}}
	inputs := []Input{{
		Address:  fromAddr,
		Security: securityLevel,
		KeyIndex: fromIndex,
		Balance:  balance,
	}}
	ts := lib.UnixSec(time.Now()) // currected, must be seconds, not milis
	prepTransferOptions := PrepareTransfersOptions{
		Inputs:    inputs,
		Timestamp: &ts,
	}
	// signing is here
	// do not use MultiAPi because PrepareTransfers won't call to api anyway
	// TODO multiapi
	bundlePrep, err := gen.iotaMultiAPI.GetAPI().PrepareTransfers(seed, transfers, prepTransferOptions)

	if AEC.CheckError("local", err) {
		return nil, err
	}
	//----- end prepare transfer

	//------ find two transactions to approve

	var apiret multiapi.MultiCallRet
	gttaResp, err := gen.iotaMultiAPIgTTA.GetTransactionsToApprove(3, &apiret)

	if AEC.CheckError(apiret.Endpoint, err) {
		return nil, err
	}
	ret.TotalDurationTipselMs = uint64(apiret.Duration / time.Millisecond)

	// do POW by calling attachToTangle
	bundleRet, err := gen.iotaMultiAPIaTT.AttachToTangle(
		gttaResp.TrunkTransaction,
		gttaResp.BranchTransaction,
		14,
		bundlePrep,
		&apiret,
	)
	if AEC.CheckError(apiret.Endpoint, err) {
		return nil, err
	}
	ret.TotalDurationPoWMs = uint64(apiret.Duration / time.Millisecond)

	// store and broadcast bundle
	// no multi args!!!
	_, err = gen.iotaMultiAPI.StoreAndBroadcast(bundleRet, &apiret)
	if AEC.CheckError(apiret.Endpoint, err) {
		return nil, err
	}
	ret.BundleTrytes = bundleRet
	return ret, nil
}

func (gen *transferBundleGenerator) sendToNext(addr Hash) (*bundle_source.FirstBundleData, error) {
	nextAddr, err := gen.getAddress(gen.index + 1)
	if err != nil {
		return nil, err
	}
	gen.log.Debugf("Inside sendToNext with tag = '%v'. idx=%v. %v --> %v", gen.txTag, gen.index, addr, nextAddr)

	gbResp, err := gen.getBalanceAddr(Hashes{addr})
	if err != nil {
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

func (gen *transferBundleGenerator) findBundleToConfirm(addr Hash) (*bundle_source.FirstBundleData, error) {
	// find all transactions of the address
	txs, err := gen.findTransactionObjects(
		FindTransactionsQuery{
			Addresses: Hashes{addr},
		},
	)
	if err != nil {
		return nil, err
	}
	// filter out spending transactions, collect set of bundles of those transactions
	// note that bundle hashes can be more than one in rare cases
	var spendingBundleHashes Hashes
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
		FindTransactionsQuery{
			Bundles: spendingBundleHashes,
		},
	)

	// collect all tails
	var tails []*Transaction
	for i := range txs {
		if IsTailTransaction(&txs[i]) {
			tail := txs[i]
			tails = append(tails, &tail)
		}
	}
	if len(tails) == 0 {
		gen.log.Errorf("findBundleToConfirm: there are transaction but no tails found. No consistent bundle for address%v", addr)
		return nil, nil // empty bundle, i.e. no bundle found
	}
	// collect hashes of tails
	var tailHashes Hashes
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
	maxTail := tails[0]
	maxTime := maxTail.Timestamp
	for _, tail := range tails {
		if tail.Timestamp > maxTime {
			maxTime = tail.Timestamp
			maxTail = tail
		}
	}
	// collect the bundle by maxTail
	txSet := lib.ExtractBundleTransactionsByTail(maxTail, txs)
	if txSet == nil {
		return nil, errors.New(fmt.Sprintf("Can't get bundle from tail hash = %v in addr = %v: %v",
			maxTail.Hash, addr, err))
	}
	txSet, err = lib.CheckAndSortTxSetAsBundle(txSet)

	if err != nil {
		// report error but not return, return no consistent bundle was found. To be created new one
		gen.log.Errorf("Inconsistency of spending bundle in addr = %v: %v", addr, err)
		return nil, nil
	}
	var bundleTrytes []Trytes
	bundleTrytes, err = lib.TransactionSetToBundleTrytes(txSet)
	if err != nil {
		return nil, err
	}
	ret := &bundle_source.FirstBundleData{
		BundleTrytes: bundleTrytes,
		NumAttach:    uint64(len(tails)),
	}
	return ret, nil
}

func (gen *transferBundleGenerator) isAnyConfirmed(txHashes Hashes) (bool, error) {
	//confirmed, err := lib.IsAnyConfirmed(txHashes, gen.iotaAPI)
	var apiret multiapi.MultiCallRet
	confirmed, err := lib.IsAnyConfirmedMulti(txHashes, gen.iotaMultiAPI, &apiret)
	AEC.CheckError(apiret.Endpoint, err)
	return confirmed, err
}

// new one works with big number of transactions
func (gen *transferBundleGenerator) findTransactionObjects(query FindTransactionsQuery) (Transactions, error) {
	//ret, err := lib.FindTransactionObjects(query, gen.iotaAPI)
	var apiret multiapi.MultiCallRet
	ret, err := lib.FindTransactionObjectsMulti(query, gen.iotaMultiAPI, &apiret)
	if AEC.CheckError(apiret.Endpoint, err) {
		return nil, err
	}
	return ret, nil
}
