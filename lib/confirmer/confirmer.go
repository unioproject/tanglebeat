package confirmer

import (
	"errors"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/lib/multiapi"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/op/go-logging"
	"runtime"
	"strings"
	"sync"
	"time"
)

type UpdateType int

const (
	UPD_NO_ACTION UpdateType = 0
	UPD_REATTACH  UpdateType = 1
	UPD_PROMOTE   UpdateType = 2
	UPD_CONFIRM   UpdateType = 3
)

type Confirmer struct {
	IotaMultiAPI     multiapi.MultiAPI
	IotaMultiAPIgTTA multiapi.MultiAPI
	IotaMultiAPIaTT  multiapi.MultiAPI

	TxTagPromote          Trytes
	AddressPromote        Hash
	ForceReattachAfterMin uint64
	PromoteChain          bool
	PromoteEverySec       uint64
	PromoteDisable        bool
	Log                   *logging.Logger
	AEC                   utils.ErrorCounter
	SlowDownThreshold     int
	// internal
	mutex *sync.Mutex    //task state access sync
	wg    sync.WaitGroup // wait until both promote and reattach are finished
	// confirmer task state
	running               bool
	chanUpdate            chan *ConfirmerUpdate
	lastBundleTrytes      []Trytes
	bundleHash            Hash
	nextForceReattachTime time.Time
	numAttach             uint64
	nextPromoTime         time.Time
	nextTailHashToPromote Hash
	numPromote            uint64
	totalDurationATTMsec  uint64
	totalDurationGTTAMsec uint64
	isNotPromotable       bool
}

type ConfirmerUpdate struct {
	NumAttaches           uint64
	NumPromotions         uint64
	TotalDurationATTMsec  uint64
	TotalDurationGTTAMsec uint64
	UpdateTime            time.Time
	UpdateType            UpdateType
	Err                   error
}

func (ut UpdateType) ToString() string {
	var r string
	switch ut {
	case UPD_NO_ACTION:
		r = "no action"
	case UPD_REATTACH:
		r = "reattach"
	case UPD_PROMOTE:
		r = "promote"
	case UPD_CONFIRM:
		r = "confirm"
	default:
		r = "???"
	}
	return r
}

func (conf *Confirmer) debugf(f string, p ...interface{}) {
	if conf.Log != nil {
		conf.Log.Debugf(f, p...)
	}
}

func (conf *Confirmer) errorf(f string, p ...interface{}) {
	if conf.Log != nil {
		conf.Log.Errorf(f, p...)
	}
}

func (conf *Confirmer) warningf(f string, p ...interface{}) {
	if conf.Log != nil {
		conf.Log.Warningf(f, p...)
	}
}

const (
	loopSleepPeriod                     = 5 * time.Second
	sleepAfterError                     = 5 * time.Second
	defaultSlowDownThesholdNumGoroutine = 100
)

func (conf *Confirmer) getSleepLoopPeriod() time.Duration {
	sleepPeriod := loopSleepPeriod
	if runtime.NumGoroutine() > conf.SlowDownThreshold {
		sleepPeriod = sleepPeriod * 2
	}
	return sleepPeriod
}

func (conf *Confirmer) StartConfirmerTask(bundleTrytes []Trytes) (chan *ConfirmerUpdate, error) {
	// not very nice
	if conf.mutex == nil {
		conf.mutex = &sync.Mutex{}
	}

	tail, err := utils.TailFromBundleTrytes(bundleTrytes)
	if err != nil {
		return nil, err
	}
	bundleHash := tail.Bundle
	nowis := time.Now()

	conf.mutex.Lock()
	defer conf.mutex.Unlock()

	if conf.running {
		return nil, errors.New("Confirmer task is already running")
	}
	conf.running = true
	conf.lastBundleTrytes = bundleTrytes
	conf.bundleHash = bundleHash
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextPromoTime = nowis
	conf.nextTailHashToPromote = tail.Hash
	conf.isNotPromotable = false
	conf.chanUpdate = make(chan *ConfirmerUpdate)
	conf.numAttach = 0
	conf.numPromote = 0
	conf.totalDurationGTTAMsec = 0
	conf.totalDurationATTMsec = 0
	if conf.AEC == nil {
		conf.AEC = &utils.DummyAEC{}
	}
	if conf.SlowDownThreshold == 0 {
		conf.SlowDownThreshold = defaultSlowDownThesholdNumGoroutine
	}

	// starting 4 routines
	cancelPromoCheck := conf.goPromotabilityCheck()
	cancelPromo := conf.goPromote()
	cancelReattach := conf.goReattach()

	go conf.waitForConfirmation(cancelPromoCheck, cancelPromo, cancelReattach)

	return conf.chanUpdate, nil
}

// will wait confirmation of the bundle and cancel other routines when confirmed
func (conf *Confirmer) waitForConfirmation(cancelPromoCheck, cancelPromo, cancelReattach func()) {
	conf.debugf("CONFIRMER-WAIT: 'wait confirmation' routine started for %v", conf.bundleHash)
	defer conf.debugf("CONFIRMER-WAIT: 'wait confirmation' routine ended for %v", conf.bundleHash)
	bundleHash := conf.bundleHash

	WaitfForConfirmation(bundleHash, conf.IotaMultiAPI, conf.Log, conf.AEC)

	conf.Log.Debugf("CONFIRMER-WAIT: confirmed bundle %v", bundleHash)

	conf.mutex.Lock()
	conf.sendConfirmerUpdate(UPD_CONFIRM, nil)
	conf.running = false
	conf.mutex.Unlock()

	conf.Log.Debugf("CONFIRMER-WAIT: canceling confirmer task for bundle %v", bundleHash)
	cancelPromoCheck()
	cancelPromo()
	cancelReattach()
	conf.wg.Wait()

	conf.Log.Debugf("CONFIRMER-WAIT: stopped promoter and reattacher routines for bundle %v", bundleHash)

	close(conf.chanUpdate) // stop update channel
	conf.Log.Debugf("CONFIRMER-WAIT: closed update channel for bundle %v", bundleHash)
	return //>>>>>>>>>>>>>>
}

//func (conf *Confirmer) waitForConfirmationOld(cancelPromoCheck, cancelPromo, cancelReattach func()) {
//	started := time.Now()
//	conf.debugf("CONFIRMER-WAIT: 'wait confirmation' routine started for %v", conf.bundleHash)
//	defer conf.debugf("CONFIRMER-WAIT: 'wait confirmation' routine ended for %v", conf.bundleHash)
//
//	var apiret multiapi.MultiCallRet
//	bundleHash := conf.bundleHash
//	for count := 0; ; count++ {
//		if count%3 == 0 {
//			conf.debugf("CONFIRMER-WAIT: confirm task for bundle hash %v running already %v", bundleHash, time.Since(started))
//		}
//		confirmed, err := lib.IsBundleHashConfirmedMulti(bundleHash, conf.IotaMultiAPI, &apiret)
//		if conf.AEC.CheckError(apiret.Endpoint, err) {
//			conf.errorf("CONFIRMER-WAIT: isBundleHashConfirmed returned %v", err)
//		} else {
//			if confirmed {
//				// stop the stopwatch for the bundle
//				stopwatch.Stop(bundleHash)
//
//				conf.Log.Debugf("CONFIRMER-WAIT: confirmed bundle %v", bundleHash)
//
//				conf.mutex.Lock()
//				conf.sendConfirmerUpdate(UPD_CONFIRM, nil)
//				conf.running = false
//				conf.mutex.Unlock()
//
//				conf.Log.Debugf("CONFIRMER-WAIT: canceling confirmer task for bundle %v", bundleHash)
//				cancelPromoCheck()
//				cancelPromo()
//				cancelReattach()
//				conf.wg.Wait()
//
//				conf.Log.Debugf("CONFIRMER-WAIT: stopped promoter and reattacher routines for bundle %v", bundleHash)
//
//				close(conf.chanUpdate) // stop update channel
//				conf.Log.Debugf("CONFIRMER-WAIT: closed update channel for bundle %v", bundleHash)
//				return //>>>>>>>>>>>>>>
//			}
//		}
//		time.Sleep(conf.getSleepLoopPeriod())
//	}
//}

func (conf *Confirmer) sendConfirmerUpdate(updType UpdateType, err error) {
	upd := &ConfirmerUpdate{
		NumAttaches:           conf.numAttach,
		NumPromotions:         conf.numPromote,
		TotalDurationATTMsec:  conf.totalDurationATTMsec,
		TotalDurationGTTAMsec: conf.totalDurationATTMsec,
		UpdateTime:            time.Now(),
		UpdateType:            updType,
		Err:                   err,
	}
	conf.chanUpdate <- upd
}

func (conf *Confirmer) checkConsistency(tailHash Hash) (bool, error) {
	var apiret multiapi.MultiCallRet

	consistent, info, err := conf.IotaMultiAPI.CheckConsistency(tailHash, &apiret)
	if conf.AEC.CheckError(apiret.Endpoint, err) {
		return false, err
	}
	if !consistent && strings.Contains(info, "not solid") {
		consistent = true
	}
	if !consistent {
		conf.debugf("CONFIRMER: inconsistent tail. Reason: %v", info)
	}
	return consistent, nil
}

func (conf *Confirmer) goPromotabilityCheck() func() {
	chCancel := make(chan struct{})
	go func() {
		conf.debugf("CONFIRMER-PROMOCHECK: started promotability checker routine for bundle hash %v", conf.bundleHash)
		defer conf.debugf("CONFIRMER-PROMOCHECK: finished promotability checker routine for bundle hash %v", conf.bundleHash)

		conf.wg.Add(1)
		defer conf.wg.Done()
		var err error
		var consistent bool
		for {
			select {
			case <-chCancel:
				return
			case <-time.After(conf.getSleepLoopPeriod()):
				consistent, err = conf.checkConsistency(conf.nextTailHashToPromote)
				if err != nil {
					conf.Log.Errorf("CONFIRMER-PROMOCHECK: checkConsistency returned: %v", err)
					time.Sleep(sleepAfterError)
				} else {
					conf.mutex.Lock()
					conf.isNotPromotable = !consistent
					conf.mutex.Unlock()
				}
			}
		}
	}()
	return func() {
		close(chCancel)
	}
}

func (conf *Confirmer) promoteIfNeeded() error {
	conf.mutex.Lock()
	defer conf.mutex.Unlock()

	if conf.isNotPromotable || time.Now().Before(conf.nextPromoTime) {
		// if not promotable, routine will be idle until reattached
		return nil
	}
	err := conf.promote()
	if err != nil {
		conf.sendConfirmerUpdate(UPD_NO_ACTION, err)
	} else {
		conf.sendConfirmerUpdate(UPD_PROMOTE, nil)
	}
	return err
}

func (conf *Confirmer) goPromote() func() {
	if conf.PromoteDisable {
		conf.debugf("CONFIRMER-PROMO: promotion is disabled, promo routine won't be started")
		return func() {} // routine is not started, empty cancel function is returned
	}

	chCancel := make(chan struct{})
	go func() {
		conf.debugf("CONFIRMER-PROMO: started promoter routine  for bundle hash %v", conf.bundleHash)
		defer conf.debugf("CONFIRMER-PROMO: finished promoter routine for bundle hash %v", conf.bundleHash)

		conf.wg.Add(1)
		defer conf.wg.Done()

		for {
			if err := conf.promoteIfNeeded(); err != nil {
				conf.errorf("CONFIRMER-PROMO: promotion routine: %v. Sleep 5 sec: ", err)
				time.Sleep(sleepAfterError)
			}
			select {
			case <-chCancel:
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
	}()
	return func() {
		close(chCancel)
	}
}

func (conf *Confirmer) reattachIfNeeded() error {
	conf.mutex.Lock()
	defer conf.mutex.Unlock()

	var err error
	if conf.isNotPromotable || time.Now().After(conf.nextForceReattachTime) {
		err = conf.reattach()
		if err != nil {
			conf.sendConfirmerUpdate(UPD_NO_ACTION, err)
		} else {
			conf.sendConfirmerUpdate(UPD_REATTACH, nil)
		}
	}
	return err
}

func (conf *Confirmer) goReattach() func() {
	chCancel := make(chan struct{})

	go func() {
		conf.debugf("CONFIRMER-REATT: started reattacher routine. Bundle = %v", conf.bundleHash)
		defer conf.debugf("CONFIRMER-REATT: finished reattacher routine. Bundle = %v", conf.bundleHash)

		conf.wg.Add(1)
		defer conf.wg.Done()

		for {
			if err := conf.reattachIfNeeded(); err != nil {
				conf.sendConfirmerUpdate(UPD_NO_ACTION, err)
				conf.errorf("reattach function returned: %v. Bundle hash = %v", err, conf.bundleHash)
				time.Sleep(sleepAfterError)
			}
			select {
			case <-chCancel:
				return
			case <-time.After(100 * time.Millisecond):
			}
		}
	}()
	return func() {
		close(chCancel)
	}
}