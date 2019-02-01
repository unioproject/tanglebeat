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
	UPD_FAILED    UpdateType = 4
)

// TODO confirmer task timeout (result 'failed')

type ConfirmerParams struct {
	IotaMultiAPI          multiapi.MultiAPI
	IotaMultiAPIgTTA      multiapi.MultiAPI
	IotaMultiAPIaTT       multiapi.MultiAPI
	TxTagPromote          Trytes
	AddressPromote        Hash
	ForceReattachAfterMin uint64
	PromoteChain          bool
	PromoteEverySec       uint64
	PromoteDisable        bool
	Log                   *logging.Logger
	AEC                   utils.ErrorCounter
	SlowDownThreshold     int
}

type Confirmer struct {
	ConfirmerParams
	mutex     *sync.RWMutex  //task state access sync
	wgTaskEnd sync.WaitGroup // wait until task routines are finished
	confMon   *ConfirmationMonitor

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
	PromoteTailHash       Hash
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
	case UPD_FAILED:
		r = "failed"
	default:
		r = "???"
	}
	return r
}

func NewConfirmer(params ConfirmerParams, confMon *ConfirmationMonitor) *Confirmer {
	if confMon == nil {
		confMon = NewConfirmationMonitor(params.IotaMultiAPI, params.Log, params.AEC)
	}
	return &Confirmer{
		ConfirmerParams: params,
		confMon:         confMon,
		mutex:           &sync.RWMutex{},
	}
}

func (conf *Confirmer) IsConfirming() (bool, Hash) {
	conf.mutex.RLock()
	defer conf.mutex.RUnlock()
	if conf.running {
		return true, conf.bundleHash
	}
	return false, ""
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
	loopSleepPeriodPromoCheck            = 5 * time.Second
	loopSleepPeriodPromote               = 1 * time.Second
	loopSleepPeriodReattach              = 1 * time.Second
	sleepAfterError                      = 5 * time.Second
	defaultSlowDownThresholdNumGoroutine = 300
)

func (conf *Confirmer) getCorrectedSleepLoopPeriod(origSleepLoop time.Duration) time.Duration {
	if runtime.NumGoroutine() > conf.SlowDownThreshold {
		return origSleepLoop * 2
	}
	return origSleepLoop
}

// TODO remove limitation of one confirmer task per confirmer at the time

func (conf *Confirmer) StartConfirmerTask(bundleTrytes []Trytes) (chan *ConfirmerUpdate, func(), error) {
	tail, err := utils.TailFromBundleTrytes(bundleTrytes)
	if err != nil {
		return nil, nil, err
	}
	bundleHash := tail.Bundle
	nowis := time.Now()

	conf.mutex.Lock()
	defer conf.mutex.Unlock()

	if conf.running {
		return nil, nil, errors.New("Confirmer task is already running")
	}
	conf.running = true
	conf.lastBundleTrytes = bundleTrytes
	conf.bundleHash = bundleHash
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextPromoTime = nowis
	conf.nextTailHashToPromote = tail.Hash
	conf.isNotPromotable = false
	conf.chanUpdate = make(chan *ConfirmerUpdate, 1) // not to block each time
	conf.numAttach = 0
	conf.numPromote = 0
	conf.totalDurationGTTAMsec = 0
	conf.totalDurationATTMsec = 0
	if conf.AEC == nil {
		conf.AEC = &utils.DummyAEC{}
	}
	if conf.SlowDownThreshold == 0 {
		conf.SlowDownThreshold = defaultSlowDownThresholdNumGoroutine
	}

	// starting 3 routines
	cancelPromoCheck := conf.goPromotabilityCheck()
	cancelPromo := conf.goPromote()
	cancelReattach := conf.goReattach()

	// confirmation monitor starts yet another routine
	conf.confMon.OnConfirmation(bundleHash, func(nowis time.Time) {
		conf.mutex.RLock()
		defer conf.mutex.RUnlock()

		conf.sendConfirmerUpdate(UPD_CONFIRM, "", nil)
	})

	return conf.chanUpdate, func() {
		conf.stopConfirmerTask(cancelPromoCheck, cancelPromo, cancelReattach)
		conf.confMon.CancelConfirmationPolling(bundleHash)
	}, nil
}

func (conf *Confirmer) stopConfirmerTask(cancelPromoCheck, cancelPromo, cancelReattach func()) {
	conf.mutex.Lock()
	defer conf.mutex.Unlock()
	if !conf.running {
		return
	}
	cancelPromoCheck()
	cancelPromo()
	cancelReattach()
	close(conf.chanUpdate)
	conf.running = false

	conf.wgTaskEnd.Wait()
	conf.Log.Debugf("CONFIRMER: task for %v has ended", conf.bundleHash)
}

func (conf *Confirmer) sendConfirmerUpdate(updType UpdateType, promoTailHash Hash, err error) {
	upd := &ConfirmerUpdate{
		NumAttaches:           conf.numAttach,
		NumPromotions:         conf.numPromote,
		TotalDurationATTMsec:  conf.totalDurationATTMsec,
		TotalDurationGTTAMsec: conf.totalDurationATTMsec,
		UpdateTime:            time.Now(),
		UpdateType:            updType,
		PromoteTailHash:       promoTailHash,
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

		conf.wgTaskEnd.Add(1)
		defer conf.wgTaskEnd.Done()

		var err error
		var consistent bool
		for {
			select {
			case <-chCancel:
				return
			case <-time.After(conf.getCorrectedSleepLoopPeriod(loopSleepPeriodPromoCheck)):
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
	err, tailh := conf.promote()
	if err != nil {
		conf.sendConfirmerUpdate(UPD_NO_ACTION, "", err)
	} else {
		conf.sendConfirmerUpdate(UPD_PROMOTE, tailh, nil)
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

		conf.wgTaskEnd.Add(1)
		defer conf.wgTaskEnd.Done()

		for {
			if err := conf.promoteIfNeeded(); err != nil {
				conf.errorf("CONFIRMER-PROMO: promotion routine: %v. Sleep 5 sec: ", err)
				time.Sleep(sleepAfterError)
			}
			select {
			case <-chCancel:
				return
			case <-time.After(loopSleepPeriodPromote):
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
			conf.sendConfirmerUpdate(UPD_NO_ACTION, "", err)
		} else {
			conf.sendConfirmerUpdate(UPD_REATTACH, "", nil)
		}
	}
	return err
}

func (conf *Confirmer) goReattach() func() {
	chCancel := make(chan struct{})

	go func() {
		conf.debugf("CONFIRMER-REATT: started reattacher routine. Bundle = %v", conf.bundleHash)
		defer conf.debugf("CONFIRMER-REATT: finished reattacher routine. Bundle = %v", conf.bundleHash)

		conf.wgTaskEnd.Add(1)
		defer conf.wgTaskEnd.Done()

		for {
			if err := conf.reattachIfNeeded(); err != nil {
				conf.sendConfirmerUpdate(UPD_NO_ACTION, "", err)
				conf.errorf("reattach function returned: %v. Bundle hash = %v", err, conf.bundleHash)
				time.Sleep(sleepAfterError)
			}
			select {
			case <-chCancel:
				return
			case <-time.After(loopSleepPeriodReattach):
			}
		}
	}()
	return func() {
		close(chCancel)
	}
}
