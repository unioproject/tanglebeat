package main

import (
	"sync"
	"time"
)

func (seq *Sequence) StartSending(index int) func() {
	chCancel := make(chan struct{})
	var wg sync.WaitGroup
	seq.log.Debugf("Starting sending routine for idx=%v", index)
	go func() {
		defer seq.log.Debugf("Finished sending routine for idx=%v", index)
		wg.Add(1)
		defer wg.Done()
		seq.DoSending(index)
	}()
	return func() {
		close(chCancel)
		wg.Wait()
	}
}

func (seq *Sequence) DoSending(index int) {
	time.Sleep(10 * time.Second)
}
