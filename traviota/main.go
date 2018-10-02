package main

import (
	"os"
	"time"
)

func main() {
	ReadConfig("traviota.yaml")
	log.Infof("Enabled sequences: %v", GetEnabledSeqNames())
	var sequences = make([]*Sequence, 0)
	c := 101
	for _, name := range GetEnabledSeqNames() {
		if seq, err := NewSequence(name); err != nil {
			log.Error(err)
			log.Info("Ciao")
			os.Exit(1)
		} else {
			sequences = append(sequences, seq)
			seq.SaveIndex(c)
			c += 5
			idx := seq.ReadLastIndex()
			log.Infof("%v last idx = %v", seq.Name, idx)
		}

	}
	for _, seq := range sequences {
		go seq.Run()
	}
	time.Sleep(10 * time.Second)
}
