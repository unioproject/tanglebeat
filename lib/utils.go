package lib

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
	"time"
)

func UnixMs(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}

// calculates hash of the same length
func KerlTrytes(s giota.Trytes) (giota.Trytes, error) {
	k := giota.NewKerl()
	if k == nil {
		return "", errors.New(fmt.Sprintf("Couldn't initialize Kerl instance"))
	}
	trits := s.Trits()
	err := k.Absorb(trits)
	if err != nil {
		return "", errors.New(fmt.Sprintf("Absorb(_) failed: %s", err))
	}

	// squeeze same len as
	ts, err := k.Squeeze(len(trits))
	if err != nil {
		return "", errors.New(fmt.Sprintf("Squeeze() failed: %v", err))
	}
	return ts.Trytes(), nil
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

func TrytesInSet(a giota.Trytes, list []giota.Trytes) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

// check consistency of the indices of the set and return sorted slice.
// if finds inconsistency, returns same set and error
func CheckAndSortBundle(txSet []giota.Transaction) ([]giota.Transaction, error) {
	if len(txSet) == 0 {
		return nil, nil
	}
	ret := make([]giota.Transaction, len(txSet))
	filled := make([]bool, len(txSet))
	lastIndex := txSet[0].LastIndex
	bundleHash := txSet[0].Bundle
	if lastIndex+1 != int64(len(txSet)) {
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
			if _, inSet := FindTxByHash(tx.TrunkTransaction, txSet); !inSet {
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

// by hash find specific tx in a set of transaction
func FindTxByHash(hash giota.Trytes, txList []giota.Transaction) (giota.Transaction, bool) {
	for _, tx := range txList {
		if tx.Hash() == hash {
			return tx, true
		}
	}
	return giota.Transaction{}, false
}

func GetTail(bundle giota.Bundle) *giota.Transaction {
	for _, tx := range bundle {
		if tx.CurrentIndex == 0 {
			return &tx
		}
	}
	return nil
}
