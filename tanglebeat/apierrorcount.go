// module implements API error counter and exposes it as metrics to Prometheus

package main

import (
	. "github.com/iotaledger/iota.go/api"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"
)

type apiErrorCount struct {
	apiEndpoints      map[*API]string
	apiEndpointsMutex sync.Mutex
	apiErrorCounter   *prometheus.CounterVec
}

var AEC *apiErrorCount

// TODO restart the whole program if some conditions are met

var errorTs = make([]time.Time, 0, 10)
var mutex sync.Mutex

func init() {
	AEC = &apiErrorCount{
		apiEndpoints: make(map[*API]string),
		apiErrorCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tanglebeat_iota_api_error_counter",
			Help: "Increases every time IOTA (giota) API returns an error",
		}, []string{"endpoint"}),
	}
	prometheus.MustRegister(AEC.apiErrorCounter)
}

var th = 100

func addError() {
	mutex.Lock()
	defer mutex.Unlock()
	errorTs = append(errorTs, time.Now())
	if len(errorTs) > th {
		ago10m := time.Now().Add(-10 * time.Minute)
		tmp := make([]time.Time, 0, len(errorTs))
		for _, ts := range errorTs {
			if !ts.Before(ago10m) {
				tmp = append(tmp, ts)
			}
		}
		errorTs = tmp
	}
	if len(errorTs) > 10 {
		log.Debugf("----------------Error count = %v", len(errorTs))
	}
}

func (aec *apiErrorCount) registerAPI(api *API, endpoint string) {
	aec.apiEndpointsMutex.Lock()
	defer aec.apiEndpointsMutex.Unlock()
	aec.apiEndpoints[api] = endpoint
}

func (aec *apiErrorCount) getEndpoint(api *API) string {
	aec.apiEndpointsMutex.Lock()
	defer aec.apiEndpointsMutex.Unlock()
	ret, ok := aec.apiEndpoints[api]
	if !ok {
		return "???"
	}
	return ret
}

func (aec *apiErrorCount) CountError(api *API, err error) bool {
	if err != nil {
		aec.apiErrorCounter.With(prometheus.Labels{"endpoint": aec.getEndpoint(api)}).Inc()
		addError()
	}
	return err != nil
}
