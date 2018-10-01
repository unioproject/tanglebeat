package main

import (
	"os"
	"time"
)

func main() {
	ReadConfig("traviota.yaml")
	log.Infof("Enabled sequences: %v", GetEnabledSeqNames())
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
	for _, seq := range sequences {
		go seq.Run()
	}
	time.Sleep(60 * time.Second)
}
