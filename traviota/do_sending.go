package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
	"sync"
	"time"
)

// Performs (re)attachments and promotions

type sendingState struct {
	index                 int
	addr                  giota.Address
	balance               int64
	lastBundle            giota.Bundle
	lastAttachmentTime    time.Time
	nextForceReattachTime time.Time
	lastPromoTime         time.Time
	nextPromoTime         time.Time
	lastPromoBundle       giota.Bundle
}

// starts sending routine. Returns stop function
func (seq *Sequence) DoSending(stateOrig *sendingState) func() *sendingState {
	state := *stateOrig
	chStop := make(chan struct{})
	var wg sync.WaitGroup
	go func() {
		seq.log.Debugf("Started DoSending routine for idx = %v, %v", state.index, state.addr)
		defer seq.log.Debugf("Finished DoSending routine for idx = %v, %v", state.index, state.addr)

		publishStartSending(state.index, state.addr)
		defer publishFinishSending(state.index, state.addr)

		wg.Add(1)
		defer wg.Done()

		var err error

		// in case of errors will keep retrieving latest bundle
		// can return empty bundle without error if sending just started or after snapshot
		for found := false; !found; {
			state.lastBundle, err = seq.findLatestSpendingBundle(&state)
			found = err == nil
			if !found {
				seq.log.Errorf("Error while retrieving latest spending bundle for index=%v:  %v", state.index, err)
				time.Sleep(10 * time.Second)
			} else {
				if len(state.lastBundle) > 0 {
					// TODO use AttachmentTimestamp
					state.lastAttachmentTime = state.lastBundle[0].Timestamp
					state.nextForceReattachTime =
						state.lastAttachmentTime.Add(time.Duration(seq.Params.ForceReattachAfterMin) * time.Minute)
				}
			}
		}
		// main sending loop. Programmatically can be interrupted only with cancel function
		var nextState *sendingState
		for {
			nextState, err = seq.doSendingAction(&state)
			if err != nil {
				seq.log.Errorf("DoSendingAction returned: %v", err)
			} else {
				if nextState != nil {
					state = *nextState
				}
			}
			select {
			case <-chStop:
				return
			case <-time.After(5 * time.Second):
			}
		}
		// update original structure
	}()
	return func() *sendingState {
		close(chStop)
		wg.Wait()
		return &state
	}
}

// performs action if it is time. Returns changed state
// otherwise return nil state
func (seq *Sequence) doSendingAction(state *sendingState) (*sendingState, error) {
	index := state.index
	if len(state.lastBundle) == 0 {
		// can be in the beginning of sending or after snapshot
		seq.log.Debugf("Didn't find spending bundle. idx = %v. Sending balance to the next", index)
		ret, err := seq.sendToNext(state)
		if err != nil {
			return nil, err
		}
		return ret, nil
	}
	confirmed, err := seq.isConfirmed(state.lastBundle[0].Hash())
	if err != nil {
		return nil, err
	}
	if confirmed {
		balance, err := seq.GetBalanceAddr([]giota.Address{state.addr})
		if err != nil {
			return nil, err
		}
		if balance[0] == 0 {
			// there's nothing to do, if balance = 0 and last bundle is confirmed
			seq.log.Debugf("balance = 0, last bundle confirmed -> all done, no action. idx=%v, addr=%v", index, state.addr)
		} else {
			// exotic situation: iotas have been sent to the address while current sending. But it happens!
			seq.log.Debugf("balance > 0, last bundle confirmed -> sending remaining balance. idx=%v, addr=%v", index, state.addr)
			ret, err := seq.sendToNext(state)
			if err != nil {
				return nil, err
			}
			return ret, nil
		}
	}
	if len(state.lastPromoBundle) != 0 {
		// promo already started.
		if time.Now().After(state.nextPromoTime) {
			if seq.Params.PromoteNoChain {
				// promote blowball
				seq.log.Debugf("Promote 'blowball' idx=%v, txh=%v", index, state.lastBundle[0].Hash())
				if ret, err := seq.promoteOrReattach(&state.lastBundle[0], state); err != nil {
					return nil, err
				} else {
					return ret, nil
				}
			} else {
				// promote chain
				seq.log.Debugf("Promote 'chain' idx=%v, txh=%v", index, state.lastPromoBundle[0].Hash())
				if ret, err := seq.promoteOrReattach(&state.lastPromoBundle[0], state); err != nil {
					return nil, err
				} else {
					return ret, nil
				}
			}
		}
	} else {
		seq.log.Debugf("Didn't find last promotion bundle, starting new promo chain. idx=%v", index)
		if ret, err := seq.promoteOrReattach(&state.lastBundle[0], state); err != nil {
			return nil, err
		} else {
			return ret, nil
		}
	}
	// promoteOrReattach conditions wasn't met. Check if reattachment is needed
	consistent, err := seq.checkConsistency(state.lastBundle[0].Hash())
	if err != nil {
		return nil, err
	}
	if !consistent {
		seq.log.Debugf("Last bundle is inconsistent. idx=%v", index)
	}
	if time.Now().After(state.nextForceReattachTime) {
		seq.log.Infof("It's time for forced reattachment after %v min. idx=%v", seq.Params.ForceReattachAfterMin, index)
	}
	if !consistent || time.Now().After(state.nextForceReattachTime) {
		seq.log.Debugf("Reattach idx=%v, txh %v", index, state.lastBundle[0].Hash())
		ret, err := seq.reattach(state)
		if err != nil {
			return nil, err
		}
		return ret, nil
	}
	seq.log.Debugf("No action, wait. idx=%v", index)
	return nil, nil
}

func (seq *Sequence) promoteOrReattach(tx *giota.Transaction, state *sendingState) (*sendingState, error) {
	var consistent bool
	var err error
	if consistent, err = seq.checkConsistency(tx.Hash()); err != nil {
		return nil, err
	}
	if consistent {
		seq.log.Debugf("Promote. idx=%v, tail %v", state.index, tx.Hash())
		ret, err := seq.promote(tx, state)
		if err != nil {
			return nil, err
		}
		return ret, nil
	}
	// can't promote --> reattach
	seq.log.Debugf("Reattach latest bundle. idx=%v", state.index)
	ret, err := seq.reattach(state)
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

func (seq *Sequence) findLatestSpendingBundle(state *sendingState) (giota.Bundle, error) {
	index := state.index
	addr := state.addr

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
