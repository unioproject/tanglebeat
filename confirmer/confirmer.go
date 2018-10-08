package confirmer

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/comm"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"net/http"
	"strings"
	"time"
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
	PromoteNoChain        bool
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
	Stats      comm.SendingStats
	UpdateTime time.Time
	UpdateType comm.UpdateType
	Err        error
}

func (conf *Confirmer) createAPIs() {
	conf.iotaAPI = giota.NewAPI(
		conf.IOTANode,
		&http.Client{
			Timeout: time.Duration(conf.TimeoutAPI) * time.Second,
		},
	)
	//ret.log.Infof("IOTA node: %v, Timeout: %v sec", ret.Params.IOTANode[0], ret.Params.TimeoutAPI)
	conf.iotaAPIgTTA = giota.NewAPI(
		conf.IOTANodeGTTA,
		&http.Client{
			Timeout: time.Duration(conf.TimeoutGTTA) * time.Second,
		},
	)
	// ret.log.Infof("IOTA node for gTTA: %v, Timeout: %v sec", ret.Params.IOTANodeGTTA[0], ret.Params.TimeoutGTTA)
	conf.iotaAPIaTT = giota.NewAPI(
		conf.IOTANodeATT,
		&http.Client{
			Timeout: time.Duration(conf.TimeoutATT) * time.Second,
		},
	)
}

func (conf *Confirmer) Run(bundle giota.Bundle, log *logging.Logger) chan *ConfirmerUpdate {
	conf.log = log
	conf.createAPIs()
	nowis := time.Now()
	conf.lastBundle = bundle
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextPromoTime = nowis.Add(time.Duration(conf.PromoteEverySec) * time.Second)
	conf.chanUpdate = make(chan *ConfirmerUpdate)
	go func() {
		defer close(conf.chanUpdate)
		if conf.log != nil {
			defer conf.log.Debugf("CONFIRMER: Leaving confirmer routine")
		}
		for {
			incl, err := conf.iotaAPI.GetLatestInclusion(
				[]giota.Trytes{lib.GetTail(conf.lastBundle).Hash()})
			confirmed := err == nil && incl[0]
			if confirmed {
				conf.sendConfirmerUpdate(comm.UPD_CONFIRM, nil)
				return
			}
			updType, err := conf.doSendingAction()
			if updType != comm.UPD_NO_ACTION || err != nil {
				conf.sendConfirmerUpdate(updType, err)
			}
			time.Sleep(5 * time.Second)
		}
	}()
	return conf.chanUpdate
}

func (conf *Confirmer) sendConfirmerUpdate(updType comm.UpdateType, err error) {
	upd := &ConfirmerUpdate{
		Stats: comm.SendingStats{
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

func (conf *Confirmer) doSendingAction() (comm.UpdateType, error) {
	var tail *giota.Transaction
	if len(conf.lastPromoBundle) != 0 {
		// promo already started.
		if time.Now().After(conf.nextPromoTime) {
			if conf.PromoteNoChain {
				// promote blowball
				if conf.log != nil {
					conf.log.Debugf("CONFIRMER: promoting 'blowball'")
				}
				tail = lib.GetTail(conf.lastBundle)
			} else {
				// promote chain
				if conf.log != nil {
					conf.log.Debugf("CONFIRMER: promoting 'chain'")
				}
				tail = lib.GetTail(conf.lastPromoBundle)
			}
			return conf.promoteOrReattach(tail)
		}
	} else {
		return conf.promoteOrReattach(lib.GetTail(conf.lastBundle))
	}
	// Not time for promotion yet. Check if reattachment is needed
	consistent, err := conf.checkConsistency(lib.GetTail(conf.lastBundle).Hash())
	if err != nil {
		return comm.UPD_NO_ACTION, err
	}
	if !consistent || time.Now().After(conf.nextForceReattachTime) {
		err := conf.reattach()
		if err != nil {
			return comm.UPD_NO_ACTION, err
		} else {
			return comm.UPD_REATTACH, err
		}
	}
	// no action
	return comm.UPD_NO_ACTION, nil
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
