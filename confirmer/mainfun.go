package confirmer

import (
	"errors"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/bundle"
	. "github.com/iotaledger/iota.go/transaction"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/lib"
	"strings"
	"time"
)

func (conf *Confirmer) attachToTangle(trunkHash, branchHash Hash, trytes []Trytes) ([]Trytes, error) {
	ret, err := conf.IotaAPIaTT.AttachToTangle(trunkHash, branchHash, 14, trytes)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPIaTT)
	}
	return ret, err
}

var all9 = Trytes(strings.Repeat("9", 81))

func (conf *Confirmer) promote() error {
	var err error
	if conf.Log != nil {
		var m string
		if conf.PromoteChain {
			m = "chain"
		} else {
			m = "blowball"
		}
		conf.Log.Debugf("CONFIRMER: promoting '%v'. Tag = '%v'. Tail = %v Bundle = %v",
			m, conf.TxTagPromote, conf.nextTailHashToPromote, conf.bundleHash)
	}
	transfers := Transfers{{
		Address: all9,
		Value:   0,
		Tag:     conf.TxTagPromote,
	}}
	ts := lib.UnixMs(time.Now())
	prepTransferOptions := PrepareTransfersOptions{
		Timestamp: &ts,
	}
	bundleTrytesPrep, err := conf.IotaAPI.PrepareTransfers(all9, transfers, prepTransferOptions)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	st := lib.UnixMs(time.Now())
	gttaResp, err := conf.IotaAPIgTTA.GetTransactionsToApprove(3)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPIgTTA)
		return err
	}
	conf.totalDurationGTTAMsec += lib.UnixMs(time.Now()) - st

	trunkTxh := conf.nextTailHashToPromote
	branchTxh := gttaResp.BranchTransaction

	st = lib.UnixMs(time.Now())
	btrytes, err := conf.attachToTangle(trunkTxh, branchTxh, bundleTrytesPrep)
	if err != nil {
		return err
	}
	conf.totalDurationATTMsec += lib.UnixMs(time.Now()) - st

	_, err = conf.IotaAPI.BroadcastTransactions(btrytes...)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	_, err = conf.IotaAPI.StoreTransactions(btrytes...)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	nowis := time.Now()
	conf.numPromote += 1
	if conf.PromoteChain {
		tail, err := lib.TailFromBundleTrytes(btrytes)
		if err != nil {
			return err
		}
		conf.nextTailHashToPromote = tail.Hash
	}
	conf.Log.Debugf("CONFIRMER: finished promoting bundle hash %v. Next tail to promote = %v",
		conf.bundleHash, conf.nextTailHashToPromote)
	conf.nextPromoTime = nowis.Add(time.Duration(conf.PromoteEverySec) * time.Second)
	return nil
}

func (conf *Confirmer) reattach() error {
	var err error
	if curTail, err := lib.TailFromBundleTrytes(conf.lastBundleTrytes); err != nil {
		return err
	} else {
		if curTail.Bundle != conf.bundleHash {
			// assert
			return errors.New("CONFIRMER:reattach: inconsistency curTail.Bundle != conf.bundleHash")
		}
	}
	st := lib.UnixMs(time.Now())
	gttaResp, err := conf.IotaAPIgTTA.GetTransactionsToApprove(3)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPIgTTA)
		return err
	}
	conf.totalDurationGTTAMsec += lib.UnixMs(time.Now()) - st

	var btrytes []Trytes
	btrytes, err = conf.attachToTangle(
		gttaResp.TrunkTransaction,
		gttaResp.BranchTransaction,
		conf.lastBundleTrytes)
	if err != nil {
		return err
	}
	_, err = conf.IotaAPI.BroadcastTransactions(btrytes...)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	_, err = conf.IotaAPI.StoreTransactions(btrytes...)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	var newTail *Transaction
	if newTail, err = lib.TailFromBundleTrytes(btrytes); err != nil {
		return err
	}
	conf.debugf("CONFIRMER: finished reattaching. New tail hash %v", newTail.Hash)
	nowis := time.Now()
	conf.numAttach += 1
	conf.lastBundleTrytes = btrytes
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextTailHashToPromote = newTail.Hash
	conf.nextPromoTime = nowis // start promoting immediately
	conf.isNotPromotable = false
	return nil
}
