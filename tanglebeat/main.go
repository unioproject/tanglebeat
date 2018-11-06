package main

import (
	"os"
	"time"
)

func runSender() int {
	var ret int
	for _, name := range getEnabledSeqNames() {
		if seq, err := NewSequence(name); err == nil {
			go seq.Run()
			ret += 1
		} else {
			log.Error(err)
			log.Info("Ciao")
			os.Exit(1)
		}
	}
	return ret
}

func main() {
	masterConfig("tanglebeat.yml")
	var en bool
	initSenderDataCollector()

	if Config.SenderDataCollector.Publish {
		log.Infof("Starting publisher")
		initAndRunPublisher()
		en = true
	}
	if Config.Prometheus.Enabled && (Config.Prometheus.ZmqMetrics.Enabled || Config.Prometheus.SenderMetricsEnabled) {
		log.Infof("Exposing metrics to Prometheus")
		initExposeToPometheus()
		en = true
	}
	if Config.Prometheus.Enabled && Config.Prometheus.ZmqMetrics.Enabled {
		log.Infof("Starting ZMQ metrics updater")
		initMetricsZMQ()
		en = true
	}
	if Config.Sender.Enabled {
		log.Infof("Starting sender. Enabled sequences: %v", getEnabledSeqNames())
		numSeq := runSender()
		en = en || numSeq > 0
	}
	if !en {
		log.Errorf("Nothing is enabled. Leaving...")
		os.Exit(0)
	}

	for {
		time.Sleep(5 * time.Second)
	}
}
