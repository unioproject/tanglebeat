package multiapi

import (
	"errors"
	. "github.com/iotaledger/iota.go/trinary"
	"sync"
)

type MultiAPIGetLatestInclusionResult struct {
	States   []bool
	Err      error
	Endpoint string
}

func (mapi MultiAPI) GetLatestInclusion(transactions Hashes, resOrig ...*MultiAPIGetLatestInclusionResult) ([]bool, error) {
	if len(mapi) == 0 {
		return nil, errors.New("empty MultiAPI")
	}
	if len(mapi) == 1 {
		res, err := mapi[0].api.GetLatestInclusion(transactions)
		if len(resOrig) > 0 {
			*resOrig[0] = MultiAPIGetLatestInclusionResult{
				States:   res,
				Err:      err,
				Endpoint: mapi[0].endpoint,
			}
		}
		return res, err
	}
	chInterimResult := make(chan *MultiAPIGetLatestInclusionResult)
	var wg sync.WaitGroup
	for _, api := range mapi {
		wg.Add(1)
		go func() {
			defer wg.Done()
			states, err := api.api.GetLatestInclusion(transactions)
			chInterimResult <- &MultiAPIGetLatestInclusionResult{
				States:   states,
				Err:      err,
				Endpoint: api.endpoint}
		}()
	}
	// assuming each api has timeout, all go routines has to finish anyway
	// TODO detect and report leak of goroutines/channels
	go func() {
		wg.Wait()
		close(chInterimResult)
	}()

	var result *MultiAPIGetLatestInclusionResult
	var waitResult sync.WaitGroup
	waitResult.Add(1)
	go func() {
		var res *MultiAPIGetLatestInclusionResult
		var noerr bool
		// reading all results to get all go routines finish
		for res = range chInterimResult {
			if !noerr && res.Err == nil {
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
	if len(resOrig) > 0 {
		*resOrig[0] = *result
	}
	return result.States, result.Err
}
