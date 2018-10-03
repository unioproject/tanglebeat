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

func (seq *Sequence) sendToNext(index int) (giota.Bundle, error) {
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
	var ret []giota.Transaction
	if gttaResp, err := seq.IotaAPIgTTA.GetTransactionsToApprove(3, 100, giota.Trytes("")); err == nil {
		if attResp, err := seq.attachToTangle(gttaResp.TrunkTransaction, gttaResp.BranchTransaction, bundle); err == nil {
			if err = seq.IotaAPI.BroadcastTransactions(attResp.Trytes); err == nil {
				if err = seq.IotaAPI.StoreTransactions(attResp.Trytes); err != nil {
					return nil, err
				} else {
					ret = attResp.Trytes
				}
			}
		} else {
			return nil, err
		}
	} else {
		return nil, err
	}
	// wait until address will acquire isSpent status
	spent, err := seq.IsSpentAddr(addr)
	timeout := 10
	for count := 0; !spent && err == nil; count++ {
		if count > timeout {
			return nil, errors.New("Didn't get 'spent' state in 10 seconds")
		}
		time.Sleep(1 * time.Second)
		spent, err = seq.IsSpentAddr(addr)
	}
	return ret, err
}
