package confirmer

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/comm"
	"github.com/lunfardo314/tanglebeat/lib"
	"net/http"
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
	totalDurationATTMsec  int
	numATT                int
	totalDurationGTTAMsec int
	numGTTA               int
	tps                   float32
}

type ConfirmerUpdate struct {
	UpdateTime            time.Time
	NumAttach             int
	NumPromote            int
	TotalDurationATTMsec  int
	NumATT                int
	TotalDurationGTTAMsec int
	NumGTTA               int
	UpdateType            comm.UpdateType
	Err                   error
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

func (conf *Confirmer) Run(bundle giota.Bundle) chan *ConfirmerUpdate {
	conf.createAPIs()
	nowis := time.Now()
	conf.lastBundle = bundle
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextPromoTime = nowis.Add(time.Duration(conf.PromoteEverySec) * time.Second)
	chanConfirmerUpdate := make(chan *ConfirmerUpdate)
	go func() {
		defer close(chanConfirmerUpdate)
		for {
			incl, err := conf.iotaAPI.GetLatestInclusion(
				[]giota.Trytes{lib.GetTail(conf.lastBundle).Hash()})
			confirmed := err == nil && incl[0]
			if confirmed {
				conf.sendUpdateIfNeeded(chanConfirmerUpdate, comm.UPD_CONFIRM, nil)
				return
			}
			updType, err := conf.doSendingAction()
			conf.sendUpdateIfNeeded(chanConfirmerUpdate, updType, err)
		}
	}()
	return chanConfirmerUpdate
}

func (conf *Confirmer) sendUpdateIfNeeded(ch chan<- *ConfirmerUpdate, updType comm.UpdateType, err error) {
	if updType == comm.UPD_NO_ACTION && err == nil {
		// nothing changes, so nothing to update
		return
	}
	upd := &ConfirmerUpdate{
		UpdateTime:            time.Now(),
		NumAttach:             conf.numAttach,
		NumPromote:            conf.numPromote,
		TotalDurationATTMsec:  conf.totalDurationATTMsec,
		NumATT:                conf.numATT,
		TotalDurationGTTAMsec: conf.totalDurationATTMsec,
		NumGTTA:               conf.numGTTA,
		UpdateType:            updType,
		Err:                   err,
	}
	if updType == comm.UPD_CONFIRM {
		ch <- upd // last update should be never lost
	} else {
		select {
		case ch <- upd:
		case <-time.After(2 * time.Second): // update lost in this case
		}
	}
}

func (conf *Confirmer) doSendingAction() (comm.UpdateType, error) {
	var tail *giota.Transaction
	if len(conf.lastPromoBundle) != 0 {
		// promo already started.
		if time.Now().After(conf.nextPromoTime) {
			if conf.PromoteNoChain {
				// promote blowball
				tail = lib.GetTail(conf.lastBundle)
			} else {
				// promote chain
				tail = lib.GetTail(conf.lastPromoBundle)
			}
			return conf.promoteOrReattach(tail)

		}
	} else {
		return conf.promoteOrReattach(lib.GetTail(conf.lastBundle))
	}
	// Not time for promotion yet. Check if reattachment is needed
	gccResp, err := conf.iotaAPI.CheckConsistency([]giota.Trytes{lib.GetTail(conf.lastBundle).Hash()})
	if err != nil {
		return comm.UPD_NO_ACTION, err
	}
	consistent := gccResp.State
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
