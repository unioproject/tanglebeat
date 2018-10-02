package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/gosender/lib"
	"sync"
	"time"
)

const (
	POSITIVE_VALUE = 0x1
	ZERO_VALUE     = 0x2
	NEGATIVE_VALUE = 0x4
	ALL_VALUES     = POSITIVE_VALUE | ZERO_VALUE | NEGATIVE_VALUE
)

type TransactionsNew struct {
	Transactions []giota.Transaction
	Refreshed    time.Time
	Duration     time.Duration
}

// channel provides new transactions, appeared for the address since the last message
// can be filtered based on value sign
func (seq *Sequence) NewListenTxChan(index int, filterFlags int) (chan *TransactionsNew, func()) {
	chOut := make(chan *TransactionsNew)
	chCancel := make(chan struct{})

	addr, err := seq.GetAddress(index)
	if err != nil {
		seq.log.Error(err)
	}
	seq.log.Debugf("Starting listening tx channel for idx=%v", index)

	var wg sync.WaitGroup
	var txHashes []giota.Trytes

	go func() {
		defer seq.log.Debugf("Closed listening tx state channel for %v", index)
		defer close(chOut)
		wg.Add(1)
		defer wg.Done()

		for {
			start := time.Now()
			allTxHashes, newTx := seq.readNewTx(addr, txHashes, filterFlags)
			if len(newTx) > 0 {
				select {
				case <-chCancel:
					return
				case chOut <- &TransactionsNew{
					Transactions: newTx,
					Refreshed:    time.Now(),
					Duration:     time.Since(start),
				}:
					txHashes = allTxHashes
				case <-time.After(5 * time.Second):
				}
			}
		}
	}()

	return chOut, func() {
		close(chCancel)
		wg.Wait()
	} // return the channel and cancel closure
}

func (seq *Sequence) readNewTx(addr giota.Address, currentTxHashes []giota.Trytes, filterFlags int) ([]giota.Trytes, []giota.Transaction) {
	ftResp, err := seq.IotaAPI.FindTransactions(
		&giota.FindTransactionsRequest{
			Addresses: []giota.Address{addr},
		},
	)
	if err != nil {
		seq.log.Error(err)
		return currentTxHashes, nil
	}

	var newTxh []giota.Trytes
	for _, hash := range ftResp.Hashes {
		if !lib.TrytesInSet(hash, currentTxHashes) {
			newTxh = append(newTxh, hash)
		}
	}
	if len(newTxh) == 0 {
		return currentTxHashes, nil
	}
	gtResp, err := seq.IotaAPI.GetTrytes(newTxh)
	if err != nil {
		return currentTxHashes, nil
	}
	var ret []giota.Transaction
	for _, tx := range gtResp.Trytes {
		switch {
		case tx.Value > 0:
			if (filterFlags & POSITIVE_VALUE) != 0 {
				ret = append(ret, tx)
			}
		case tx.Value == 0:
			if (filterFlags & ZERO_VALUE) != 0 {
				ret = append(ret, tx)
			}
		case tx.Value < 0:
			if (filterFlags & NEGATIVE_VALUE) != 0 {
				ret = append(ret, tx)
			}
		}
	}
	return ftResp.Hashes, ret
}
