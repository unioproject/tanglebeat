package multiapi

import (
	"fmt"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/op/go-logging"
)

var log *logging.Logger

func SetLog(logger *logging.Logger) {
	log = logger
}

func debugf(format string, args ...interface{}) {
	if log != nil {
		log.Debugf(format, args...)
	}
}
func getArgs(args []interface{}) ([]interface{}, *MultiCallRet) {
	lenarg := len(args)
	if lenarg == 0 {
		return nil, nil
	}
	callRet, ok := args[lenarg-1].(*MultiCallRet)
	if ok {
		lenarg -= 1
	}
	return args[:lenarg], callRet
}

func (mapi MultiAPI) GetLatestInclusion(args ...interface{}) ([]bool, error) {
	funname := "GetLatestInclusion"
	funargs, callRet := getArgs(args)
	r, err := mapi.__multiCall__(funname, callRet, funargs...)
	if err != nil {
		return nil, err
	}
	rr, ok := r.([]bool)
	if !ok {
		return nil, fmt.Errorf("internal error: wrong type in '%v'", funname)
	}
	return rr, err
}

func (mapi MultiAPI) GetTransactionsToApprove(args ...interface{}) (*TransactionsToApprove, error) {
	funname := "GetTransactionsToApprove"
	funargs, callRet := getArgs(args)
	r, err := mapi.__multiCall__(funname, callRet, funargs...)
	if err != nil {
		return nil, err
	}
	rr, ok := r.(*TransactionsToApprove)
	if !ok {
		return nil, fmt.Errorf("internal error: wrong type in '%v'", funname)
	}
	return rr, err
}

func (mapi MultiAPI) GetBalances(args ...interface{}) (*Balances, error) {
	funname := "GetBalances"
	funargs, callRet := getArgs(args)
	r, err := mapi.__multiCall__(funname, callRet, funargs...)
	if err != nil {
		return nil, err
	}
	rr, ok := r.(*Balances)
	if !ok {
		return nil, fmt.Errorf("internal error: wrong type in '%v'", funname)
	}
	return rr, err
}

func (mapi MultiAPI) FindTransactions(args ...interface{}) (Hashes, error) {
	funname := "FindTransactions"
	funargs, callRet := getArgs(args)
	r, err := mapi.__multiCall__(funname, callRet, funargs...)
	if err != nil {
		return nil, err
	}
	rr, ok := r.(Hashes)
	if !ok {
		return nil, fmt.Errorf("internal error: wrong type in '%v'", funname)
	}
	return rr, err
}

func (mapi MultiAPI) WereAddressesSpentFrom(args ...interface{}) ([]bool, error) {
	funname := "WereAddressesSpentFrom"
	funargs, callRet := getArgs(args)
	r, err := mapi.__multiCall__(funname, callRet, funargs...)
	if err != nil {
		return nil, err
	}
	rr, ok := r.([]bool)
	if !ok {
		return nil, fmt.Errorf("internal error: wrong type in '%v'", funname)
	}
	return rr, err
}

func (mapi MultiAPI) GetTrytes(args ...interface{}) ([]Trytes, error) {
	funname := "GetTrytes"
	funargs, callRet := getArgs(args)
	r, err := mapi.__multiCall__(funname, callRet, funargs...)
	if err != nil {
		return nil, err
	}
	rr, ok := r.([]Trytes)
	if !ok {
		return nil, fmt.Errorf("internal error: wrong type in '%v'", funname)
	}
	return rr, err
}
