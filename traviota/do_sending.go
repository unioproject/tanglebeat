package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
	"sync"
	"time"
)

// Performs (re)attachments and promotions

func (seq *Sequence) DoSending(index int) func() {
	chCancel := make(chan struct{})

	var wg sync.WaitGroup
	seq.log.Debugf("Starting DoSending routine for idx = %v", index)
	go func() {
		seq.log.Debugf("Started DoSending routine for idx=%v", index)
		defer seq.log.Debugf("Finished DoSending routine for idx=%v", index)

		wg.Add(1)
		defer wg.Done()

		latestBundle, err := seq.findLatestSpendingBundle(index)
		if err != nil {
			seq.log.Errorf("DoSending: can't find latest spending bundle: %v", err)
		}
		for {
			if len(latestBundle) == 0 {
				latestBundle, err = seq.sendToNext(index)
			} else {
				////
			}
			select {
			case <-chCancel:
				return
			case <-time.After(5 * time.Second):
				// promote or reattach
			}
		}
	}()
	return func() {
		close(chCancel)
		wg.Wait()
	}
}

func (seq *Sequence) findLatestSpendingBundle(index int) (giota.Bundle, error) {
	addr, err := seq.GetAddress(index)
	if err != nil {
		return nil, err
	}
	ftResp, err := seq.IotaAPI.FindTransactions(
		&giota.FindTransactionsRequest{
			Addresses: []giota.Address{addr},
		},
	)
	if err != nil {
		return nil, err
	}
	gtResp, err := seq.IotaAPI.GetTrytes(ftResp.Hashes)
	if err != nil {
		return nil, err
	}
	var spendingBundleHashes []giota.Trytes
	for _, tx := range gtResp.Trytes {
		if tx.Value < 0 && !lib.TrytesInSet(tx.Bundle, spendingBundleHashes) {
			spendingBundleHashes = append(spendingBundleHashes, tx.Bundle)
		}
	}
	ftResp, err = seq.IotaAPI.FindTransactions(
		&giota.FindTransactionsRequest{
			Bundles: spendingBundleHashes,
		},
	)
	gtResp, err = seq.IotaAPI.GetTrytes(ftResp.Hashes)
	if err != nil {
		return nil, err
	}
	spendingBundlesTx := gtResp.Trytes
	var maxTime time.Time
	var maxTail giota.Transaction

	for _, tx := range spendingBundlesTx {
		if tx.CurrentIndex == 0 && tx.Timestamp.After(maxTime) {
			maxTime = tx.Timestamp
			maxTail = tx
		}
	}
	// collect the bundle by maxTail
	bundleTx := extractBundleTxByTail(maxTail, spendingBundlesTx)
	return giota.Bundle(bundleTx), nil
}

func extractBundleTxByTail(tail giota.Transaction, allTx []giota.Transaction) []giota.Transaction {
	ret := []giota.Transaction{tail}
	next := tail.TrunkTransaction.Hash()
	count := 1
	for tx, ok := findTxByHash(next, allTx); ok && count <= len(allTx); count++ {
		ret = append(ret, tx)
	}
	// TODO consistency check. Sorting
	return ret
}

func findTxByHash(hash giota.Trytes, txList []giota.Transaction) (giota.Transaction, bool) {
	for _, tx := range txList {
		if tx.Hash() == hash {
			return tx, true
		}
	}
	return giota.Transaction{}, false
}
