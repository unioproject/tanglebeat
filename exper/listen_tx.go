package main

//
//import (
//	"github.com/lunfardo314/giota"
//	"github.com/lunfardo314/gosender/lib"
//	"sync"
//	"time"
//)
//
//const (
//	POSITIVE_VALUE = 0x1
//	ZERO_VALUE     = 0x2
//	NEGATIVE_VALUE = 0x4
//	ALL_VALUES     = POSITIVE_VALUE | ZERO_VALUE | NEGATIVE_VALUE
//)
//
//type TransactionsNew struct {
//	Transactions []giota.Transaction
//	TotalNumTx   int
//	Refreshed    time.Time
//	Duration     time.Duration
//}
//
//// producer channel for new transactions, appeared for the address since the last message
//// can be filtered based on value sign
//func (seq *Sequence) NewListenTxChan(index int, filterFlags int) (chan *TransactionsNew, func()) {
//	chOut := make(chan *TransactionsNew)
//	chCancel := make(chan struct{})
//	seq.log.Debugf("Created ListenTx channel for idx=%v filter = %X", index, filterFlags)
//
//	addr, err := seq.GetAddress(index)
//	if err != nil {
//		seq.log.Error(err)
//	}
//
//	var wg sync.WaitGroup
//	var txHashes []giota.Trytes
//
//	go func() {
//		seq.log.Debugf("Starting processing ListenTx channel for idx = %v filter = %X", index, filterFlags)
//		defer close(chOut)
//		defer seq.log.Debugf("ListenTx: closed channel for %v", index)
//
//		wg.Add(1)
//		defer wg.Done()
//
//		firstRun := true
//		for {
//			start := time.Now()
//			allTxHashes, newTx := seq.readNewTx(addr, txHashes, filterFlags)
//			if len(newTx) > 0 || firstRun {
//				select {
//				case <-chCancel:
//					return
//				case <-time.After(5 * time.Second):
//				case chOut <- &TransactionsNew{
//					Transactions: newTx,
//					TotalNumTx:   len(allTxHashes),
//					Refreshed:    time.Now(),
//					Duration:     time.Since(start),
//				}:
//					txHashes = allTxHashes
//					seq.log.Debugf("ListenTx: sending out %v new transactions out of total %v for address filter = %X idx = %v",
//						len(newTx), len(txHashes), filterFlags, index)
//				}
//			} else {
//				seq.log.Debugf("ListenTx: no new tx to send for idx = %v : %v ", index, addr)
//				select {
//				case <-chCancel:
//					return
//				case <-time.After(5 * time.Second):
//				}
//			}
//			firstRun = false
//		}
//	}()
//
//	return chOut, func() {
//		close(chCancel)
//		wg.Wait()
//	} // return the channel and cancel closure
//}
//
//func (seq *Sequence) readNewTx(addr giota.Address, currentTxHashes []giota.Trytes, filterFlags int) ([]giota.Trytes, []giota.Transaction) {
//	ftResp, err := seq.IotaAPI.FindTransactions(
//		&giota.FindTransactionsRequest{
//			Addresses: []giota.Address{addr},
//		},
//	)
//	if err != nil {
//		seq.log.Error(err)
//		return currentTxHashes, nil
//	}
//
//	var newTxh []giota.Trytes
//	for _, hash := range ftResp.Hashes {
//		if !lib.TrytesInSet(hash, currentTxHashes) {
//			newTxh = append(newTxh, hash)
//		}
//	}
//	if len(newTxh) == 0 {
//		return currentTxHashes, nil
//	}
//	gtResp, err := seq.IotaAPI.GetTrytes(newTxh)
//	if err != nil {
//		seq.log.Error(err)
//		return currentTxHashes, nil
//	}
//	var ret []giota.Transaction
//	for _, tx := range gtResp.Trytes {
//		switch {
//		case tx.Value > 0:
//			if (filterFlags & POSITIVE_VALUE) != 0 {
//				ret = append(ret, tx)
//			}
//		case tx.Value == 0:
//			if (filterFlags & ZERO_VALUE) != 0 {
//				ret = append(ret, tx)
//			}
//		case tx.Value < 0:
//			if (filterFlags & NEGATIVE_VALUE) != 0 {
//				ret = append(ret, tx)
//			}
//		}
//	}
//	return ftResp.Hashes, ret
//}
//
//// pipeline channel
//// detect new tails in bundles according to the filter
//// receives new transactions (filtered), finds if there new bundle hashes,
//// then finds all transactions of those bundle hashes, returns to channel new tails
//
//func (seq *Sequence) NewListenTailsChan(index int, filterFlags int) (chan *TransactionsNew, func()) {
//	chOut := make(chan *TransactionsNew)
//	chCancel := make(chan struct{})
//
//	seq.log.Debugf("Created ListenTails channel for idx=%v filter = %X", index, filterFlags)
//
//	chTx, cancelChanTx := seq.NewListenTxChan(index, filterFlags)
//
//	var wg sync.WaitGroup
//
//	go func() {
//		seq.log.Debugf("Start processing ListenTails channel for idx=%v filter = %X", index, filterFlags)
//		defer close(chOut)
//		defer seq.log.Debugf("ListenTails: closed channel for %v", index)
//
//		wg.Add(1)
//		defer wg.Done()
//
//		var tails []giota.Transaction
//		for newTx := range chTx {
//			start := time.Now()
//			newTails := seq.readNewTails(newTx.Transactions, tails)
//			tailsTmp := make([]giota.Transaction, len(tails))
//			for _, tx := range newTails {
//				tailsTmp = append(tailsTmp, tx)
//			}
//			select {
//			case <-chCancel:
//				return
//			case <-time.After(5 * time.Second):
//			case chOut <- &TransactionsNew{
//				Transactions: newTails,
//				TotalNumTx:   len(tailsTmp),
//				Refreshed:    time.Now(),
//				Duration:     time.Since(start),
//			}:
//				tails = tailsTmp
//				seq.log.Debugf("ListenTails: sending out %v new tails out of total %v for address filter=%X idx = %v",
//					len(newTails), len(tails), filterFlags, index)
//			}
//		}
//	}()
//	return chOut, func() {
//		cancelChanTx()
//		close(chCancel)
//		wg.Wait()
//	} // return the channel and cancel closure
//
//}
//
//func (seq *Sequence) readNewTails(newTx, currentTails []giota.Transaction) []giota.Transaction {
//	if len(newTx) == 0 {
//		return nil
//	}
//	var bundleHashes []giota.Trytes
//
//	// bundle hashes of new transactions
//	for _, tx := range newTx {
//		if !lib.TrytesInSet(tx.Bundle, bundleHashes) {
//			bundleHashes = append(bundleHashes, tx.Bundle)
//		}
//	}
//	// find ALL tx with those bundle hashes
//	ftResp, err := seq.IotaAPI.FindTransactions(
//		&giota.FindTransactionsRequest{
//			Bundles: bundleHashes,
//		},
//	)
//	if err != nil {
//		seq.log.Error(err)
//		return nil
//	}
//	// load tx objects of all transactions
//	// TODO caching GetTrytes, optimize
//	gtResp, err := seq.IotaAPI.GetTrytes(ftResp.Hashes)
//	if err != nil {
//		seq.log.Error(err)
//		return nil
//	}
//	// hashes of current tails
//	var currentTailHashes []giota.Trytes
//	for _, tx := range currentTails {
//		currentTailHashes = append(currentTailHashes, tx.Hash())
//	}
//	// collect new tails of bundles
//	var newTails []giota.Transaction
//	for _, tx := range gtResp.Trytes {
//		if tx.CurrentIndex == 0 && !lib.TrytesInSet(tx.Hash(), currentTailHashes) {
//			newTails = append(newTails, tx)
//		}
//	}
//	return newTails
//}
