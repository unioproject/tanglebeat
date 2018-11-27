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

// creates or reinitializes stopwatch entry.
// Returns false if already exist
func Start(name string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	_, exists := stopwatches[name]
	stopwatches[name] = stopwatchEntry{started: lib.UnixMs(time.Now())}
	return !exists
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

func _get(name string) (uint64, uint64, bool) {
	entry, ok := stopwatches[name]
	if !ok {
		return 0, 0, false
	}
	started := entry.started
	var stopped uint64
	if entry.stopped == 0 {
		stopped = lib.UnixMs(time.Now())
	} else {
		stopped = entry.stopped
	}
	return started, stopped, true
}

func Get(name string) (uint64, uint64, bool) {
	mutex.Lock()
	defer mutex.Unlock()
	return _get(name)
}

func GetAndRemove(name string) (uint64, uint64, bool) {
	mutex.Lock()
	defer mutex.Unlock()
	started, stopped, succ := _get(name)
	if succ {
		delete(stopwatches, name)
	}
	return started, stopped, succ
}
