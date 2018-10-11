package confirmer

import (
	"errors"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
	"strings"
	"time"
)

func (conf *Confirmer) attachToTangle(trunkHash, branchHash giota.Trytes, trytes []giota.Transaction) (*giota.AttachToTangleResponse, error) {
	return conf.iotaAPIaTT.AttachToTangle(&giota.AttachToTangleRequest{
		TrunkTransaction:   trunkHash,
		BranchTransaction:  branchHash,
		Trytes:             trytes,
		MinWeightMagnitude: 14,
	})
}

func (conf *Confirmer) promote() error {
	if conf.log != nil {
		var m string
		if conf.PromoteChain {
			m = "chain"
		} else {
			m = "blowball"
		}
		conf.log.Debugf("CONFIRMER: promoting '%v' every ~%v sec if bundle is consistent. Tag = '%v'",
			m, conf.PromoteEverySec, conf.TxTagPromote)
	}
	transfers := []giota.Transfer{
		{Address: giota.Address(strings.Repeat("9", 81)),
			Value: 0,
			Tag:   conf.TxTagPromote,
		},
	}
	bundle, err := giota.PrepareTransfers(
		conf.iotaAPI,
		"",
		transfers,
		nil,
		giota.Address(""),
		2,
	)
	if err != nil {
		return err
	}
	st := lib.UnixMs(time.Now())
	gttaResp, err := conf.iotaAPIgTTA.GetTransactionsToApprove(3, 100, giota.Trytes(""))
	if err != nil {
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
	err = conf.iotaAPI.BroadcastTransactions(bundle)
	if err != nil {
		return err
	}
	err = conf.iotaAPI.StoreTransactions(bundle)
	if err != nil {
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
	if conf.log != nil {
		conf.log.Debugf("CONFIRMER: reattaching")
	}
	gttaResp, err := conf.iotaAPIgTTA.GetTransactionsToApprove(3, 100, giota.Trytes(""))
	if err != nil {
		return err
	}
	attResp, err := conf.attachToTangle(gttaResp.TrunkTransaction, gttaResp.BranchTransaction, conf.lastBundle)
	if err != nil {
		return err
	}
	err = conf.iotaAPI.BroadcastTransactions(attResp.Trytes)
	if err != nil {
		return err
	}
	err = conf.iotaAPI.StoreTransactions(attResp.Trytes)
	if err != nil {
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
