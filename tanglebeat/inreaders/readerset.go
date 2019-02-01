package inreaders

import (
	"sync"
	"time"
)

type InputReaderSet struct {
	sync.RWMutex
	name   string // for logging
	theSet map[string]InputReader
}

func NewInputReaderSet(name string) *InputReaderSet {
	ret := &InputReaderSet{
		name:   name,
		theSet: make(map[string]InputReader),
	}
	go ret.runStarter()
	return ret
}

func (irs *InputReaderSet) NumRunning() int {
	irs.RLock()
	defer irs.RUnlock()
	var ret int
	for _, r := range irs.theSet {
		r.Lock()
		if r.isRunning__() {
			ret++
		}
		r.Unlock()
	}
	return ret
}

func (irs *InputReaderSet) AddInputReader(name string, ir InputReader) {
	irs.Lock()
	defer irs.Unlock()
	_, ok := irs.theSet[name]
	if !ok {
		ir.SetId__(byte(len(irs.theSet)))
		irs.theSet[name] = ir
		debugf("Routine set '%v': added routine '%v'", irs.name, name)
	}
}

func (irs *InputReaderSet) runStarter() {
	debugf("---- running starter '%v'", irs.name)
	for {
		irs.Lock()
		for n, r := range irs.theSet {
			inputRoutine := r
			name := n
			//----------------
			inputRoutine.Lock()
			if !inputRoutine.isRunning__() && inputRoutine.isTimeToRestart__() {
				inputRoutine.setRunning__()
				go func() {
					stopReason := inputRoutine.Run(name)
					var restartAfter time.Duration
					switch stopReason {
					case REASON_NORUN_ONHOLD_10MIN:
						restartAfter = 10 * time.Minute
					case REASON_NORUN_ONHOLD_15MIN:
						restartAfter = 15 * time.Minute
					case REASON_NORUN_ONHOLD_30MIN:
						restartAfter = 30 * time.Minute
					case REASON_NORUN_ONHOLD_1H:
						restartAfter = 1 * time.Hour
					case REASON_NORUN_ERROR:
						restartAfter = 15 * time.Second
					default:
						restartAfter = 1 * time.Minute
					}
					inputRoutine.Lock()
					inputRoutine.setIdle__(restartAfter, stopReason)
					inputRoutine.Unlock()
					infof("Input routine '%v' will be restarted after %v", name, restartAfter)
				}()
			}
			inputRoutine.Unlock()
			//---------------
		}
		irs.Unlock()
		time.Sleep(10 * time.Second)
	}
}

func (irs *InputReaderSet) ForEach(callback func(name string, ir InputReader)) {
	irs.Lock()
	defer irs.Unlock()
	for name, ir := range irs.theSet {
		callback(name, ir)
	}
}
