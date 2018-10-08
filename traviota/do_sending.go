package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/comm"
	"github.com/lunfardo314/tanglebeat/lib"
	"sync"
	"time"
)

func (seq *Sequence) startSending(addr giota.Address, index int) func() {
	sendingStarted := time.Now()

	var wg sync.WaitGroup

	go func() {
		seq.log.Debugf("Started startSending routine for idx = %v, %v", index, addr)
		defer seq.log.Debugf("Finished startSending routine for idx = %v, %v", index, addr)

		wg.Add(1)
		defer wg.Done()

		var initStats comm.SendingStats
		bundle, err := seq.findOrCreateBundleToConfirm(addr, index, &initStats)
		if err != nil {
			seq.log.Errorf("startSending: index = %v: %v", index, err)
		}
		chConfUpd := seq.startConfirmer(bundle, seq.log)
		for updConf := range chConfUpd {
			// summing up with stats collected during findOrCreateBundleToConfirm
			if updConf.Err != nil {
				seq.log.Errorf("Received error from confirmer: %v", updConf.Err)
			} else {
				if updConf.UpdateType != comm.UPD_NO_ACTION {
					updConf.Stats.NumAttaches += initStats.NumAttaches
					updConf.Stats.NumATT += initStats.NumATT
					updConf.Stats.NumGTTA += initStats.NumGTTA
					updConf.Stats.TotalDurationATTMsec += initStats.TotalDurationATTMsec
					updConf.Stats.TotalDurationGTTAMsec += initStats.TotalDurationGTTAMsec
					seq.publishSenderUpdate(updConf, addr, index, sendingStarted)
				}
			}
		}
	}()
	return func() {
		wg.Wait()
	}
}

func (seq *Sequence) findOrCreateBundleToConfirm(addr giota.Address, index int, sendingStats *comm.SendingStats) (giota.Bundle, error) {
	bundle, err := seq.findLatestSpendingBundle(addr, index, sendingStats)
	if err != nil {
		return nil, err
	}
	if len(bundle) == 0 {
		return seq.sendToNext(addr, index, sendingStats)
	}
	confirmed, err := seq.isConfirmed(lib.GetTail(bundle).Hash())
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return bundle, nil
	}
	// exotic situation, when balance != 0 and address is spent
	// can happen when sent iotas in the middle of sending
	balance, err := seq.GetBalanceAddr([]giota.Address{addr})
	if err != nil {
		return nil, err
	}
	if balance[0] > 0 {
		return seq.sendToNext(addr, index, sendingStats)
	}
	// confirmed, bal = 0  --> nothing to do
	return nil, err
}

func (seq *Sequence) findTrytes(txReq *giota.FindTransactionsRequest) (*giota.GetTrytesResponse, error) {
	// TODO tx cache
	ftResp, err := seq.IotaAPI.FindTransactions(txReq)
	if err != nil {
		return nil, err
	}
	return seq.IotaAPI.GetTrytes(ftResp.Hashes)
}

func (seq *Sequence) findLatestSpendingBundle(addr giota.Address, index int, sendingStats *comm.SendingStats) (giota.Bundle, error) {
	// find all transactions of the address
	ftResp, err := seq.findTrytes(
		&giota.FindTransactionsRequest{
			Addresses: []giota.Address{addr},
		},
	)
	// filter out spending transactions, collect set of bundles of those transactions
	// note that bundle hashes can be more than one in rate cases
	var spendingBundleHashes []giota.Trytes
	for _, tx := range ftResp.Trytes {
		if tx.Value < 0 && !lib.TrytesInSet(tx.Bundle, spendingBundleHashes) {
			spendingBundleHashes = append(spendingBundleHashes, tx.Bundle)
		}
	}
	if len(spendingBundleHashes) == 0 {
		return nil, nil // no error, empty bundle
	}

	//find all transactions, belonging to spending bundles
	ftResp, err = seq.findTrytes(
		&giota.FindTransactionsRequest{
			Bundles: spendingBundleHashes,
		},
	)
	spendingBundlesTx := ftResp.Trytes
	// select the oldest tail by Timestamp
	// TODO  use attachmentTimestamp instead
	var maxTime time.Time
	var maxTail giota.Transaction
	var numTails int
	for _, tx := range spendingBundlesTx {
		if tx.CurrentIndex == 0 {
			if tx.Timestamp.After(maxTime) {
				maxTime = tx.Timestamp
				maxTail = tx
			}
			numTails += 1
		}
	}
	// collect the bundle by maxTail
	bundleTx := extractBundleTxByTail(maxTail, spendingBundlesTx)
	bundleTx, err = lib.CheckAndSortBundle(bundleTx)
	if err != nil {
		// report but don't rise the error about bundle inconsistency,
		// because it come from the node
		seq.log.Errorf("Bundle inconsistency in idx = %v: %v", index, err)
	}
	sendingStats.NumAttaches = numTails
	return giota.Bundle(bundleTx), nil
}

// give the tail and transaction set, filers out from the set the bundle of that tail
// checks consistency of the bundle. Sorts it by Index

func extractBundleTxByTail(tail giota.Transaction, allTx []giota.Transaction) []giota.Transaction {
	// first in a bundle is tail tx
	var ret []giota.Transaction
	tx := tail
	count := 0
	// counting and caping steps to avoid eternal loops along trunk (impossible, I know)
	for {
		if count >= len(allTx) {
			return ret
		}
		ret = append(ret, tx)
		var ok bool
		tx, ok = lib.FindTxByHash(tx.TrunkTransaction, allTx)
		if !ok {
			return ret
		}
		count++
	}
	return ret
}
