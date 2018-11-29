package multiapi

import (
	"errors"
	. "github.com/iotaledger/iota.go/api"
	"net/http"
	"time"
)

type endpointEntry struct {
	api      *API
	endpoint string
}

type MultiAPI []endpointEntry

func New(endpoints []string, timeout int) (MultiAPI, error) {
	if len(endpoints) == 0 {
		return nil, errors.New("Must be at least 1 Endpoint")
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
		ret = append(ret, endpointEntry{api: api, endpoint: ep})
	}
	return ret, nil
}
