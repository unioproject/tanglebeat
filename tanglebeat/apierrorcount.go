// module implements API error counter and exposes it as metrics to Prometheus

package main

import (
	. "github.com/iotaledger/iota.go/api"
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"sync"
	"time"
)

type apiErrorCount struct {
	apiEndpoints    map[*API]string
	mutex           sync.Mutex
	apiErrorCounter *prometheus.CounterVec
	errorTs         []time.Time
}

var AEC *apiErrorCount

func (aec *apiErrorCount) CheckError(api *API, err error) bool {
	if err != nil {
		aec.apiErrorCounter.With(prometheus.Labels{"endpoint": aec.getEndpoint(api)}).Inc()
		aec.addError()
	}
	return err != nil
}

func (aec *apiErrorCount) RegisterAPI(api *API, endpoint string) {
	aec.mutex.Lock()
	defer aec.mutex.Unlock()
	aec.apiEndpoints[api] = endpoint
}

func init() {
	// register prometheus metrics
	AEC = &apiErrorCount{
		apiEndpoints: make(map[*API]string),
		errorTs:      make([]time.Time, 0, 10),
		apiErrorCounter: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "tanglebeat_iota_api_error_counter",
			Help: "Increases every time IOTA (iota.go) API returns an error",
		}, []string{"endpoint"}),
	}
	prometheus.MustRegister(AEC.apiErrorCounter)
	// periodically purge timestamp buffer
	go func() {
		for {
			time.Sleep(1 * time.Minute)
			AEC.cleanOldErrors()
		}
	}()
	// periodically check error counter and exit program if some exit condition is met
	go func() {
		for {
			time.Sleep(5 * time.Second)
			AEC.checkErrorCounter()
		}
	}()
}

func (aec *apiErrorCount) addError() {
	aec.mutex.Lock()
	defer aec.mutex.Unlock()
	aec.errorTs = append(aec.errorTs, time.Now())
}

func (aec *apiErrorCount) getEndpoint(api *API) string {
	aec.mutex.Lock()
	defer aec.mutex.Unlock()
	ret, ok := aec.apiEndpoints[api]
	if !ok {
		return "???"
	}
	return ret
}

func (aec *apiErrorCount) cleanOldErrors() {
	aec.mutex.Lock()
	defer aec.mutex.Unlock()

	if len(aec.errorTs) > 100 {
		ago10m := time.Now().Add(-10 * time.Minute)
		tmp := make([]time.Time, 0, len(aec.errorTs))
		for _, ts := range aec.errorTs {
			if !ts.Before(ago10m) {
				tmp = append(tmp, ts)
			}
		}
		aec.errorTs = tmp
	}
}

func (aec *apiErrorCount) checkErrorCounter() {
	aec.mutex.Lock()
	defer aec.mutex.Unlock()

	if len(aec.errorTs) > 10 {
		log.Debugf("----------------Error count = %v", len(aec.errorTs))
	}

	aec.check1minCondition()
	aec.check10minCondition()
}

func (aec *apiErrorCount) check1minCondition() {
	if !Config.ExitProgram.Enabled || !Config.ExitProgram.Exit1min.Enabled {
		return
	}
	ago1min := time.Now().Add(-1 * time.Minute)
	count := 0
	for _, ts := range aec.errorTs {
		if count >= Config.ExitProgram.Exit1min.Threshold {
			log.Criticalf("------------- Exiting program due to 'error count in 1 min exceeded %d'",
				Config.ExitProgram.Exit1min.Threshold)
			os.Exit(Config.ExitProgram.Exit1min.RC)
		}
		if ts.After(ago1min) {
			count += 1
		}
	}
}

func (aec *apiErrorCount) check10minCondition() {
	if !Config.ExitProgram.Enabled || !Config.ExitProgram.Exit1min.Enabled {
		return
	}
	ago10min := time.Now().Add(-10 * time.Minute)
	count := 0
	for _, ts := range aec.errorTs {
		if count >= Config.ExitProgram.Exit10min.Threshold {
			log.Criticalf("------------- Exiting program due to 'error count in 10 min exceeded %d'",
				Config.ExitProgram.Exit1min.Threshold)
			os.Exit(Config.ExitProgram.Exit10min.RC)
		}
		if ts.After(ago10min) {
			count += 1
		}
	}
}
