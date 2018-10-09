package confirmer

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/pubsub"
	"github.com/op/go-logging"
	"net/http"
	"strings"
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
	lastBundle            giota.Bundle
	lastAttachmentTime    time.Time
	nextForceReattachTime time.Time
	numAttach             int
	lastPromoTime         time.Time
	nextPromoTime         time.Time
	lastPromoBundle       giota.Bundle
	numPromote            int
	//
	totalDurationATTMsec  int64
	numATT                int
	totalDurationGTTAMsec int64
	numGTTA               int
	tps                   float32
	//
	chanUpdate chan *ConfirmerUpdate
	log        *logging.Logger
}

type ConfirmerUpdate struct {
	Stats      pubsub.SendingStats
	UpdateTime time.Time
	UpdateType UpdateType
	Err        error
}

func (conf *Confirmer) createIotaAPIs(log *logging.Logger) {
	conf.iotaAPI = giota.NewAPI(
		conf.IOTANode,
		&http.Client{
			Timeout: time.Duration(conf.TimeoutAPI) * time.Second,
		},
	)
	if log != nil {
		log.Debugf("CONFIRMER: IOTA node: %v, Timeout: %v sec", conf.IOTANode, conf.TimeoutAPI)
	}
	conf.iotaAPIgTTA = giota.NewAPI(
		conf.IOTANodeGTTA,
		&http.Client{
			Timeout: time.Duration(conf.TimeoutGTTA) * time.Second,
		},
	)
	if log != nil {
		log.Debugf("CONFIRMER: IOTA node for gTTA: %v, Timeout: %v sec", conf.IOTANodeGTTA, conf.TimeoutGTTA)
	}
	conf.iotaAPIaTT = giota.NewAPI(
		conf.IOTANodeATT,
		&http.Client{
			Timeout: time.Duration(conf.TimeoutATT) * time.Second,
		},
	)
	if log != nil {
		log.Debugf("CONFIRMER: IOTA node for ATT: %v, Timeout: %v sec", conf.IOTANodeATT, conf.TimeoutATT)
	}
}

func (conf *Confirmer) Run(bundle giota.Bundle, log *logging.Logger) chan *ConfirmerUpdate {
	conf.log = log
	conf.createIotaAPIs(log)
	nowis := time.Now()
	conf.lastBundle = bundle
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextPromoTime = nowis.Add(time.Duration(conf.PromoteEverySec) * time.Second)
	conf.chanUpdate = make(chan *ConfirmerUpdate)
	go func() {
		defer close(conf.chanUpdate)
		if conf.log != nil {
			defer conf.log.Debugf("CONFIRMER: confirmer routine ended")
		}
		for {
			incl, err := conf.iotaAPI.GetLatestInclusion(
				[]giota.Trytes{lib.GetTail(conf.lastBundle).Hash()})
			confirmed := err == nil && incl[0]
			if confirmed {
				conf.sendConfirmerUpdate(UPD_CONFIRM, nil)
				return
			}
			updType, err := conf.doSendingAction()
			if updType != UPD_NO_ACTION || err != nil {
				conf.sendConfirmerUpdate(updType, err)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return conf.chanUpdate
}

func (conf *Confirmer) sendConfirmerUpdate(updType UpdateType, err error) {
	upd := &ConfirmerUpdate{
		Stats: pubsub.SendingStats{
			NumAttaches:           conf.numAttach,
			NumPromotions:         conf.numPromote,
			TotalDurationATTMsec:  conf.totalDurationATTMsec,
			NumATT:                conf.numATT,
			TotalDurationGTTAMsec: conf.totalDurationATTMsec,
			NumGTTA:               conf.numGTTA,
		},
		UpdateTime: time.Now(),
		UpdateType: updType,
		Err:        err,
	}
	conf.chanUpdate <- upd
}

func (conf *Confirmer) doSendingAction() (UpdateType, error) {
	var tail *giota.Transaction
	if len(conf.lastPromoBundle) != 0 {
		// promo already started.
		if time.Now().After(conf.nextPromoTime) {
			if conf.PromoteChain {
				// promote chain
				if conf.log != nil {
					conf.log.Debugf("CONFIRMER: promoting 'chain'")
				}
				tail = lib.GetTail(conf.lastPromoBundle)
			} else {
				// promote blowball
				if conf.log != nil {
					conf.log.Debugf("CONFIRMER: promoting 'blowball'")
				}
				tail = lib.GetTail(conf.lastBundle)
			}
			return conf.promoteOrReattach(tail)
		}
	} else {
		return conf.promoteOrReattach(lib.GetTail(conf.lastBundle))
	}
	// Not time for promotion yet. Check if reattachment is needed
	consistent, err := conf.checkConsistency(lib.GetTail(conf.lastBundle).Hash())
	if err != nil {
		return UPD_NO_ACTION, err
	}
	if !consistent || time.Now().After(conf.nextForceReattachTime) {
		err := conf.reattach()
		if err != nil {
			return UPD_NO_ACTION, err
		} else {
			return UPD_REATTACH, err
		}
	}
	// no action
	return UPD_NO_ACTION, nil
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
