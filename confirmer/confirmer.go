package confirmer

import (
	"errors"
	"fmt"
	"github.com/lunfardo314/giota"
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
	IotaAPI               *giota.API
	IotaAPIgTTA           *giota.API
	IotaAPIaTT            *giota.API
	TxTagPromote          giota.Trytes
	ForceReattachAfterMin int
	PromoteChain          bool
	PromoteEverySec       int64
	Log                   *logging.Logger
	// internal
	chanUpdate chan *ConfirmerUpdate
	mutex      sync.Mutex //task state access sync
	// confirmer task state
	lastBundle            giota.Bundle
	nextForceReattachTime time.Time
	numAttach             int64
	nextPromoTime         time.Time
	nextBundleToPromote   giota.Bundle
	numPromote            int64
	totalDurationATTMsec  int64
	totalDurationGTTAMsec int64
	isNotPromotable       bool
}

type ConfirmerUpdate struct {
	NumAttaches           int64
	NumPromotions         int64
	TotalDurationATTMsec  int64
	TotalDurationGTTAMsec int64
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

func (conf *Confirmer) StartConfirmerTask(bundle giota.Bundle) (chan *ConfirmerUpdate, func(), error) {
	if err := lib.CheckBundle(bundle); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Attempt to run confirmer with wrong bundle: %v", err))
	}
	nowis := time.Now()
	conf.lastBundle = bundle
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextPromoTime = nowis
	conf.nextBundleToPromote = bundle
	conf.isNotPromotable = false
	conf.chanUpdate = make(chan *ConfirmerUpdate)
	conf.numAttach = 0
	conf.numPromote = 0
	conf.totalDurationGTTAMsec = 0
	conf.totalDurationATTMsec = 0

	cancelPromo := conf.goPromote()
	cancelReattach := conf.goReattach()

	h := bundle.Hash()

	return conf.chanUpdate, func() {
		conf.Log.Debugf("CONFIRMER: canceling confirmer task for %v", h)
		cancelPromo()
		cancelReattach()
		close(conf.chanUpdate)
	}, nil

}

func (conf *Confirmer) RunConfirm(bundle giota.Bundle) (chan *ConfirmerUpdate, error) {
	chUpd, cancelFun, err := conf.StartConfirmerTask(bundle)
	if err != nil {
		return nil, err
	}

	bundleHash := bundle.Hash()
	go func() {
		defer conf.debugf("CONFIRMER: confirmer task ended")
		defer cancelFun()

		for {
			if confirmed, err := conf.isBundleHashConfirmed(bundleHash); err != nil {
				conf.errorf("CONFIRMER:isBundleHashConfirmed: %v", err)
				time.Sleep(5 * time.Second)
			} else {
				if confirmed {
					conf.sendConfirmerUpdate(UPD_CONFIRM, nil)
					return
				}
			}
			time.Sleep(1 * time.Second)
		}
	}()
	return chUpd, nil
}

func (conf *Confirmer) isBundleHashConfirmed(bundleHash giota.Trytes) (bool, error) {
	for {
		time.Sleep(2 * time.Second)

		ftResp, err := conf.IotaAPI.FindTransactions(&giota.FindTransactionsRequest{
			Bundles: []giota.Trytes{bundleHash},
		})
		if err != nil {
			return false, err
		}

		states, err := conf.IotaAPI.GetLatestInclusion(ftResp.Hashes)
		if err != nil {
			return false, err
		}
		for _, conf := range states {
			if conf {
				return true, nil
			}
		}
	}
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

func (conf *Confirmer) checkConsistency(tailHash giota.Trytes) (bool, error) {
	ccResp, err := conf.IotaAPI.CheckConsistency([]giota.Trytes{tailHash})
	if err != nil {
		return false, err
	}
	consistent := ccResp.State
	if !consistent && strings.Contains(ccResp.Info, "not solid") {
		consistent = true
	}
	if !consistent {
		conf.debugf("CONFIRMER: inconsistent tail. Reason: %v", ccResp.Info)
	}
	return consistent, nil
}

func (conf *Confirmer) checkIfToPromote() (bool, error) {
	conf.mutex.Lock()
	defer conf.mutex.Unlock()

	if conf.isNotPromotable || time.Now().Before(conf.nextPromoTime) {
		// if not promotable, routine will be idle until reattached
		return false, nil
	}
	tail := lib.GetTail(conf.nextBundleToPromote)
	if tail != nil {
		txh := tail.Hash()
		consistent, err := conf.checkConsistency(txh)
		if err != nil {
			return false, err
		}
		conf.isNotPromotable = !consistent
		return consistent, nil
	}
	return false, errors.New("can't get tail")
}

func (conf *Confirmer) goPromote() func() {
	chCancel := make(chan struct{})
	h := conf.lastBundle.Hash()

	var wg sync.WaitGroup
	go func() {
		conf.debugf("CONFIRMER: started promoter routine for bundle hash%v", h)
		defer conf.debugf("CONFIRMER: ended promoter routine for bundle hash %v", h)

		wg.Add(1)
		defer wg.Done()

		var err error
		var toPromote bool
		for {
			toPromote, err = conf.checkIfToPromote()
			if err == nil && toPromote {

				conf.mutex.Lock()
				err = conf.promote()
				conf.mutex.Unlock()

				if err != nil {
					conf.sendConfirmerUpdate(UPD_NO_ACTION, err)
				} else {
					conf.sendConfirmerUpdate(UPD_PROMOTE, nil)
				}
			}
			if err != nil {
				conf.errorf("CONFIRMER: promotion routine: %v", err)
			}
			if err != nil {
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

func (conf *Confirmer) goReattach() func() {
	chCancel := make(chan struct{})
	h := conf.lastBundle.Hash()

	var wg sync.WaitGroup
	go func() {
		conf.debugf("CONFIRMER: started reattacher routine for bundle hash %v", h)
		defer conf.debugf("CONFIRMER: ended reattacher routine bundle hash %v", h)

		wg.Add(1)
		defer wg.Done()

		var err error
		var sendUpdate bool
		for {
			conf.mutex.Lock()
			if conf.isNotPromotable || time.Now().After(conf.nextForceReattachTime) {
				err = conf.reattach()
				sendUpdate = true
			}
			conf.mutex.Unlock()

			if sendUpdate {
				if err != nil {
					conf.sendConfirmerUpdate(UPD_NO_ACTION, err)
				} else {
					conf.sendConfirmerUpdate(UPD_REATTACH, nil)
				}
			}
			if err != nil {
				conf.errorf("promotion routine: %v", err)
			}
			if err != nil {
				time.Sleep(5 * time.Second)
			}
			sendUpdate = false
			err = nil
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
