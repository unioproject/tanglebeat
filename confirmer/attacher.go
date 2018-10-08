package confirmer

import (
	"github.com/lunfardo314/giota"
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

func (conf *Confirmer) promote(tx *giota.Transaction) error {
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
	gttaResp, err := conf.iotaAPIgTTA.GetTransactionsToApprove(3, 100, giota.Trytes(""))
	if err != nil {
		return err
	}
	// TODO GTTA duration
	conf.numGTTA += 1
	trunkTxh := tx.Hash()
	branchTxh := gttaResp.BranchTransaction

	attResp, err := conf.attachToTangle(trunkTxh, branchTxh, bundle)
	if err != nil {
		return err
	}
	// TODO ATT duration
	conf.numATT += 1

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
	conf.lastPromoBundle = bundle
	conf.lastPromoTime = nowis
	conf.nextPromoTime = nowis.Add(time.Duration(conf.PromoteEverySec) * time.Second)
	return nil
}

func (conf *Confirmer) reattach() error {
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
	conf.lastBundle = attResp.Trytes
	conf.lastAttachmentTime = nowis
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.lastPromoBundle = nil
	return nil
}

func (conf *Confirmer) promoteOrReattach(tx *giota.Transaction) error {
	if ccResp, err := conf.iotaAPI.CheckConsistency([]giota.Trytes{tx.Hash()}); err != nil {
		return err
	} else {
		if ccResp.State {
			err := conf.promote(tx)
			return err
		}
	}
	// can't promote --> reattach
	return conf.reattach()
}
