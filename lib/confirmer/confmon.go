package confirmer

import (
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/lib/multiapi"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/op/go-logging"
	"sync"
	"time"
)

type bundleState struct {
	confirmed bool
	when      time.Time
	wg        *sync.WaitGroup
	log       *logging.Logger
	aec       utils.ErrorCounter
}

var bundles = make(map[Hash]bundleState)
var mutexConfmon = &sync.Mutex{}

func errorf(log *logging.Logger, format string, args ...interface{}) {
	if log != nil {
		log.Errorf(format, args...)
	}
}

func debugf(log *logging.Logger, format string, args ...interface{}) {
	if log != nil {
		log.Debugf(format, args...)
	}
}

func checkError(aec utils.ErrorCounter, endoint string, err error) bool {
	if aec != nil {
		return aec.CheckError(endoint, err)
	}
	return err != nil
}

func init() {
	go cleanup()
}

func WaitfForConfirmation(bundleHash Hash, mapi multiapi.MultiAPI, log *logging.Logger, aec utils.ErrorCounter) {
	getWG(bundleHash, mapi, log, aec).Wait()
}

func getWG(bundleHash Hash, mapi multiapi.MultiAPI, log *logging.Logger, aec utils.ErrorCounter) *sync.WaitGroup {
	mutexConfmon.Lock()
	defer mutexConfmon.Unlock()

	bs, ok := bundles[bundleHash]
	var wg *sync.WaitGroup
	if !ok {
		wg = &sync.WaitGroup{}
		bundles[bundleHash] = bundleState{
			wg:  wg,
			log: log,
			aec: aec,
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
	if !ok {
		panic("internal inconsistency")
	}
	log := bs.log
	mutexConfmon.Unlock()

	var apiret multiapi.MultiCallRet
	var err error
	var confirmed bool
	wg := bs.wg
	count := 0
	startWaiting := time.Now()
	for {
		count++
		if count%5 == 0 {
			debugf(log, "Confirmation polling for %v. Time since waiting: %v", bundleHash, time.Since(startWaiting))
		}
		confirmed, err = utils.IsBundleHashConfirmedMulti(bundleHash, mapi, &apiret)
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
		if checkError(bs.aec, apiret.Endpoint, err) {
			errorf(log, "Confirmation polling for %v: '%v' from %v ", bundleHash, err, apiret.Endpoint)
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
	log := bs.log
	if bs.confirmed && time.Since(bs.when) > 1*time.Minute {
		delete(bundles, bundleHash)
		debugf(log, "Confirmation polling: removed %v", bundleHash)
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
