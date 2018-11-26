package stopwatch

import (
	"github.com/lunfardo314/tanglebeat/lib"
	"sync"
	"time"
)

// stopwatchEntry with a given name must be started once
// first stop (earliest) will have effect of caling callback
// subsequent stops won't have any effect

type stopwatchEntry struct {
	started uint64
	stopped uint64
}

var stopwatches = make(map[string]stopwatchEntry)
var mutex sync.Mutex

func Start(name string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	_, ok := stopwatches[name]
	if ok {
		return false
	}
	stopwatches[name] = stopwatchEntry{started: lib.UnixMs(time.Now())}
	return true
}

func Stop(name string) bool {
	mutex.Lock()
	defer mutex.Unlock()

	entry, ok := stopwatches[name]
	if !ok {
		return false
	}
	if entry.stopped == 0 {
		entry.stopped = lib.UnixMs(time.Now())
	}
	return true
}

func Get(name string) (uint64, uint64, bool) {
	mutex.Lock()
	defer mutex.Unlock()

	entry, ok := stopwatches[name]
	if !ok {
		return 0, 0, false
	}
	started := entry.started
	stopped := entry.stopped
	delete(stopwatches, name)
	return started, stopped, true

}
