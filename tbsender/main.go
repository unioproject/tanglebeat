package main

import (
	"os"
	"time"
)

const CONFIG_FILE = "tbsender.yml"

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
	mustReadMasterConfig(CONFIG_FILE)

	var enabled bool

	mustInitAndRunPublisher()
	if Config.SenderUpdatePublisher.Enabled {
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
