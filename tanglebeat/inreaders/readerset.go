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

func (irs *InputReaderSet) AddInputReader(ir InputReader) {
	irs.lock()
	defer irs.unlock()
	name := ir.GetName()
	_, ok := irs.theSet[name]
	if !ok {
		irs.theSet[name] = ir
		debugf("Routine set '%v': added routine '%v'", irs.name, name)
	}
}

func (irs *InputReaderSet) runStarter() {
	debugf("---- Running starter '%v'", irs.name)
	for {
		irs.lock()
		for _, r := range irs.theSet {
			inputRoutine := r
			//----------------
			inputRoutine.Lock()
			if !inputRoutine.isRunning() {
				inputRoutine.setRunning(true)
				go func() {
					inputRoutine.Run()
					inputRoutine.Lock()
					inputRoutine.setRunning(false)
					inputRoutine.Unlock()
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
