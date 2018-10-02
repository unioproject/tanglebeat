package main

import (
	"github.com/lunfardo314/giota"
	"sync"
	"time"
)

type AddressBalance struct {
	balance   int64
	isSpent   bool
	refreshed time.Time
}

func (seq *Sequence) NewAddrBalanceChan(index int) (chan *AddressBalance, func()) {
	chOut := make(chan *AddressBalance)
	chCancel := make(chan struct{})
	addr, err := seq.GetAddress(index)
	if err != nil {
		seq.log.Error(err)
	}
	seq.log.Debugf("Created new address balance channel for idx=%v", index)
	isSpent := false
	balance := int64(0)
	var wg sync.WaitGroup
	go func() {
		defer seq.log.Debugf("Closing balance channel for idx=%v", index)
		defer close(chOut)
		wg.Add(1)
		defer wg.Done()

		var balList []int64
		for {
			balList, err = seq.GetBalanceAddr([]giota.Address{addr})
			if err == nil {
				balance = balList[0]
				if !isSpent {
					isSpent, err = seq.IsSpentAddr(addr)
				}
			}
			if err != nil {
				seq.log.Errorf("Error: %v. Sleep 30 sec", err)
				time.Sleep(30 * time.Second)
			} else {
				select {
				case <-chCancel:
					return
				case chOut <- &AddressBalance{balance: balance, isSpent: isSpent, refreshed: time.Now()}:
				case <-time.After(5 * time.Second):
				}
			}
		}
	}()
	return chOut, func() {
		close(chCancel)
		wg.Wait()
	} // returns the channel and cancel closure
}
