package confirmer

import (
	"errors"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"net/http"
	"strings"
	"sync"
	"time"
)
// TODO get rid dependency from lib
type UpdateType int

const (
	UPD_NO_ACTION UpdateType = 0
	UPD_REATTACH  UpdateType = 1
	UPD_PROMOTE   UpdateType = 2
	UPD_CONFIRM   UpdateType = 3
)

type Confirmer struct {
	IOTANode              string
	IOTANodeGTTA          string
	IOTANodeATT           string
	TimeoutAPI            int
	TimeoutGTTA           int
	TimeoutATT            int
	TxTagPromote          giota.Trytes
	ForceReattachAfterMin int
	PromoteChain          bool
	PromoteEverySec       int
	// internal
	iotaAPI               *giota.API
	iotaAPIgTTA           *giota.API
	iotaAPIaTT            *giota.API
	log                   *logging.Logger
	chanUpdate            chan *ConfirmerUpdate
	mutex                 sync.Mutex         //task state access sync
	// confirmer task state
	lastBundle            giota.Bundle
	nextForceReattachTime time.Time
	numAttach             int
	nextPromoTime         time.Time
	nextBundleToPromote   giota.Bundle
	numPromote            int
	totalDurationATTMsec  int64
	totalDurationGTTAMsec int64
	isNotPromotable       bool
}

type ConfirmerUpdate struct {
	NumAttaches           int
	NumPromotions         int
	TotalDurationATTMsec  int64
	TotalDurationGTTAMsec int64
	UpdateTime time.Time
	UpdateType UpdateType
	Err        error
}

// TODO
func (conf *Confirmer) debugf(f string, p ...interface{}){
	if conf.log != nil {
		conf.log.Debugf(f, p...)
	}
}

func (conf *Confirmer) createIotaAPIs() {
	conf.iotaAPI = giota.NewAPI(
		conf.IOTANode,
		&http.Client{
			Timeout: time.Duration(conf.TimeoutAPI) * time.Second,
		},
	)
	if conf.log != nil {
		conf.log.Debugf("CONFIRMER: IOTA node: %v, Timeout: %v sec", conf.IOTANode, conf.TimeoutAPI)
	}
	conf.iotaAPIgTTA = giota.NewAPI(
		conf.IOTANodeGTTA,
		&http.Client{
			Timeout: time.Duration(conf.TimeoutGTTA) * time.Second,
		},
	)
	if conf.log != nil {
		conf.log.Debugf("CONFIRMER: IOTA node for gTTA: %v, Timeout: %v sec", conf.IOTANodeGTTA, conf.TimeoutGTTA)
	}
	conf.iotaAPIaTT = giota.NewAPI(
		conf.IOTANodeATT,
		&http.Client{
			Timeout: time.Duration(conf.TimeoutATT) * time.Second,
		},
	)
	if conf.log != nil {
		conf.log.Debugf("CONFIRMER: IOTA node for ATT: %v, Timeout: %v sec", conf.IOTANodeATT, conf.TimeoutATT)
	}
}
// TODO new or init confirmer method 

func (conf *Confirmer) Run(bundle giota.Bundle, log *logging.Logger) (chan *ConfirmerUpdate, error) {
	// TODO check validity of the bundle
	if len(bundle) == 0 {
		return nil, errors.New("attempt to run confirmer with empty bundle")
	}
	conf.log = log
	conf.createIotaAPIs()
	nowis := time.Now()
	conf.lastBundle = bundle
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextPromoTime = nowis
	conf.nextBundleToPromote = bundle
	conf.isNotPromotable = false
	conf.chanUpdate = make(chan *ConfirmerUpdate)

	go func() {
		defer close(conf.chanUpdate)
		if conf.log != nil {
			defer conf.log.Debugf("CONFIRMER: confirmer routine ended")
		}
		cancelPromo := conf.runPromote()
		cancelReattach := conf.runReattach()

		for {
			conf.mutex.Lock()
			tail := lib.GetTail(conf.lastBundle)
			conf.mutex.Unlock()

			if tail == nil {
				if log != nil {
					conf.log.Criticalf("can't get tail")
					return
				}
			}
			incl, err := conf.iotaAPI.GetLatestInclusion(
				[]giota.Trytes{tail.Hash()})
			confirmed := err == nil && incl[0]

			if confirmed {
				cancelPromo()
				cancelReattach()
				conf.sendConfirmerUpdate(UPD_CONFIRM, nil)
				return
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return conf.chanUpdate, nil
}

func (conf *Confirmer) sendConfirmerUpdate(updType UpdateType, err error) {
	conf.mutex.Lock()
	upd := &ConfirmerUpdate{
		NumAttaches:           conf.numAttach,
		NumPromotions:         conf.numPromote,
		TotalDurationATTMsec:  conf.totalDurationATTMsec,
		TotalDurationGTTAMsec: conf.totalDurationATTMsec,
		UpdateTime: time.Now(),
		UpdateType: updType,
		Err:        err,
	}
	conf.mutex.Unlock()
	conf.chanUpdate <- upd
}

func (conf *Confirmer) checkConsistency(tailHash giota.Trytes) (bool, error) {
	ccResp, err := conf.iotaAPI.CheckConsistency([]giota.Trytes{tailHash})
	if err != nil {
		return false, err
	}
	consistent := ccResp.State
	if !consistent && strings.Contains(ccResp.Info, "not solid") {
		consistent = true
	}
	if !consistent {
		if conf.log != nil {
			conf.log.Debugf("CONFIRMER: inconsistent tail. Reason: %v", ccResp.Info)
		}
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

func (conf *Confirmer) runPromote() func() {
	chCancel := make(chan struct{})
	var wg sync.WaitGroup
	go func() {
		if conf.log != nil {
			conf.log.Debug("Started promoter routine")
			defer conf.log.Debug("Ended promoter routine")
		}
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
			if err != nil && conf.log != nil {
				conf.log.Errorf("promotion routine: %v", err)
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

func (conf *Confirmer) runReattach() func() {
	chCancel := make(chan struct{})
	var wg sync.WaitGroup
	go func() {
		if conf.log != nil {
			conf.log.Debug("Started reattacher routine")
			defer conf.log.Debug("Ended reattacher routine")
		}
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
			if err != nil && conf.log != nil {
				conf.log.Errorf("promotion routine: %v", err)
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
