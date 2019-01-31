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
	log       *logging.Logger
	aec       utils.ErrorCounter
	callbacks []func(time.Time)
}

var bundles = make(map[Hash]*bundleState)
var mutexConfmon = &sync.Mutex{}

const loopSleepConfmon = 5 * time.Second

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

func OnConfirmation(bundleHash Hash, mapi multiapi.MultiAPI, log *logging.Logger, aec utils.ErrorCounter, callback func(time.Time)) {
	mutexConfmon.Lock()
	defer mutexConfmon.Unlock()

	_, ok := bundles[bundleHash]
	if !ok {
		bundles[bundleHash] = &bundleState{
			log:       log,
			aec:       aec,
			callbacks: make([]func(time.Time), 0, 2),
		}
	}
	bundles[bundleHash].callbacks = append(bundles[bundleHash].callbacks, callback)
	go pollConfirmed(bundleHash, mapi)
}

func CancelConfirmationPolling(bundleHash Hash) {
	mutexConfmon.Lock()
	defer mutexConfmon.Unlock()
	delete(bundles, bundleHash)
}

func pollConfirmed(bundleHash Hash, mapi multiapi.MultiAPI) {
	var apiret multiapi.MultiCallRet
	var err error
	var confirmed bool
	var bs *bundleState
	var ok bool
	count := 0

	startWaiting := time.Now()
	for {
		mutexConfmon.Lock()
		if bs, ok = bundles[bundleHash]; !ok {
			mutexConfmon.Unlock()
			return // not in map, was cancelled or never started
		}
		mutexConfmon.Unlock()

		count++
		if count%5 == 0 {
			debugf(bs.log, "Confirmation polling for %v. Time since waiting: %v", bundleHash, time.Since(startWaiting))
		}
		confirmed, err = utils.IsBundleHashConfirmedMulti(bundleHash, mapi, &apiret)
		if err == nil && confirmed {
			nowis := time.Now()
			// call all callbacks synchronously
			for _, cb := range bs.callbacks {
				cb(nowis)
			}

			mutexConfmon.Lock()
			delete(bundles, bundleHash) // delete from map
			mutexConfmon.Unlock()

			// stop the stopwatch for the bundle
			StopStopwatch(bundleHash)
			return // confirmed: stop polling
		}
		if checkError(bs.aec, apiret.Endpoint, err) {
			errorf(bs.log, "Confirmation polling for %v: '%v' from %v ", bundleHash, err, apiret.Endpoint)
			time.Sleep(sleepAfterError)
		}
		time.Sleep(loopSleepConfmon)
	}
}
