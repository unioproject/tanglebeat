//  standalone agent. Produces ZMQ based metrics for Tanglebeat

package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/metricszmq"
	"github.com/op/go-logging"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
)

const (
	version   = "0.0"
	logFormat = "%{time:2006-01-02 15:04:05.000} %{level:.4s} [%{module:.6s}|%{shortfunc:.12s}] %{message}"
	level     = logging.DEBUG
)

var (
	log   *logging.Logger
	hosts = []string{
		"tcp://snap1.iota.partners:5556",
		"tcp://snap2.iota.partners:5556",
		"tcp://snap3.iota.partners:5556",
		"tcp://node.iotalt.com:31415",
	}
)

// TODO 2 dynamic selection of zmq hosts

func init() {
	log = logging.MustGetLogger("main")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	logFormat := logging.MustStringFormatter(logFormat)
	backendFormatter := logging.NewBackendFormatter(backend, logFormat)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "main")
	log.SetBackend(backendLeveled)
}

func main() {
	log.Infof("Starting 'tbzmq', ZMQ metrics collector for Tanglebeat")
	metricszmq.InitMetricsZMQ(log, nil)
	for _, uri := range hosts {
		metricszmq.RunZMQMetricsFor(uri)
	}

	http.Handle("/metrics", promhttp.Handler())
	listenAndServeOn := fmt.Sprintf(":%d", 8081)
	log.Infof("Exposing Prometheus metrics on %v", listenAndServeOn)
	panic(http.ListenAndServe(listenAndServeOn, nil))
}
