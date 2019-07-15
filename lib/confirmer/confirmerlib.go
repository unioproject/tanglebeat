package confirmer

import (
	"errors"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/bundle"
	. "github.com/iotaledger/iota.go/transaction"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/unioproject/tanglebeat/lib/multiapi"
	"github.com/unioproject/tanglebeat/lib/utils"
	"strings"
	"time"
)

func (conf *Confirmer) attachToTangle(trunkHash, branchHash Hash, trytes []Trytes) ([]Trytes, uint64, error) {
	var apiret multiapi.MultiCallRet
	ret, err := conf.IotaMultiAPIaTT.AttachToTangle(trunkHash, branchHash, 14, trytes, &apiret)
	conf.AEC.CheckError(apiret.Endpoint, err)
	return ret, uint64(apiret.Duration / time.Millisecond), err
}

var all9 = Trytes(strings.Repeat("9", 81))

func (conf *Confirmer) promote() (error, Hash) {
	var err error
	if conf.Log != nil {
		var m string
		if conf.PromoteChain {
			m = "chain"
		} else {
			m = "blowball"
		}
		conf.Log.Debugf("CONFIRMER-PROMO: promoting '%v'. Tag = '%v'. Address: %v Tail = %v Bundle = %v",
			m, conf.TxTagPromote, conf.AddressPromote, conf.nextTailHashToPromote, conf.bundleHash)
	}
	transfers := Transfers{{
		Address: conf.AddressPromote,
		Value:   0,
		Tag:     conf.TxTagPromote,
	}}
	ts := utils.UnixSec(time.Now()) // corrected, must be seconds, not millis
	prepTransferOptions := PrepareTransfersOptions{
		Timestamp: &ts,
	}
	// TODO multi api
	bundleTrytesPrep, err := conf.IotaMultiAPI.GetAPI().PrepareTransfers(all9, transfers, prepTransferOptions)
	if conf.AEC.CheckError(conf.IotaMultiAPI.GetAPIEndpoint(), err) {
		return err, ""
	}
	var apiret multiapi.MultiCallRet
	st := utils.UnixMs(time.Now())
	gttaResp, err := conf.IotaMultiAPIgTTA.GetTransactionsToApprove(3, &apiret)
	if conf.AEC.CheckError(apiret.Endpoint, err) {
		return err, ""
	}
	conf.totalDurationGTTAMsec += utils.UnixMs(time.Now()) - st

	trunkTxh := conf.nextTailHashToPromote
	branchTxh := gttaResp.BranchTransaction

	btrytes, duration, err := conf.attachToTangle(trunkTxh, branchTxh, bundleTrytesPrep)
	if err != nil {
		return err, ""
	}
	conf.totalDurationATTMsec += duration

	// no multi args!!!
	_, err = conf.IotaMultiAPI.StoreAndBroadcast(btrytes, &apiret)
	if conf.AEC.CheckError(apiret.Endpoint, err) {
		return err, ""
	}

	nowis := time.Now()
	conf.numPromote += 1
	tail, err := utils.TailFromBundleTrytes(btrytes)
	if err != nil {
		return err, ""
	}
	if conf.PromoteChain {
		conf.nextTailHashToPromote = tail.Hash
	}
	conf.Log.Debugf("CONFIRMER-PROMO: finished promoting bundle hash %v. Next tail to promote = %v",
		conf.bundleHash, conf.nextTailHashToPromote)
	conf.nextPromoTime = nowis.Add(time.Duration(conf.PromoteEverySec) * time.Second)
	return nil, tail.Hash
}

func (conf *Confirmer) reattach() error {
	var err error
	if curTail, err := utils.TailFromBundleTrytes(conf.lastBundleTrytes); err != nil {
		return err
	} else {
		if curTail.Bundle != conf.bundleHash {
			// assert
			return errors.New("CONFIRMER-REATT:reattach: inconsistency curTail.Bundle != conf.bundleHash")
		}
	}

	var apiret multiapi.MultiCallRet
	gttaResp, err := conf.IotaMultiAPIgTTA.GetTransactionsToApprove(3, &apiret)
	if conf.AEC.CheckError(apiret.Endpoint, err) {
		return err
	}
	conf.totalDurationGTTAMsec += uint64(apiret.Duration / time.Millisecond)

	var btrytes []Trytes
	var duration uint64
	btrytes, duration, err = conf.attachToTangle(
		gttaResp.TrunkTransaction,
		gttaResp.BranchTransaction,
		conf.lastBundleTrytes)
	if err != nil {
		return err
	}
	conf.totalDurationATTMsec += duration

	// no multi args!!!
	_, err = conf.IotaMultiAPI.StoreAndBroadcast(btrytes, &apiret)
	if conf.AEC.CheckError(apiret.Endpoint, err) {
		return err
	}

	var newTail *Transaction
	if newTail, err = utils.TailFromBundleTrytes(btrytes); err != nil {
		return err
	}
	conf.debugf("CONFIRMER-REATT: finished reattaching. New tail hash %v", newTail.Hash)
	nowis := time.Now()
	conf.numAttach += 1
	conf.lastBundleTrytes = btrytes
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextTailHashToPromote = newTail.Hash
	conf.nextPromoTime = nowis // start promoting immediately
	conf.isNotPromotable = false
	return nil
}
