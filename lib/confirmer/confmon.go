package confirmer

import (
	"github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/op/go-logging"
	"github.com/unioproject/tanglebeat/lib/multiapi"
	"github.com/unioproject/tanglebeat/lib/utils"
	"sync"
	"time"
)

type bundleState struct {
	callbacks []func(time.Time)
}

type addrState struct {
	callbacks []func(time.Time)
}

type ConfirmationMonitor struct {
	sync.Mutex
	bundles   map[Hash]*bundleState
	addresses map[Hash]*addrState
	mapi      multiapi.MultiAPI
	log       *logging.Logger
	aec       utils.ErrorCounter
}

func NewConfirmationMonitor(mapi multiapi.MultiAPI, log *logging.Logger, aec utils.ErrorCounter) *ConfirmationMonitor {
	return &ConfirmationMonitor{
		bundles:   make(map[Hash]*bundleState),
		addresses: make(map[Hash]*addrState),
		mapi:      mapi,
		log:       log,
		aec:       aec,
	}
}

const loopSleepConfmon = 10 * time.Second

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

// can't be called from within OnConfirmation callback
func (cmon *ConfirmationMonitor) CancelConfirmationPolling(bundleHash Hash) {
	cmon.Lock()
	defer cmon.Unlock()
	delete(cmon.bundles, bundleHash)
}

func (cmon *ConfirmationMonitor) pollConfirmed(bundleHash Hash) {
	count := 0
	var exit bool

	startWaiting := time.Now()
	for !exit {
		time.Sleep(loopSleepConfmon)
		count++
		if count%5 == 0 {
			cmon.debugf("Confirmation polling for %v. Time since waiting: %v", bundleHash, time.Since(startWaiting))
		}
		exit = cmon.checkBundle(bundleHash)
	}
}

func (cmon *ConfirmationMonitor) checkBundle(bundleHash Hash) bool {
	var apiret multiapi.MultiCallRet
	var err error
	var confirmed bool
	var bs *bundleState
	var ok bool

	cmon.Lock()
	defer cmon.Unlock()

	if bs, ok = cmon.bundles[bundleHash]; !ok {
		return true // not in map, was cancelled or never started
	}

	confirmed, err = utils.IsBundleHashConfirmedMulti(bundleHash, cmon.mapi, &apiret)

	if cmon.checkError(apiret.Endpoint, err) {
		cmon.errorf("Confirmation polling for %v: '%v' from %v ", bundleHash, err, apiret.Endpoint)
		time.Sleep(sleepAfterError)
		return false
	}
	if confirmed {
		nowis := time.Now()

		// call all callbacks asynchronously
		for _, cb := range bs.callbacks {
			go cb(nowis)
		}
		delete(cmon.bundles, bundleHash) // delete from map

		// stop the stopwatch for the bundle
		StopStopwatch(bundleHash)
		return true // confirmed: stop polling
	}
	return false
}

func (cmon *ConfirmationMonitor) OnBalanceZero(addr Hash, callback func(time.Time)) {
	cmon.Lock()
	defer cmon.Unlock()

	_, ok := cmon.addresses[addr]
	if !ok {
		cmon.addresses[addr] = &addrState{
			callbacks: make([]func(time.Time), 0, 2),
		}
		go cmon.pollZeroBalance(addr)
	}
	cmon.addresses[addr].callbacks = append(cmon.bundles[addr].callbacks, callback)
}

func (cmon *ConfirmationMonitor) CancelZeroBalancePolling(addr Hash) {
	cmon.Lock()
	defer cmon.Unlock()
	delete(cmon.addresses, addr)
}

// TODO zero balance
func (cmon *ConfirmationMonitor) pollZeroBalance(addr Hash) {
	var apiret multiapi.MultiCallRet
	var err error
	var as *addrState
	var ok bool
	var bal *api.Balances

	cmon.Lock()
	mapi := cmon.mapi
	cmon.Unlock()

	for {
		cmon.Lock()
		if as, ok = cmon.addresses[addr]; !ok {
			cmon.Unlock()
			return // not in map, was cancelled or never started
		}
		cmon.Unlock()

		bal, err = mapi.GetBalances(Hashes{addr}, 100, &apiret)
		if err == nil && bal.Balances[0] == 0 {
			nowis := time.Now()

			// call all callbacks asynchronously
			cmon.Lock()
			for _, cb := range as.callbacks {
				go cb(nowis)
			}
			delete(cmon.addresses, addr) // delete from map
			cmon.Unlock()
		}
		if cmon.checkError(apiret.Endpoint, err) {
			cmon.errorf("Zero balance polling for %v: '%v' from %v ", addr, err, apiret.Endpoint)
			time.Sleep(sleepAfterError)
		}
		time.Sleep(loopSleepConfmon)
	}
}
