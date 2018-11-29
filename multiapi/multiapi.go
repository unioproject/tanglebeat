package multiapi

import (
	"fmt"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/trinary"
)

func (mapi MultiAPI) GetLatestInclusion(transactions Hashes, ret ...*MultiCallRet) ([]bool, error) {
	var callRet *MultiCallRet
	switch len(ret) {
	case 0:
		callRet = nil
	case 1:
		callRet = ret[0]
	default:
		return nil, fmt.Errorf("wrong number of arguments")
	}
	r, err := mapi.__multiCall__("GetLatestInclusion", callRet, transactions)
	rr, ok := r.([]bool)
	if !ok {
		return nil, fmt.Errorf("internal error: wrong type")
	}
	return rr, err
}

func (mapi MultiAPI) GetTransactionsToApprove(depth uint64, args ...interface{}) (*TransactionsToApprove, error) {
	var callRet *MultiCallRet
	// strip the last one if necessary
	lenArgs := len(args)
	if lenArgs > 0 {
		var ok bool
		callRet, ok = args[len(args)-1].(*MultiCallRet)
		// callRet == nil if not provided as last arg
		if ok {
			lenArgs = len(args) - 1
		}
	}
	r, err := mapi.__multiCall__("GetTransactionsToApprove", callRet, args[:lenArgs]...)
	rr, ok := r.(*TransactionsToApprove)
	if !ok {
		return nil, fmt.Errorf("internal error: wrong type")
	}
	return rr, err
}
