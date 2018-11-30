package multiapi

import (
	"fmt"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/trinary"
)

var funMap = map[string]func(api *API, args []interface{}) (interface{}, error){
	"GetLatestInclusion":       __getLatestInclusion__,
	"GetTransactionsToApprove": __getTransactionsToApprove__,
	"GetBalances":              __getBalances__,
	"FindTransactions":         __findTransactions__,
}

func __getLatestInclusion__(iotaapi *API, args []interface{}) (interface{}, error) {
	var ok bool
	var hashes Hashes
	if len(args) == 1 {
		hashes, ok = args[0].(Hashes)
	}
	if !ok {
		return nil, fmt.Errorf("__polyCall__ '__getLatestInclusion__': wrong arguments")
	}
	return iotaapi.GetLatestInclusion(hashes)
}

func __getTransactionsToApprove__(iotaapi *API, args []interface{}) (interface{}, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("__polyCall__ '__getTransactionsToApprove__': wrong arguments")
	}
	depth, ok := args[0].(uint64)
	if !ok {
		return nil, fmt.Errorf("__polyCall__ '__getTransactionsToApprove__': wrong first argument")
	}
	var references []Hash
	if len(args) > 1 {
		references = make([]Hash, 0, len(args)-1)
		for i := 1; i < len(args); i++ {
			h, ok := args[i].(Hash)
			if !ok {
				return nil, fmt.Errorf("__polyCall__ '__getTransactionsToApprove__': wrong arguments")
			}
			references = append(references, h)
		}
	}
	return iotaapi.GetTransactionsToApprove(depth, references...)
}

func __getBalances__(iotaapi *API, args []interface{}) (interface{}, error) {
	var ok bool
	if len(args) != 2 {
		return nil, fmt.Errorf("__polyCall__ '__getBalances__': wrong arguments. Must be exactly 2")
	}
	var addresses Hashes
	var threshold uint64
	addresses, ok = args[0].(Hashes)
	if ok {
		threshold, ok = args[1].(uint64)
	}
	if !ok {
		return nil, fmt.Errorf("__polyCall__ '__getBalances__': wrong argument types")
	}
	return iotaapi.GetBalances(addresses, threshold)
}

func __findTransactions__(iotaapi *API, args []interface{}) (interface{}, error) {
	var ok bool
	if len(args) != 1 {
		return nil, fmt.Errorf("__polyCall__ '__findTransactions__': wrong number of arguments. Must be exactly 1")
	}
	query, ok := args[0].(FindTransactionsQuery)
	if !ok {
		return nil, fmt.Errorf("__polyCall__ '__findTransactions__': wrong argument type")
	}
	return iotaapi.FindTransactions(query)
}
