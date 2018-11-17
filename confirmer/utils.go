package confirmer

import (
	"errors"
	"github.com/iotaledger/iota.go/api"
	"github.com/iotaledger/iota.go/bundle"
	"github.com/iotaledger/iota.go/transaction"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat1/lib"
	"strings"
	"time"
)

func (conf *Confirmer) attachToTangle(trunkHash, branchHash trinary.Hash, trytes []trinary.Trytes) ([]trinary.Trytes, error) {
	ret, err := conf.IotaAPIaTT.AttachToTangle(trunkHash, branchHash, 14, trytes)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPIaTT)
	}
	return ret, err
}

func (conf *Confirmer) promote() error {
	var err error
	if conf.Log != nil {
		var m string
		if conf.PromoteChain {
			m = "chain"
		} else {
			m = "blowball"
		}
		conf.Log.Debugf("CONFIRMER: promoting '%v' every ~%v sec if bundle is consistent. Tag = '%v'",
			m, conf.PromoteEverySec, conf.TxTagPromote)
	}
	all9 := trinary.Trytes(strings.Repeat("9", 81))
	transfers := bundle.Transfers{{
		Address: all9,
		Value:   0,
		Tag:     conf.TxTagPromote,
	}}
	ts := lib.UnixMs(time.Now())
	prepTransferOptions := api.PrepareTransfersOptions{
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
		// vienintele tx turi buti tail
		tail, err := transaction.AsTransactionObject(btrytes[0])
		if err != nil {
			conf.AEC.IncErrorCount(conf.IotaAPI)
			return err
		}
		if !transaction.IsTailTransaction(tail) {
			return errors.New("can't get tail of the bundle")
		}
		conf.nextTailHashToPromote = tail.Hash
	}
	conf.nextPromoTime = nowis.Add(time.Duration(conf.PromoteEverySec) * time.Second)
	return nil
}

func (conf *Confirmer) reattach() error {
	var err error
	if conf.Log != nil {
		conf.Log.Debugf("CONFIRMER: reattaching")
	}
	st := lib.UnixMs(time.Now())
	gttaResp, err := conf.IotaAPIgTTA.GetTransactionsToApprove(3)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPIgTTA)
		return err
	}
	conf.totalDurationGTTAMsec += lib.UnixMs(time.Now()) - st

	var btrytes []trinary.Trytes
	btrytes, err = conf.attachToTangle(
		gttaResp.TrunkTransaction,
		gttaResp.BranchTransaction,
		conf.lastBundleTrytes)
	if err != nil {
		return err
	}
	tmpTxs, err := transaction.AsTransactionObjects(btrytes, nil)
	if err != nil {
		return err
	}
	tail := lib.FindTail(tmpTxs)
	if tail == nil {
		return errors.New("FindTail: inconsistency")
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
	nowis := time.Now()
	conf.numAttach += 1
	conf.lastBundleTrytes = btrytes
	conf.lastTail = *tail
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextTailHashToPromote = tail.Hash
	conf.nextPromoTime = nowis // start promoting immediately
	conf.isNotPromotable = false
	return nil
}
