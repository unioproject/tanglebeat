package main

import (
	"os"
	"sync"
)

func runSender(wg *sync.WaitGroup) {
	for _, name := range getEnabledSeqNames() {
		if seq, err := NewSequence(name); err != nil {
			log.Error(err)
			log.Info("Ciao")
			os.Exit(1)
		} else {
			wg.Add(1)
			go func() {
				seq.Run()
				wg.Done()
			}()
		}
	}
}

func main() {
	masterConfig("traviota.yml")
	var wg sync.WaitGroup
	if !Config.Publisher.Disabled {
		log.Infof("Starting publisher")
		initAndRunPublisher()
	}
	if !Config.Sender.Disabled {
		log.Infof("Starting sender. Enabled sequences: %v", getEnabledSeqNames())
		runSender(&wg)
	}
	wg.Wait()
}
