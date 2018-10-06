package main

import (
	"os"
	"sync"
)

func main() {
	ReadConfig("traviota.yaml")
	log.Infof("Enabled sequences: %v", GetEnabledSeqNames())
	initPublisher()

	var sequences = make([]*Sequence, 0)
	for _, name := range GetEnabledSeqNames() {
		if seq, err := NewSequence(name); err != nil {
			log.Error(err)
			log.Info("Ciao")
			os.Exit(1)
		} else {
			sequences = append(sequences, seq)
		}

	}
	var wg sync.WaitGroup
	for _, seq := range sequences {
		wg.Add(1)
		go func() {
			defer wg.Done()
			seq.Run()
		}()
	}
	wg.Wait()
}
