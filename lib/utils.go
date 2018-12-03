package lib

import (
	"errors"
	"fmt"
	. "github.com/iotaledger/iota.go/kerl"
	. "github.com/iotaledger/iota.go/transaction"
	. "github.com/iotaledger/iota.go/trinary"
	"time"
)

func UnixMs(t time.Time) uint64 {
	return uint64(t.UnixNano()) / uint64(time.Millisecond)
}

// calculates hash of the same length
func KerlTrytes(s Trytes) (Trytes, error) {
	k := NewKerl()
	if k == nil {
		return "", errors.New(fmt.Sprintf("Couldn't initialize Kerl instance"))
	}
	var err error
	var trits Trits
	if trits, err = TrytesToTrits(s); err != nil {
		return "", err
	}
	err = k.Absorb(trits)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Absorb(_) failed: %s", err))
	}

	// squeeze same len as
	ts, err := k.Squeeze(len(trits))
	if err != nil {
		return "", errors.New(fmt.Sprintf("Squeeze() failed: %v", err))
	}
	return TritsToTrytes(ts)

}

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func TrytesInSet(a Trytes, list []Trytes) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func StringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// check consistency of the indices of the set and return error if not consistent
func CheckBundle(txSet []Transaction) error {
	if len(txSet) == 0 {
		return errors.New("BundleTrytes is empty")
	}
	filled := make([]bool, len(txSet))
	lastIndex := txSet[0].LastIndex
	bundleHash := txSet[0].Bundle
	if lastIndex+1 != uint64(len(txSet)) {
		return errors.New(fmt.Sprintf("Inconsistent LastIndex %v in the bundle", lastIndex))
	}
	for _, tx := range txSet {
		if tx.LastIndex != lastIndex {
			return errors.New(fmt.Sprintf("Inconsistent LastIndex %v in the bundle (CurrentIndex=%v)",
				tx.LastIndex, tx.CurrentIndex))
		}
		if tx.Bundle != bundleHash {
			return errors.New(fmt.Sprintf("Inconsistent BundleHash %v in the bundle (CurrentIndex=%v)",
				tx.Bundle, tx.CurrentIndex))
		}
		if tx.CurrentIndex != tx.LastIndex {
			if _, inSet := FindTxByHash(tx.TrunkTransaction, txSet); !inSet {
				return errors.New(fmt.Sprintf("Trunk chain is broken in CurrentIndex %v of the bundle",
					tx.CurrentIndex))
			}
		}
		if tx.CurrentIndex < 0 || tx.CurrentIndex > lastIndex {
			return errors.New(fmt.Sprintf("Wrong CurrentIndex %v in the bundle", tx.CurrentIndex))
		}
		if filled[tx.CurrentIndex] {
			return errors.New(fmt.Sprintf("Duplicated CurrentIndex %v in the bundle", tx.CurrentIndex))
		}
		filled[tx.CurrentIndex] = true
	}
	for _, f := range filled {
		if !f {
			return errors.New("Wrong index in the bundle")
		}
	}
	return nil
}

// check consistency of the indices of the set and return sorted slice.
// if finds inconsistency, returns same set and error
//
func CheckAndSortTxSetAsBundle(txSet []*Transaction) ([]*Transaction, error) {
	if len(txSet) == 0 {
		return nil, nil
	}
	ret := make([]*Transaction, len(txSet))
	filled := make([]bool, len(txSet))
	lastIndex := txSet[0].LastIndex
	bundleHash := txSet[0].Bundle
	if lastIndex+1 != uint64(len(txSet)) {
		return txSet, errors.New(fmt.Sprintf("Inconsistent LastIndex %v in the bundle", lastIndex))
	}
	for _, tx := range txSet {
		if tx.LastIndex != lastIndex {
			return txSet, errors.New(fmt.Sprintf("Inconsistent LastIndex %v in the bundle (CurrentIndex=%v)",
				tx.LastIndex, tx.CurrentIndex))
		}
		if tx.Bundle != bundleHash {
			return txSet, errors.New(fmt.Sprintf("Inconsistent BundleHash %v in the bundle (CurrentIndex=%v)",
				tx.Bundle, tx.CurrentIndex))
		}
		if tx.CurrentIndex != tx.LastIndex {
			if _, inSet := FindTxByHashP(tx.TrunkTransaction, txSet); !inSet {
				return txSet, errors.New(fmt.Sprintf("Trunk chain is broken in CurrentIndex %v of the bundle",
					tx.CurrentIndex))
			}
		}
		if tx.CurrentIndex < 0 || tx.CurrentIndex > lastIndex {
			return txSet, errors.New(fmt.Sprintf("Wrong CurrentIndex %v in the bundle", tx.CurrentIndex))
		}
		if filled[tx.CurrentIndex] {
			return txSet, errors.New(fmt.Sprintf("Duplicated CurrentIndex %v in the bundle", tx.CurrentIndex))
		}
		ret[tx.CurrentIndex] = tx
		filled[tx.CurrentIndex] = true
	}
	return ret, nil
}

func TransactionSetToBundleTrytes(txSet []*Transaction) ([]Trytes, error) {
	if len(txSet) == 0 {
		return nil, nil
	}
	ret := make([]Trytes, 0, len(txSet))
	for _, tx := range txSet {
		tr, err := TransactionToTrytes(tx)
		if err != nil {
			return nil, err
		}
		ret = append(ret, tr)
	}
	return ret, nil
}

// by hash find specific tx in a set of transaction
func FindTxByHash(hash Trytes, txList []Transaction) (*Transaction, bool) {
	for _, tx := range txList {
		if tx.Hash == hash {
			return &tx, true
		}
	}
	return nil, false
}

func FindTxByHashP(hash Trytes, txList []*Transaction) (*Transaction, bool) {
	for _, tx := range txList {
		if tx.Hash == hash {
			return tx, true
		}
	}
	return nil, false
}

func FindTail(txs Transactions) *Transaction {
	for i := range txs {
		if IsTailTransaction(&txs[i]) {
			return &txs[i]
		}
	}
	return nil
}

func TailFromBundleTrytes(bundleTrytes []Trytes) (*Transaction, error) {
	// find tail
	txs, err := AsTransactionObjects(bundleTrytes, nil)
	if err != nil {
		return nil, err
	}
	tail := FindTail(txs)
	if tail == nil {
		return nil, errors.New("can't find tail")
	}
	return tail, nil
}

// give the tail and transaction set, filers out from the set the bundle of that tail
// checks consistency of the bundle. Sorts it by index

func ExtractBundleTransactionsByTail(tail *Transaction, allTx []Transaction) []*Transaction {
	if !IsTailTransaction(tail) {
		return nil
	}
	// first in a bundle is tail tx
	var ret []*Transaction
	tx := tail
	count := 0
	// counting and capping steps to avoid eternal loops along trunk (impossible, I know)
	for {
		if count >= len(allTx) {
			return ret
		}
		ret = append(ret, tx)
		var ok bool
		tx, ok = FindTxByHash(tx.TrunkTransaction, allTx)
		if !ok {
			return ret
		}
		count++
	}
	return ret
}
