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
		// create api endpoint as in https://powsrv.io/
		ret, err = createPoWServiceAPI(params.EndpointPOW, params.ApiKeyPOWService)
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

// only one powsrw.io client

var powClient *powsrvio.PowClient

func createPoWServiceAPI(endpointPOW, apiKeyPOWService string) (multiapi.MultiAPI, error) {
	var err error
	if powClient == nil {
		powClientTmp := &powsrvio.PowClient{
			APIKey:        apiKeyPOWService,
			ReadTimeOutMs: 5000,
			Verbose:       false,
		}

		// init powsrv.io
		err = powClientTmp.Init()
		if err != nil {
			return nil, err
		}
		powClient = powClientTmp
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
