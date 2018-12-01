package lib

import (
	"errors"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/transaction"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/multiapi"
)

// checks if transaction in any bundle with specified hash is confirmed
const maxTxHashesForGLI = 100 // max number of hashes in one call to GetLatestInclusion

func IsAnyConfirmed(transactions Hashes, api *API) (bool, error) {
	var upper int
	for i := 0; i < len(transactions); i += maxTxHashesForGLI {
		upper = i + maxTxHashesForGLI
		if upper > len(transactions) {
			upper = len(transactions)
		}
		states, err := api.GetLatestInclusion(transactions[i:upper])
		if err != nil {
			return false, err
		}
		for _, conf := range states {
			if conf {
				return true, nil
			}
		}
	}
	return false, nil

}

func IsBundleHashConfirmed(bundleHash Trytes, api *API) (bool, error) {
	respHashes, err := api.FindTransactions(FindTransactionsQuery{
		Bundles: Hashes{bundleHash},
	})
	if err != nil {
		return false, err
	}
	return IsAnyConfirmed(respHashes, api)
}

const maxTxHashesForGetTrytes = 50 // max number of hashes in one call to GetTrytes

// Finds transactions, loads trytes (in pieces if necessary) and parses to transactions
func FindTransactionObjects(query FindTransactionsQuery, api *API) (Transactions, error) {
	ftHashes, err := api.FindTransactions(query)
	if err != nil {
		return nil, err
	}
	if len(ftHashes) == 0 {
		return nil, errors.New("no transactions found")
	}
	ret := make([]Transaction, len(ftHashes))
	idx := 0
	var upper int
	for i := 0; i < len(ftHashes); i += maxTxHashesForGetTrytes {
		upper = i + maxTxHashesForGetTrytes
		if upper > len(ftHashes) {
			upper = len(ftHashes)
		}
		rawTrytes, err := api.GetTrytes(ftHashes[i:upper]...)
		if err != nil {
			return nil, err
		}
		for i := range rawTrytes {
			ptx, err := AsTransactionObject(rawTrytes[i])
			if err != nil {
				return nil, err
			}
			ret[idx] = *ptx
			idx++
		}
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func FindTransactionObjectsMulti(query FindTransactionsQuery, api multiapi.MultiAPI) (Transactions, error) {
	ftHashes, err := api.FindTransactions(query)
	if err != nil {
		return nil, err
	}
	if len(ftHashes) == 0 {
		return nil, errors.New("no transactions found")
	}
	ret := make([]Transaction, len(ftHashes))
	idx := 0
	var upper int
	for i := 0; i < len(ftHashes); i += maxTxHashesForGetTrytes {
		upper = i + maxTxHashesForGetTrytes
		if upper > len(ftHashes) {
			upper = len(ftHashes)
		}
		// TODO with multi api
		rawTrytes, err := api.GetAPI().GetTrytes(ftHashes[i:upper]...)
		if err != nil {
			return nil, err
		}
		for i := range rawTrytes {
			ptx, err := AsTransactionObject(rawTrytes[i])
			if err != nil {
				return nil, err
			}
			ret[idx] = *ptx
			idx++
		}
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}
