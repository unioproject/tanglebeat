//  standalone agent. Produces ZMQ based metrics for Tanglebeat

package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/config"
	"github.com/lunfardo314/tanglebeat/metricszmq"
	"github.com/op/go-logging"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"os"
)

const (
	version   = "0.2"
	logFormat = "%{time:2006-01-02 15:04:05.000} %{level:.4s} [%{module:.6s}|%{shortfunc:.12s}] %{message}"
	level     = logging.DEBUG
)

var (
	log            *logging.Logger
	logInitialized bool
)

type ConfigStructYAML struct {
	siteDataDir      string
	ScrapeTargetPort int      `yaml:"prometheusScrapeTargetPort"`
	ZMQUri           []string `yaml:"zmqUri"`
}

var Config = ConfigStructYAML{}

// TODO 2 dynamic selection of zmq hosts

func initLogging(msgBeforeLog []string) ([]string, bool) {
	log = logging.MustGetLogger("main")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	logFormat := logging.MustStringFormatter(logFormat)
	backendFormatter := logging.NewBackendFormatter(backend, logFormat)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "main")
	log.SetBackend(backendLeveled)
	logInitialized = true
	return msgBeforeLog, true
}

func main() {
	msgBeforeLog := make([]string, 0, 10)
	msgBeforeLog = append(msgBeforeLog, "---- Starting 'tbzmq', ZMQ metrics collector for Tanglebeat ver. "+version)
	var success bool
	var siteDataDir string
	msgBeforeLog, siteDataDir, success = config.ReadYAML("tbzmq.yml", msgBeforeLog, &Config)
	if !success {
		flushMsgBeforeLog(msgBeforeLog)
		os.Exit(1)
	}
	Config.siteDataDir = siteDataDir
	msgBeforeLog, success = initLogging(msgBeforeLog)
	flushMsgBeforeLog(msgBeforeLog)
	if !success {
		os.Exit(1)
	}

	metricszmq.InitMetricsZMQ(log, nil)
	for _, uri := range Config.ZMQUri {
		metricszmq.RunZMQMetricsFor(uri)
	}

	http.Handle("/metrics", promhttp.Handler())
	listenAndServeOn := fmt.Sprintf(":%d", Config.ScrapeTargetPort)
	log.Infof("Exposing Prometheus metrics on %v", listenAndServeOn)
	panic(http.ListenAndServe(listenAndServeOn, nil))
}

func flushMsgBeforeLog(msgBeforeLog []string) {
	for _, msg := range msgBeforeLog {
		if logInitialized {
			log.Info(msg)
		} else {
			fmt.Println(msg)
		}
	}
}
