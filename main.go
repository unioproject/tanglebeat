package main

import (
	"os"
	"time"
)

func runSender() {
	for _, name := range getEnabledSeqNames() {
		if seq, err := NewSequence(name); err == nil {
			go seq.Run()
		} else {
			log.Error(err)
			log.Info("Ciao")
			os.Exit(1)
		}
	}
}

func main() {
	masterConfig("tanglebeat.yml")
	if !Config.Publisher.Disabled {
		log.Infof("Starting publisher")
		initAndRunPublisher()
	}
	if !Config.MetricsUpdater.Disabled {
		log.Infof("Starting Prometheus metrics updater")
		initAndRunMetricsUpdater(Config.MetricsUpdater.PrometheusTargetPort)
	}
	if !Config.Sender.Disabled {
		log.Infof("Starting sender. Enabled sequences: %v", getEnabledSeqNames())
		runSender()
	}
	for {
		time.Sleep(5 * time.Second)
	}
}