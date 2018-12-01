package multiapi

import (
	"errors"
	"fmt"
	. "github.com/iotaledger/iota.go/api"
	"sync"
	"time"
)

var disabledMultiAPI = false

func DisableMultiAPI() {
	disabledMultiAPI = true
}

func MultiApiDisabled() bool {
	return disabledMultiAPI
}

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
	started := time.Now()
	if len(mapi) == 0 {
		return nil, errors.New("empty MultiAPI")
	}
	if len(mapi) == 1 || MultiApiDisabled() {
		res, err := __polyCall__(mapi[0].api, funName, args...)
		if err != nil {
			return nil, err
		}
		if retEndpoint != nil {
			retEndpoint.Endpoint = mapi[0].endpoint
			retEndpoint.Duration = time.Since(started)
		}
		return res, err
	}
	chInterimResult := make(chan *multiCallInterimResult)
	var wg sync.WaitGroup
	for i := range mapi {
		wg.Add(1)
		// need variable for go routine!!!! https://github.com/golang/go/wiki/CommonMistakes
		go func(idx int) {
			defer wg.Done()
			res, err := __polyCall__(mapi[idx].api, funName, args...)
			chInterimResult <- &multiCallInterimResult{
				ret:      &res,
				err:      err,
				endpoint: mapi[idx].endpoint}
		}(i)
	}
	// assuming each api has timeout, all go routines has to finish anyway
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
		// reading all results to make all go routines to finish
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
		retEndpoint.Duration = time.Since(started)
	}
	return *result.ret, result.err
}
