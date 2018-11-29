package multiapi

import (
	"errors"
	"fmt"
	. "github.com/iotaledger/iota.go/api"
	"sync"
)

func __polyCall__(api *API, funName string, args ...interface{}) (interface{}, error) {
	fun, ok := funMap[funName]
	if !ok {
		return nil, fmt.Errorf("__polyCall__: function '%v' is not implemented", funName)
	}
	return fun(api, args)
}

type multiCallInterimResult struct {
	ret      *interface{}
	err      error
	endpoint string
}

func (mapi MultiAPI) __multiCall__(funName string, retEndpoint *MultiCallRet, args ...interface{}) (interface{}, error) {
	if len(mapi) == 0 {
		return nil, errors.New("empty MultiAPI")
	}
	if len(mapi) == 1 {
		res, err := __polyCall__(mapi[0].api, funName, args...)
		if retEndpoint != nil {
			retEndpoint.Endpoint = mapi[0].endpoint
		}
		return res, err
	}
	chInterimResult := make(chan *multiCallInterimResult)
	var wg sync.WaitGroup
	for _, api := range mapi {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := __polyCall__(api.api, funName, args...)
			chInterimResult <- &multiCallInterimResult{
				ret:      &res,
				err:      err,
				endpoint: api.endpoint}
		}()
	}
	// assuming each api has timeout, all go routines has to finish anyway
	// TODO detect and report leak of goroutines/channels
	go func() {
		wg.Wait()
		close(chInterimResult)
	}()

	var result *multiCallInterimResult
	var waitResult sync.WaitGroup
	waitResult.Add(1)
	go func() {
		var res *multiCallInterimResult
		var noerr bool
		// reading all results to get all go routines finish
		for res = range chInterimResult {
			if !noerr && res.err == nil {
				result = res
				noerr = true
				waitResult.Done() // first result without err is returned
			}
		}
		if !noerr {
			result = res // if all of them with err != nil, the last one is returned
			waitResult.Done()
		}
	}()
	waitResult.Wait()
	if retEndpoint != nil {
		retEndpoint.Endpoint = result.endpoint
	}
	return *result.ret, result.err
}
