package lib

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
)

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
