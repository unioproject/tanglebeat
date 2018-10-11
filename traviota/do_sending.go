package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/pubsub"
	"time"
)

func (seq *Sequence) doSending(addr giota.Address, index int) {
	sendingStarted := time.Now()

	seq.log.Debugf("Started doSending routine for idx = %v, %v", index, addr)
	defer seq.log.Debugf("Finished doSending routine for idx = %v, %v", index, addr)

	var err error
	var bundle giota.Bundle

	var initStats pubsub.SendingStats
	for {
		if zero, err := seq.isZeroBalanceAndSpent(addr); err != nil {
			seq.log.Errorf("isZeroBalanceAndSpent: %v", err)
			time.Sleep(5 * time.Second)
			continue
		} else {
			if zero {
				return //>>>>>>>>>>>>>>>>>>
			}
		}
		if bundle, err = seq.findOrCreateBundleToConfirm(addr, index, &initStats); err != nil {
			seq.log.Errorf("findOrCreateBundleToConfirm: index = %v: %v", index, err)
			time.Sleep(5 * time.Second)
			continue
		}
		if len(bundle) == 0 {
			// nothing left to spend, leaving routine
			return //>>>>>>>>>>>>>>>>>>>>>>>
		}
		seq.initSendUpdateToPub(addr, index, sendingStarted, &initStats)

		// start confirmer and run until confirmed
		var chConfUpd chan *confirmer.ConfirmerUpdate
		if chConfUpd, err = seq.RunConfirmer(bundle, seq.log); err != nil {
			seq.log.Errorf("RunConfirmer: index = %v: %v", index, err)
			time.Sleep(5 * time.Second)
			continue
		}
		for updConf := range chConfUpd {
			// summing up with stats collected during findOrCreateBundleToConfirm
			if updConf.Err != nil {
				seq.log.Errorf("Received error from confirmer: %v", updConf.Err)
			} else {
				if updConf.Err == nil {
					updConf.Stats.NumAttaches += initStats.NumAttaches
					updConf.Stats.TotalDurationATTMsec += initStats.TotalDurationATTMsec
					updConf.Stats.TotalDurationGTTAMsec += initStats.TotalDurationGTTAMsec

					seq.confirmerUpdateToPub(updConf, addr, index, sendingStarted)
				} else {
					log.Errorf("Confirmer reported an error: %v", updConf.Err)
				}
			}
		}
	}
}

// finds or creates latest unconfirmed bundle, if balance != 0
// otherwise return empty bundle
func (seq *Sequence) findOrCreateBundleToConfirm(addr giota.Address, index int, sendingStats *pubsub.SendingStats) (giota.Bundle, error) {
	bundle, err := seq.findBundleToConfirm(addr, index, sendingStats)
	if err != nil {
		return nil, err
	}
	if len(bundle) > 0 {
		// existing bundle to confirm was found
		return bundle, nil
	}
	// there're no bundles to confirm
	// check the balance
	balance, err := seq.GetBalanceAddr([]giota.Address{addr})
	if err != nil {
		return nil, err
	}
	if balance[0] > 0 {
		// exotic situation. Can happen if additional iotas were sent while trying
		// to confirm current bundle
		// all spending bundle are confirmed, but address balance still > 0.
		// initiate sending of the reminder. This exposes private key, but who cares: address won't be used again
		return seq.sendToNext(addr, index, sendingStats)
	}
	// otherwise no bundle was found and balance is 0. Nothing to do
	return nil, nil
}

func (seq *Sequence) findTrytes(txReq *giota.FindTransactionsRequest) (*giota.GetTrytesResponse, error) {
	// TODO tx cache
	ftResp, err := seq.IotaAPI.FindTransactions(txReq)
	if err != nil {
		return nil, err
	}
	return seq.IotaAPI.GetTrytes(ftResp.Hashes)
}

func (seq *Sequence) findBundleToConfirm(addr giota.Address, index int, sendingStats *pubsub.SendingStats) (giota.Bundle, error) {
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

	// collect all tails
	var tails []giota.Transaction
	for _, tx := range spendingBundlesTx {
		if tx.CurrentIndex == 0 {
			tails = append(tails, tx)
		}
	}
	// TODO assert!!!!!!
	for _, t := range tails {
		lib.Assert(t.CurrentIndex == 0, ">>>> t.CurrentIndex == 0", seq.log)
	}
	// collect hashes of tails
	var tailHashes []giota.Trytes
	for _, tail := range tails {
		tailHashes = append(tailHashes, tail.Hash())
	}
	// check is ANY of tails are confirmed
	var confirmed bool
	confirmed, err = seq.isAnyConfirmed(tailHashes)
	switch {
	case err != nil:
		return nil, err
	case confirmed:
		return nil, nil // already confirmed
	}

	// none is confirmed hence no matter which one
	// select the oldest one by Timestamp
	// TODO  use attachmentTimestamp instead
	maxTime := tails[0].Timestamp
	maxTail := tails[0]
	for _, tail := range tails {
		lib.Assert(tail.CurrentIndex == 0, ">>>> tail.CurrentIndex == 0", seq.log)
		if tail.Timestamp.After(maxTime) {
			maxTime = tail.Timestamp
			maxTail = tail
		}
	}
	// collect the bundle by maxTail
	bundleTx := extractBundleTxByTail(&maxTail, spendingBundlesTx)

	bundleTx, err = lib.CheckAndSortBundle(bundleTx)

	if err != nil {
		// report but don't rise the error about bundle inconsistency,
		// because it come from the node
		seq.log.Errorf("Bundle inconsistency in idx = %v: %v", index, err)
	}
	sendingStats.NumAttaches = len(tails)
	return giota.Bundle(bundleTx), nil
}

// give the tail and transaction set, filers out from the set the bundle of that tail
// checks consistency of the bundle. Sorts it by Index

func extractBundleTxByTail(tail *giota.Transaction, allTx []giota.Transaction) []giota.Transaction {
	lib.Assert(tail.CurrentIndex == 0, "tail.CurrentIndex == 0", nil)
	// first in a bundle is tail tx
	var ret []giota.Transaction
	tx := tail
	count := 0
	// counting and capping steps to avoid eternal loops along trunk (impossible, I know)
	for {
		if count >= len(allTx) {
			return ret
		}
		ret = append(ret, *tx)
		var ok bool
		tx, ok = lib.FindTxByHash(tx.TrunkTransaction, allTx)
		if !ok {
			return ret
		}
		count++
	}
	return ret
}
