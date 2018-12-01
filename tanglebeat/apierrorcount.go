// module implements API error counter and exposes it as metrics to Prometheus

package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"os"
	"sync"
	"time"
)

type apiErrorCount struct {
	apiErrorCounter *prometheus.CounterVec
	mutex           sync.Mutex
	errorTs         []time.Time
}

var AEC *apiErrorCount

func (aec *apiErrorCount) CheckError(endpoint string, err error) bool {
	if err != nil {
		aec.apiErrorCounter.With(prometheus.Labels{"endpoint": endpoint}).Inc()
		aec.addError()
	}
	return err != nil
}

func init() {
	// register prometheus metrics
	AEC = &apiErrorCount{
		errorTs: make([]time.Time, 0, 10),
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

func (aec *apiErrorCount) cleanOldErrors() {
	aec.mutex.Lock()
	defer aec.mutex.Unlock()

	if len(aec.errorTs) > 100 {
		tmp := make([]time.Time, 0, len(aec.errorTs))
		for _, ts := range aec.errorTs {
			if time.Since(ts) < 10*time.Minute {
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
	if !Config.ExitProgram.Enabled {
		return
	}
	if !Config.ExitProgram.Exit1min.Enabled {
		return
	}
	count := 0
	for _, ts := range aec.errorTs {
		if count >= Config.ExitProgram.Exit1min.Threshold {
			log.Criticalf("------------- Exiting program due to 'error count in 1 min exceeded %d' RC = %d",
				Config.ExitProgram.Exit1min.Threshold, Config.ExitProgram.Exit1min.RC)
			os.Exit(Config.ExitProgram.Exit1min.RC)
		}
		if time.Since(ts) < 1*time.Minute {
			count += 1
		}
	}
}

func (aec *apiErrorCount) check10minCondition() {
	if !Config.ExitProgram.Enabled {
		return
	}
	if !Config.ExitProgram.Exit10min.Enabled {
		return
	}
	count := 0
	for _, ts := range aec.errorTs {
		if count >= Config.ExitProgram.Exit10min.Threshold {
			log.Criticalf("------------- Exiting program due to 'error count in 10 min exceeded %d' RC = %v",
				Config.ExitProgram.Exit10min.Threshold, Config.ExitProgram.Exit10min.RC)
			os.Exit(Config.ExitProgram.Exit10min.RC)
		}
		if time.Since(ts) < 10*time.Minute {
			count += 1
		}
	}
}
