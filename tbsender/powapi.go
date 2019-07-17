package main

import (
	. "github.com/iotaledger/iota.go/api"
	"github.com/unioproject/tanglebeat/lib/multiapi"
	powsrvio "gitlab.com/powsrv.io/go/client"
)

// see https://powsrv.io/
// Install:
// go get -d -u github.com/golang/protobuf/protoc-gen-go
// go get -u gitlab.com/powsrv.io/go/client

func createPoWAPI(params *senderParamsYAML) (multiapi.MultiAPI, error) {
	var ret multiapi.MultiAPI
	var err error

	if !params.UsePOWService {
		ret, err = multiapi.New([]string{params.IOTANodePoW}, params.TimeoutPoW)
		if err != nil {
			return nil, err
		}
	} else {
		ret, err = getGlbPoWServiceAPI(params.EndpointPOW, params.ApiKeyPOWService)
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// only one powsrw.io client

const powTimeoutMs = 15000

func createLocalPoWServiceAPI(endpointPOW, apiKeyPOWService string) (multiapi.MultiAPI, error) {
	var err error
	powClient := &powsrvio.PowClient{
		APIKey:        apiKeyPOWService,
		ReadTimeOutMs: powTimeoutMs,
		Verbose:       false,
	}

	// init powsrv.io
	err = powClient.Init()
	if err != nil {
		return nil, err
	}

	// create a new API instance
	var api *API
	api, err = ComposeAPI(HTTPClientSettings{
		URI:                  endpointPOW,
		LocalProofOfWorkFunc: powClient.PowFunc,
	})
	if err != nil {
		return nil, err
	}
	return multiapi.NewFromAPI(api, endpointPOW)
}

var glbPoWAPI multiapi.MultiAPI

func getGlbPoWServiceAPI(args ...string) (multiapi.MultiAPI, error) {
	if glbPoWAPI != nil {
		return glbPoWAPI, nil
	}
	if len(args) != 2 {
		panic("wrong args in the call to 'getGlbPoWServiceAPI'")
	}
	endpointPOW := args[0]
	apiKeyPOWService := args[1]

	ret, err := createLocalPoWServiceAPI(endpointPOW, apiKeyPOWService)
	if err != nil {
		return nil, err
	}
	glbPoWAPI = ret
	return glbPoWAPI, nil
}
