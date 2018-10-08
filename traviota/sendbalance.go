package main

import (
	"errors"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/comm"
	"github.com/lunfardo314/tanglebeat/lib"
	"time"
)

func (seq *Sequence) sendBalance(fromAddr, toAddr giota.Address, balance int64,
	seed giota.Trytes, fromIndex int, sendingStats *comm.SendingStats) (giota.Bundle, error) {
	// fromIndex is required to calculate inputs, cant specifiy inputs explicitely to PrepareTransfers
	transfers := []giota.Transfer{
		{Address: toAddr,
			Value: balance,
			Tag:   seq.TxTag,
		},
	}
	inputs := []giota.AddressInfo{
		{Seed: seq.Seed, Index: fromIndex, Security: seq.SecurityLevel},
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
	st := lib.UnixMs(time.Now())
	gttaResp, err := seq.IotaAPIgTTA.GetTransactionsToApprove(3, 100, giota.Trytes(""))
	if err != nil {
		return nil, err
	}
	sendingStats.TotalDurationGTTAMsec += lib.UnixMs(time.Now()) - st
	sendingStats.NumGTTA += 1

	st = lib.UnixMs(time.Now())
	attResp, err := seq.attachToTangle(gttaResp.TrunkTransaction, gttaResp.BranchTransaction, bundle)
	if err != nil {
		return nil, err
	}
	sendingStats.TotalDurationATTMsec += lib.UnixMs(time.Now()) - st
	sendingStats.NumATT += 1

	err = seq.IotaAPI.BroadcastTransactions(attResp.Trytes)
	if err != nil {
		return nil, err
	}
	err = seq.IotaAPI.StoreTransactions(attResp.Trytes)
	if err != nil {
		return nil, err
	}
	// wait until address will acquire isSpent status
	spent, err := seq.IsSpentAddr(fromAddr)
	timeout := 10
	for count := 0; !spent && err == nil; count++ {
		if count > timeout {
			return nil, errors.New("!!!!!!!! Didn't get 'spent' state in 10 seconds")
		}
		time.Sleep(1 * time.Second)
		spent, err = seq.IsSpentAddr(fromAddr)
	}
	sendingStats.NumAttaches += 1
	return attResp.Trytes, nil
}
