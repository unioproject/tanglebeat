package inreaders

import (
	"sync"
	"time"
)

type InputReaderSet struct {
	name   string // for logging
	theSet map[string]InputReader
	mutex  *sync.Mutex
}

func NewInputReaderSet(name string) *InputReaderSet {
	ret := &InputReaderSet{
		name:   name,
		theSet: make(map[string]InputReader),
		mutex:  &sync.Mutex{},
	}
	go ret.runStarter()
	return ret
}

func (irs *InputReaderSet) lock() {
	irs.mutex.Lock()
}

func (irs *InputReaderSet) unlock() {
	irs.mutex.Unlock()
}

func (irs *InputReaderSet) AddInputReader(name string, ir InputReader) {
	irs.lock()
	defer irs.unlock()
	_, ok := irs.theSet[name]
	if !ok {
		ir.SetId__(byte(len(irs.theSet)))
		irs.theSet[name] = ir
		debugf("Routine set '%v': added routine '%v'", irs.name, name)
	}
}

const restartAfter = 1 * time.Minute

func (irs *InputReaderSet) runStarter() {
	debugf("---- running starter '%v'", irs.name)
	for {
		irs.lock()
		for n, r := range irs.theSet {
			inputRoutine := r
			name := n
			//----------------
			inputRoutine.Lock()
			if !inputRoutine.isRunning__() && inputRoutine.isTimeToRestart__() {
				inputRoutine.setRunning__()
				go func() {
					inputRoutine.Run(name)
					inputRoutine.Lock()
					inputRoutine.setIdle__(restartAfter)
					inputRoutine.Unlock()
					infof("Input routine '%v' will be restarted after %v", name, restartAfter)
				}()
			}
			inputRoutine.Unlock()
			//-----------------
		}
		irs.unlock()
		time.Sleep(10 * time.Second)
	}
}

func (irs *InputReaderSet) ForEach(callback func(name string, ir InputReader)) {
	irs.lock()
	defer irs.unlock()
	for name, ir := range irs.theSet {
		callback(name, ir)
	}
}
