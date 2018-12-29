package inreaders

import (
	"sync"
	"time"
)

type InputReaderSet struct {
	theSet map[string]InputReader
	mutex  *sync.Mutex
}

func NewInputReaderSet() *InputReaderSet {
	ret := &InputReaderSet{
		theSet: make(map[string]InputReader),
		mutex:  &sync.Mutex{},
	}
	go ret.runStarter()
	return ret
}

func (irs *InputReaderSet) AddInputReader(ir InputReader) {
	irs.mutex.Lock()
	defer irs.mutex.Unlock()
	_, ok := irs.theSet[ir.GetName()]
	if !ok {
		irs.theSet[ir.GetName()] = ir
	}
}

func (irs *InputReaderSet) runStarter() {
	for {
		irs.mutex.Lock()
		for _, inputRoutine := range irs.theSet {
			running, _, _ := inputRoutine.GetState()
			if !running {
				inputRoutine.setRunning(true)
				go func() {
					inputRoutine.Run()
					inputRoutine.setRunning(false)
				}()
			}
		}
		irs.mutex.Unlock()
		time.Sleep(10 * time.Second)
	}
}

func (irs *InputReaderSet) ForEach(callback func(name string, ir InputReader)) {
	irs.mutex.Lock()
	defer irs.mutex.Unlock()
	for name, ir := range irs.theSet {
		callback(name, ir)
	}
}
