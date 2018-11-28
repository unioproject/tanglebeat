package multiapi

import (
	"errors"
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/trinary"
	"net/http"
	"sync"
	"time"
)

type MultiAPI []*API

func New(endpoints []string, timeout int) (MultiAPI, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("Must be at least 1 endpoint")
	}
	if timeout <= 0 {
		return nil, errors.New("Timeout must be > 0")
	}
	ret := make(MultiAPI, 0, len(endpoints))

	for _, ep := range endpoints {
		api, err := ComposeAPI(
			HTTPClientSettings{
				URI: ep,
				Client: &http.Client{
					Timeout: time.Duration(timeout) * time.Second,
				},
			},
		)
		if err != nil {
			return nil, err
		}
		ret = append(ret, api)
	}
	return ret, nil
}

type resultGetLatestInclusion struct {
	states []bool
	err    error
	apiIdx int
}

func (mapi MultiAPI) GetLatestInclusion(transactions Hashes) ([]bool, error, int) {
	if len(mapi) == 0 {
		return nil, errors.New("empty MultiAPI"), -1
	}
	if len(mapi) == 1 {
		res, err := mapi[0].GetLatestInclusion(transactions)
		return res, err, 0
	}
	chInterimResult := make(chan *resultGetLatestInclusion)
	var wg sync.WaitGroup
	for idx, api := range mapi {
		wg.Add(1)
		go func() {
			defer wg.Done()
			states, err := api.GetLatestInclusion(transactions)
			chInterimResult <- &resultGetLatestInclusion{states: states, err: err, apiIdx: idx}
		}()
	}
	// assuming each api has timeout, all go routines has to finish anyway
	// TODO detect and report leak of goroutines/channels
	go func() {
		wg.Wait()
		close(chInterimResult)
	}()

	var result *resultGetLatestInclusion
	var waitResult sync.WaitGroup
	waitResult.Add(1)
	go func() {
		defer waitResult.Done()
		for result = range chInterimResult {
			if result.err == nil {
				return
			}
		}
	}()
	waitResult.Wait()
	return result.states, result.err, result.apiIdx
}
