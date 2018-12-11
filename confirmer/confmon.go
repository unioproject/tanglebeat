package confirmer

import (
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/multiapi"
	"github.com/op/go-logging"
	"sync"
	"time"
)

type bundleState struct {
	confirmed bool
	when      time.Time
	wg        *sync.WaitGroup
}

var bundles = make(map[Hash]bundleState)
var mutexConfmon sync.Mutex

var logLocal *logging.Logger
var aecLocal lib.ErrorCounter

func SetLogAec(log *logging.Logger, aec lib.ErrorCounter) {
	logLocal = log
	aecLocal = aec
}

func errorf(format string, args ...interface{}) {
	if logLocal != nil {
		logLocal.Errorf(format, args...)
	}
}

func debugf(format string, args ...interface{}) {
	if logLocal != nil {
		logLocal.Debugf(format, args...)
	}
}

func checkError(endoint string, err error) bool {
	if aecLocal != nil {
		return aecLocal.CheckError(endoint, err)
	}
	return err != nil
}

func init() {
	go cleanup()
}

func WaitfForConfirmation(bundleHash Hash, mapi multiapi.MultiAPI) {
	getWG(bundleHash, mapi).Wait()
}

func getWG(bundleHash Hash, mapi multiapi.MultiAPI) *sync.WaitGroup {
	mutexConfmon.Lock()
	defer mutexConfmon.Unlock()

	bs, ok := bundles[bundleHash]
	var wg *sync.WaitGroup
	if !ok {
		wg = &sync.WaitGroup{}
		bundles[bundleHash] = bundleState{
			wg: wg,
		}
		bundles[bundleHash].wg.Add(1)
		go pollConfirmed(bundleHash, mapi)
	} else {
		wg = bs.wg
	}
	return wg
}

func pollConfirmed(bundleHash Hash, mapi multiapi.MultiAPI) {
	mutexConfmon.Lock()
	bs, ok := bundles[bundleHash]
	mutexConfmon.Unlock()

	if !ok {
		panic("internal inconsistency")
	}
	var apiret multiapi.MultiCallRet
	var err error
	var confirmed bool
	wg := bs.wg
	count := 0
	startWaiting := time.Now()
	for {
		count++
		if count%5 == 0 {
			debugf("Confirmation polling for %v. Time since waiting: %v", bundleHash, time.Since(startWaiting))
		}
		confirmed, err = lib.IsBundleHashConfirmedMulti(bundleHash, mapi, &apiret)
		if err == nil && confirmed {
			wg.Done()
			mutexConfmon.Lock()
			bs.confirmed = true
			bs.when = time.Now()
			mutexConfmon.Unlock()
			// stop the stopwatch for the bundle
			StopStopwatch(bundleHash)
			return
		}
		if checkError(apiret.Endpoint, err) {
			errorf("Confirmation polling for %v: '%v' from %v ", bundleHash, err, apiret.Endpoint)
			time.Sleep(5 * time.Second)
		}
		time.Sleep(5 * time.Second)
	}
}

// any confirmed hash record older that 1 min will be removed
func removeIfObsolete(bundleHash Hash) {
	mutexConfmon.Lock()
	defer mutexConfmon.Unlock()

	bs, ok := bundles[bundleHash]
	if !ok {
		return
	}
	if bs.confirmed && time.Since(bs.when) > 1*time.Minute {
		delete(bundles, bundleHash)
		debugf("Confirmation polling: removed %v", bundleHash)
	}
}

func getHashes() []Hash {
	ret := make([]Hash, 0, len(bundles))
	mutexConfmon.Lock()
	defer mutexConfmon.Unlock()
	for k := range bundles {
		ret = append(ret, k)
	}
	return ret
}

func cleanup() {
	for {
		time.Sleep(1 * time.Minute)
		keys := getHashes()
		for _, k := range keys {
			removeIfObsolete(k)
		}
	}
}
