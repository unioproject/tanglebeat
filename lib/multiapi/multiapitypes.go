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

type MultiCallRet struct {
	Endpoint string
	Duration time.Duration
	Info     string // used by CheckConsistency
}

func NewFromAPI(api *API, endpoint string) (MultiAPI, error) {
	return MultiAPI{endpointEntry{
		api:      api,
		endpoint: endpoint,
	}}, nil
}

func New(endpoints []string, timeout uint64) (MultiAPI, error) {
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

func (mapi MultiAPI) GetAPI() *API {
	if len(endpoints4test) == 0 {
		return nil
	}
	return mapi[0].api
}

func (mapi MultiAPI) GetAPIEndpoint() string {
	if len(endpoints4test) == 0 {
		return "???"
	}
	return mapi[0].endpoint
}
