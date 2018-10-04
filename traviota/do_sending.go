package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
	"sync"
	"time"
)

// Performs (re)attachments and promotions

type sendingState struct {
	lastBundle            giota.Bundle
	lastAttachmentTime    time.Time
	nextForceReattachTime time.Time
	lastPromoTime         time.Time
	nextPromoTime         time.Time
	lastPromoBundle       giota.Bundle
}

func (seq *Sequence) DoSending(index int) func() {
	chCancel := make(chan struct{})

	var wg sync.WaitGroup
	go func() {
		seq.log.Debugf("Started DoSending routine for idx = %v", index)
		defer seq.log.Debugf("Finished DoSending routine for idx = %v", index)

		wg.Add(1)
		defer wg.Done()

		var err error
		var state sendingState

		// in case of errors will keep retrieving latest bundle
		// can return empty bundle without error if sending just started or after snapshot
		for found := false; !found; {
			state.lastBundle, err = seq.findLatestSpendingBundle(index)
			found = err == nil
			if !found {
				seq.log.Errorf("Error while retrieving latest spending bundle for index=%v:  %v", index, err)
				time.Sleep(10 * time.Second)
			} else {
				if len(state.lastBundle) > 0 {
					state.lastAttachmentTime = state.lastBundle[0].Timestamp
					state.nextForceReattachTime =
						state.lastAttachmentTime.Add(time.Duration(seq.Params.ForceReattachAfterMin) * time.Minute)
				}
			}
		}
		// main sending loop. Only programmatically can be interrupted by cancel function
		var nextState *sendingState
		for {
			nextState, err = seq.doSendingAction(index, &state)
			if err != nil {
				seq.log.Errorf("DoSending: %v", err)
			} else {
				state = *nextState
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

// performs action if it is time. Returns changed state
// otherwise return nil state
func (seq *Sequence) doSendingAction(index int, state *sendingState) (*sendingState, error) {

	if len(state.lastBundle) == 0 {
		// can be in the beginning of sending or after snapshot
		ret, err := seq.sendToNext(index, state)
		if err != nil {
			return nil, err
		}
		return ret, nil
	}
	if len(state.lastPromoBundle) != 0 {
		// promo already started.
		if time.Now().After(state.nextPromoTime) {
			if seq.Params.PromoteChain {
				if ret, err := seq.promoteOrReattach(&state.lastPromoBundle[0], state); err != nil {
					return nil, err
				} else {
					return ret, nil
				}
			} else {
				// promote blowball
				if ret, err := seq.promoteOrReattach(&state.lastBundle[0], state); err != nil {
					return nil, err
				} else {
					return ret, nil
				}
			}
		}
	}
	// promoteOrReattach conditions wasn't met. Check if reattachment is needed
	consistent, err := seq.checkConsistency(state.lastBundle[0].Hash())
	if err != nil {
		return nil, err
	}
	if !consistent {
		seq.log.Debugf("Last bundle is inconsistent. index %v", index)
	}
	if time.Now().After(state.nextForceReattachTime) {
		seq.log.Debugf("Time for forced reattachment after %v min. index=%v", seq.Params.ForceReattachAfterMin, index)
	}
	if !consistent || time.Now().After(state.nextForceReattachTime) {
		ret, err := seq.reattach(&state.lastBundle[0], state)
		if err != nil {
			return nil, err
		}
		return ret, nil
	}
	return nil, nil
}

func (seq *Sequence) promoteOrReattach(tx *giota.Transaction, state *sendingState) (*sendingState, error) {
	var consistent bool
	var err error
	if consistent, err = seq.checkConsistency(tx.Hash()); err != nil {
		return nil, err
	}
	if consistent {
		ret, err := seq.promote(tx, state)
		if err != nil {
			return nil, err
		}
		return ret, nil
	}
	// can't promote --> reattach
	ret, err := seq.reattach(tx, state)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (seq *Sequence) findTrytes(txReq *giota.FindTransactionsRequest) (*giota.GetTrytesResponse, error) {
	// TODO tx cache
	ftResp, err := seq.IotaAPI.FindTransactions(txReq)
	if err != nil {
		return nil, err
	}
	return seq.IotaAPI.GetTrytes(ftResp.Hashes)
}

func (seq *Sequence) findLatestSpendingBundle(index int) (giota.Bundle, error) {
	addr, err := seq.GetAddress(index)
	if err != nil {
		return nil, err
	}
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

	for _, tx := range spendingBundlesTx {
		if tx.CurrentIndex == 0 && tx.Timestamp.After(maxTime) {
			maxTime = tx.Timestamp
			maxTail = tx
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
	return giota.Bundle(bundleTx), nil
}

// give the tail and transaction set, filers out from the set the bundle of that tail
// checks consistency of the bundle. Sorts it by index

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
