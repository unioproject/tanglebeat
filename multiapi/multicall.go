package multiapi

import (
	"errors"
	"fmt"
	. "github.com/iotaledger/iota.go/api"
	"math/rand"
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

func __polyCall__(api *API, funName string, args []interface{}) (interface{}, error) {
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

func (mapi MultiAPI) __callFirst__(funName string, apiret *MultiCallRet, args []interface{}) (interface{}, error) {
	rnd := rand.Int() % 10000
	debugf("+++++++++++ multiCall %d: '%v' - calling first endpoint", rnd, funName)
	st := time.Now()

	ret, err := __polyCall__(mapi[0].api, funName, args)

	apiret.Endpoint = mapi[0].endpoint
	apiret.Duration = time.Since(st)
	debugf("+++++++++++ multiCall %d: '%v' finished '%v', %v err = '%v'",
		rnd, funName, apiret.Endpoint, apiret.Duration, err)
	return ret, err
}

func (mapi MultiAPI) __multiCall__(funName string, retEndpoint *MultiCallRet, args []interface{}) (interface{}, error) {
	if len(mapi) == 0 {
		return nil, errors.New("empty MultiAPI")
	}
	var apiret MultiCallRet
	if len(mapi) == 1 || MultiApiDisabled() {
		// if there's one endpoint or multiapi is disabled, calling the first in the list
		return mapi.__callFirst__(funName, &apiret, args)
	}

	rnd := rand.Int() % 10000
	debugf("+++++++++++ multiCall %d: '%v'", rnd, funName)

	started := time.Now()
	chInterimResult := make(chan *multiCallInterimResult)
	var wg sync.WaitGroup
	for i := range mapi {
		wg.Add(1)
		// need variable for go routine!!!! https://github.com/golang/go/wiki/CommonMistakes
		go func(idx int) {
			defer wg.Done()
			res, err := __polyCall__(mapi[idx].api, funName, args)
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
	apiret.Endpoint = result.endpoint
	apiret.Duration = time.Since(started)
	if retEndpoint != nil {
		*retEndpoint = apiret
	}
	debugf("+++++++++++ multiCall %d: %v finished '%v', %v err = '%v'",
		rnd, funName, apiret.Endpoint, apiret.Duration, result.err)
	return *result.ret, result.err
}
