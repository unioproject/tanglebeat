package confirmer

import (
	"errors"
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
	transfers := []giota.Transfer{
		{Address: giota.Address(strings.Repeat("9", 81)),
			Value: 0,
			Tag:   conf.TxTagPromote,
		},
	}
	bundle, err := giota.PrepareTransfers(
		conf.IotaAPI,
		"",
		transfers,
		nil,
		giota.Address(""),
		2,
	)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	st := lib.UnixMs(time.Now())
	gttaResp, err := conf.IotaAPIgTTA.GetTransactionsToApprove(3, 100, giota.Trytes(""))
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPIgTTA)
		return err
	}
	tail := lib.GetTail(conf.nextBundleToPromote)
	if tail == nil {
		return errors.New("can't get tail of the bundle")
	}
	conf.totalDurationGTTAMsec += lib.UnixMs(time.Now()) - st
	trunkTxh := tail.Hash()
	branchTxh := gttaResp.BranchTransaction

	st = lib.UnixMs(time.Now())
	attResp, err := conf.attachToTangle(trunkTxh, branchTxh, bundle)
	if err != nil {
		return err
	}
	conf.totalDurationATTMsec += lib.UnixMs(time.Now()) - st

	bundle = attResp.Trytes
	err = conf.IotaAPI.BroadcastTransactions(bundle)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	err = conf.IotaAPI.StoreTransactions(bundle)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	nowis := time.Now()
	conf.numPromote += 1
	if conf.PromoteChain {
		conf.nextBundleToPromote = bundle
	}
	conf.nextPromoTime = nowis.Add(time.Duration(conf.PromoteEverySec) * time.Second)
	return nil
}

func (conf *Confirmer) reattach() error {
	if conf.Log != nil {
		conf.Log.Debugf("CONFIRMER: reattaching")
	}
	gttaResp, err := conf.IotaAPIgTTA.GetTransactionsToApprove(3, 100, giota.Trytes(""))
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPIgTTA)
		return err
	}
	attResp, err := conf.attachToTangle(gttaResp.TrunkTransaction, gttaResp.BranchTransaction, conf.lastBundle)
	if err != nil {
		return err
	}
	err = conf.IotaAPI.BroadcastTransactions(attResp.Trytes)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	err = conf.IotaAPI.StoreTransactions(attResp.Trytes)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
		return err
	}
	nowis := time.Now()
	conf.numAttach += 1
	conf.lastBundle = attResp.Trytes
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextBundleToPromote = conf.lastBundle
	conf.nextPromoTime = nowis // start promoting immediately
	conf.isNotPromotable = false
	return nil
}
