// module implements API error counter and exposes it as metrics to Prometheus

package main

import (
	"github.com/lunfardo314/giota"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
)

type apiErrorCount struct {
	apiEndpoints      map[*giota.API]string
	apiEndpointsMutex sync.Mutex
	apiErrorCounter   *prometheus.CounterVec
}

var AEC *apiErrorCount

func init() {
	AEC = &apiErrorCount{
		apiEndpoints: make(map[*giota.API]string),
		apiErrorCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tanglebeat_iota_api_error_counter",
			Help: "Increases every time IOTA (giota) API returns an error",
		}, []string{"endpoint"}),
	}
	prometheus.MustRegister(AEC.apiErrorCounter)
}

func (aec *apiErrorCount) registerAPI(api *giota.API, endpoint string) {
	aec.apiEndpointsMutex.Lock()
	defer aec.apiEndpointsMutex.Unlock()
	aec.apiEndpoints[api] = endpoint
}

func (aec *apiErrorCount) getEndpoint(api *giota.API) string {
	aec.apiEndpointsMutex.Lock()
	defer aec.apiEndpointsMutex.Unlock()
	ret, ok := aec.apiEndpoints[api]
	if !ok {
		return "???"
	}
	return ret
}

func (aec *apiErrorCount) IncErrorCount(api *giota.API) {
	aec.apiErrorCounter.With(prometheus.Labels{"endpoint": aec.getEndpoint(api)}).Inc()
}
