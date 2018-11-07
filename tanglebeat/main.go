package main

import (
	"os"
	"time"
)

const CONFIG_FILE = "tanglebeat.yml"

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
	masterConfig(CONFIG_FILE)
	var enabled bool
	initSenderDataCollector()

	if Config.SenderDataCollector.Publish {
		log.Infof("Starting publisher")
		initAndRunPublisher()
		enabled = true
	}
	if Config.Prometheus.Enabled && (Config.Prometheus.ZmqMetrics.Enabled || Config.Prometheus.SenderMetricsEnabled) {
		log.Infof("Exposing metrics to Prometheus")
		initExposeToPometheus()
		enabled = true
	}
	if Config.Prometheus.Enabled && Config.Prometheus.ZmqMetrics.Enabled {
		log.Infof("Starting ZMQ metrics updater")
		initMetricsZMQ()
		enabled = true
	}
	if Config.Sender.Enabled {
		log.Infof("Starting sender. Enabled sequences: %v", getEnabledSeqNames())
		numSeq := runSender()
		enabled = enabled || numSeq > 0
	}
	if !enabled {
		log.Errorf("Nothing is enabled. Leaving...")
		os.Exit(0)
	}

	for {
		time.Sleep(5 * time.Second)
	}
}
