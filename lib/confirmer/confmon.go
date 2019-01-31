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
	callbacks []func(time.Time)
}

type ConfirmationMonitor struct {
	sync.Mutex
	bundles map[Hash]*bundleState
	mapi    multiapi.MultiAPI
	log     *logging.Logger
	aec     utils.ErrorCounter
}

func NewConfirmationMonitor(mapi multiapi.MultiAPI, log *logging.Logger, aec utils.ErrorCounter) *ConfirmationMonitor {
	return &ConfirmationMonitor{
		bundles: make(map[Hash]*bundleState),
		mapi:    mapi,
		log:     log,
		aec:     aec,
	}
}

const loopSleepConfmon = 5 * time.Second

func (cmon *ConfirmationMonitor) errorf(format string, args ...interface{}) {
	if cmon.log != nil {
		cmon.log.Errorf(format, args...)
	}
}

func (cmon *ConfirmationMonitor) debugf(format string, args ...interface{}) {
	if cmon.log != nil {
		cmon.log.Debugf(format, args...)
	}
}

func (cmon *ConfirmationMonitor) checkError(endpoint string, err error) bool {
	if cmon.aec != nil {
		return cmon.aec.CheckError(endpoint, err)
	}
	return err != nil
}

// TODO aec and log will be used as set by the first call. This not completely correct
func (cmon *ConfirmationMonitor) OnConfirmation(bundleHash Hash, callback func(time.Time)) {
	cmon.Lock()
	defer cmon.Unlock()

	_, ok := cmon.bundles[bundleHash]
	if !ok {
		cmon.bundles[bundleHash] = &bundleState{
			callbacks: make([]func(time.Time), 0, 2),
		}
		go cmon.pollConfirmed(bundleHash)
	}
	cmon.bundles[bundleHash].callbacks = append(cmon.bundles[bundleHash].callbacks, callback)
}

func (cmon *ConfirmationMonitor) CancelConfirmationPolling(bundleHash Hash) {
	cmon.Lock()
	defer cmon.Unlock()
	delete(cmon.bundles, bundleHash)
}

func (cmon *ConfirmationMonitor) pollConfirmed(bundleHash Hash) {
	var apiret multiapi.MultiCallRet
	var err error
	var confirmed bool
	var bs *bundleState
	var ok bool
	count := 0

	cmon.Lock()
	mapi := cmon.mapi
	cmon.Unlock()

	startWaiting := time.Now()
	for {
		cmon.Lock()
		if bs, ok = cmon.bundles[bundleHash]; !ok {
			cmon.Unlock()
			return // not in map, was cancelled or never started
		}
		cmon.Unlock()

		count++
		if count%5 == 0 {
			cmon.debugf("Confirmation polling for %v. Time since waiting: %v", bundleHash, time.Since(startWaiting))
		}
		confirmed, err = utils.IsBundleHashConfirmedMulti(bundleHash, mapi, &apiret)
		if err == nil && confirmed {
			nowis := time.Now()
			// call all callbacks synchronously
			for _, cb := range bs.callbacks {
				cb(nowis)
			}

			cmon.Lock()
			delete(cmon.bundles, bundleHash) // delete from map
			cmon.Unlock()

			// stop the stopwatch for the bundle
			StopStopwatch(bundleHash)
			return // confirmed: stop polling
		}
		if cmon.checkError(apiret.Endpoint, err) {
			cmon.errorf("Confirmation polling for %v: '%v' from %v ", bundleHash, err, apiret.Endpoint)
			time.Sleep(sleepAfterError)
		}
		time.Sleep(loopSleepConfmon)
	}
}
