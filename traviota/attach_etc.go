package main

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"time"
)

func (seq *Sequence) attachToTangle(trunkHash, branchHash giota.Trytes, trytes []giota.Transaction) (*giota.AttachToTangleResponse, error) {
	return seq.IotaAPIaTT.AttachToTangle(&giota.AttachToTangleRequest{
		TrunkTransaction:   trunkHash,
		BranchTransaction:  branchHash,
		Trytes:             trytes,
		MinWeightMagnitude: 14,
	})
}

func (seq *Sequence) sendToNext(index int, state *sendingState) (*sendingState, error) {
	addr, err := seq.GetAddress(index)
	if err != nil {
		return nil, err
	}
	gbResp, err := seq.IotaAPI.GetBalances([]giota.Address{addr}, 100)
	if err != nil {
		return nil, err
	}
	balance := gbResp.Balances[0]
	if balance == 0 {
		return nil, errors.New(fmt.Sprintf("Address %v has 0 balance, can't sent to the next.", addr))
	}
	nextAddress, err := seq.GetAddress(index + 1)
	if err != nil {
		return nil, err
	}
	transfers := []giota.Transfer{
		{Address: nextAddress,
			Value: balance,
			Tag:   seq.TxTag,
		},
	}
	inputs := []giota.AddressInfo{
		{Seed: seq.Seed, Index: index, Security: 2},
	}
	bundle, err := giota.PrepareTransfers(
		seq.IotaAPI,
		seq.Seed,
		transfers,
		inputs,
		giota.Address(""),
		seq.SecurityLevel,
	)
	if err != nil {
		return nil, err
	}
	gttaResp, err := seq.IotaAPIgTTA.GetTransactionsToApprove(3, 100, giota.Trytes(""))
	if err != nil {
		return nil, err
	}
	attResp, err := seq.attachToTangle(gttaResp.TrunkTransaction, gttaResp.BranchTransaction, bundle)
	if err != nil {
		return nil, err
	}
	err = seq.IotaAPI.BroadcastTransactions(attResp.Trytes)
	if err != nil {
		return nil, err
	}
	err = seq.IotaAPI.StoreTransactions(attResp.Trytes)
	if err != nil {
		return nil, err
	}
	// wait until address will acquire isSpent status
	spent, err := seq.IsSpentAddr(addr)
	timeout := 10
	for count := 0; !spent && err == nil; count++ {
		if count > timeout {
			return nil, errors.New("!!!!!!!! Didn't get 'spent' state in 10 seconds")
		}
		time.Sleep(1 * time.Second)
		spent, err = seq.IsSpentAddr(addr)
	}
	ret := *state
	nowis := time.Now()
	ret.lastBundle = attResp.Trytes
	ret.lastAttachmentTime = nowis
	ret.nextForceReattachTime = nowis.Add(time.Duration(seq.Params.ForceReattachAfterMin) * time.Minute)
	ret.lastPromoBundle = nil
	return &ret, err
}

func (seq *Sequence) promote(tx *giota.Transaction, state *sendingState) (*sendingState, error) {
	//nowis := time.Now()
	//ret.lastPromoTime = nowis
	//ret.nextPromoTime = nowis.Add(time.Duration(seq.Params.PromoteEverySec) * time.Second)
	return nil, nil
}

func (seq *Sequence) reattach(tx *giota.Transaction, state *sendingState) (*sendingState, error) {
	//nowis := time.Now()
	//ret.lastAttachmentTime = nowis
	//ret.nextForceReattachTime = nowis.Add(time.Duration(seq.Params.ForceReattachAfterMin) * time.Minute)
	//ret.lastPromoBundle = nil
	return nil, nil
}
