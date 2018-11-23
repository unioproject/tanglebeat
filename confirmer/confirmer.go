package confirmer

import (
	. "github.com/iotaledger/iota.go/api"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
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
	IotaAPI               *API
	IotaAPIgTTA           *API
	IotaAPIaTT            *API
	TxTagPromote          Trytes
	ForceReattachAfterMin uint64
	PromoteChain          bool
	PromoteEverySec       uint64
	Log                   *logging.Logger
	AEC                   lib.ErrorCounter
	// internal
	mutex sync.Mutex //task state access sync
	// confirmer task state
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

type dummy struct{}

func (*dummy) IncErrorCount(api *API) {}

func (conf *Confirmer) StartConfirmerTask(bundleTrytes []Trytes) (chan *ConfirmerUpdate, func(), error) {
	tail, err := lib.TailFromBundleTrytes(bundleTrytes)
	if err != nil {
		return nil, nil, err
	}
	nowis := time.Now()

	// -----
	conf.mutex.Lock()
	conf.lastBundleTrytes = bundleTrytes
	conf.bundleHash = tail.Bundle
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
		conf.AEC = &dummy{}
	}
	conf.mutex.Unlock()
	// -----

	cancelPromo := conf.goPromote(tail.Bundle)
	cancelReattach := conf.goReattach(tail.Bundle)

	return conf.chanUpdate, func() {
		conf.Log.Debugf("CONFIRMER: canceling confirmer task for bundle %v", tail.Bundle)

		// ----
		conf.mutex.Lock()
		cancelPromo()
		cancelReattach()
		close(conf.chanUpdate)
		conf.invalidateTaskState() // por las dudas
		conf.mutex.Unlock()
		// ----
	}, nil
}

func (conf *Confirmer) invalidateTaskState() {
	conf.chanUpdate = nil
	conf.lastBundleTrytes = nil
	conf.bundleHash = ""
	conf.numAttach = 0
	conf.nextTailHashToPromote = ""
	conf.numPromote = 0
	conf.totalDurationATTMsec = 0
	conf.totalDurationGTTAMsec = 0
	conf.isNotPromotable = false
}

func (conf *Confirmer) RunConfirm(bundleTrytes []Trytes) (chan *ConfirmerUpdate, error) {
	// start promote and reattach routines
	chUpd, cancelFun, err := conf.StartConfirmerTask(bundleTrytes)
	if err != nil {
		return nil, err
	}

	// wait until any bundle with the hash is confirmed
	go func() {
		started := time.Now()
		conf.debugf("CONFIRMER: confirmer task started")
		defer conf.debugf("CONFIRMER: confirmer task ended")
		defer cancelFun()

		for count := 0; ; count++ {
			if count%3 == 0 {
				conf.debugf("----- CONFIRMER: confirm task for bundle hash %v running already %v", conf.bundleHash, time.Since(started))
			}
			confirmed, err := lib.IsBundleHashConfirmed(conf.bundleHash, conf.IotaAPI)
			if err != nil {
				conf.AEC.IncErrorCount(conf.IotaAPI)
				conf.errorf("CONFIRMER:isBundleHashConfirmed: %v", err)
			} else {
				if confirmed {
					conf.sendConfirmerUpdate(UPD_CONFIRM, nil)
					return
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return chUpd, nil
}

func (conf *Confirmer) sendConfirmerUpdate(updType UpdateType, err error) {
	conf.mutex.Lock()
	upd := &ConfirmerUpdate{
		NumAttaches:           conf.numAttach,
		NumPromotions:         conf.numPromote,
		TotalDurationATTMsec:  conf.totalDurationATTMsec,
		TotalDurationGTTAMsec: conf.totalDurationATTMsec,
		UpdateTime:            time.Now(),
		UpdateType:            updType,
		Err:                   err,
	}
	conf.mutex.Unlock()
	conf.chanUpdate <- upd
}

func (conf *Confirmer) checkConsistency(tailHash Hash) (bool, error) {
	consistent, info, err := conf.IotaAPI.CheckConsistency(tailHash)
	if err != nil {
		conf.AEC.IncErrorCount(conf.IotaAPI)
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

func (conf *Confirmer) promoteIfNeeded() (bool, error) {
	conf.mutex.Lock()
	defer conf.mutex.Unlock()

	toPromote, err := conf.checkIfToPromote()
	if err != nil {
		return false, err
	}
	if !toPromote {
		return false, nil
	}
	err = conf.promote()
	return err == nil, err
}

func (conf *Confirmer) checkIfToPromote() (bool, error) {

	if conf.isNotPromotable || time.Now().Before(conf.nextPromoTime) {
		// if not promotable, routine will be idle until reattached
		return false, nil
	}
	// check if next tail to promote is consistent. If not, promote will be idle
	consistent, err := conf.checkConsistency(conf.nextTailHashToPromote)
	if err != nil {
		return false, err
	}
	conf.isNotPromotable = !consistent
	return consistent, nil
}

func (conf *Confirmer) goPromote(bundleHash Hash) func() {
	chCancel := make(chan struct{})

	var wg sync.WaitGroup
	go func() {
		conf.debugf("CONFIRMER: started promoter routine  for bundle hash %v", bundleHash)
		defer conf.debugf("CONFIRMER: finished promoter routine for bundle hash %v", bundleHash)

		wg.Add(1)
		defer wg.Done()

		var err error
		var promoted bool
		for {
			promoted, err = conf.promoteIfNeeded()
			if err != nil {
				conf.sendConfirmerUpdate(UPD_NO_ACTION, err)
			} else {
				if promoted {
					conf.sendConfirmerUpdate(UPD_PROMOTE, nil)
				}
			}
			if err != nil {
				conf.errorf("CONFIRMER: promotion routine: %v. Sleep 5 sec: ", err)
				time.Sleep(5 * time.Second)
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
		wg.Wait()
	}
}

func (conf *Confirmer) reattachIfNeeded() (bool, error) {
	conf.mutex.Lock()
	defer conf.mutex.Unlock()

	if conf.isNotPromotable || time.Now().After(conf.nextForceReattachTime) {
		err := conf.reattach()
		return err == nil, err
	}
	return false, nil
}

func (conf *Confirmer) goReattach(bundleHash Hash) func() {
	chCancel := make(chan struct{})

	var wg sync.WaitGroup
	go func() {
		conf.debugf("CONFIRMER: started reattacher routine. Bundle = %v", bundleHash)
		defer conf.debugf("CONFIRMER: finished reattacher routine. Bundle = %v", bundleHash)

		wg.Add(1)
		defer wg.Done()

		for {
			reattached, err := conf.reattachIfNeeded()
			if err != nil {
				conf.sendConfirmerUpdate(UPD_NO_ACTION, err)
				conf.errorf("reattach function returned: %v bundle hash = %v", err, bundleHash)
				time.Sleep(5 * time.Second)
			} else {
				if reattached {
					conf.sendConfirmerUpdate(UPD_REATTACH, nil)
				}
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
		wg.Wait()
	}
}
