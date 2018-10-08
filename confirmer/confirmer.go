package confirmer

import (
	"github.com/lunfardo314/giota"
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
	NumErrors             int
	TotalDurationATTMsec  int
	NumATT                int
	TotalDurationGTTAMsec int
	NumGTTA               int
}

func (conf *Confirmer) Run(bundle giota.Bundle) chan *ConfirmerUpdate {
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
	nowis := time.Now()
	conf.lastBundle = bundle
	conf.nextForceReattachTime = nowis.Add(time.Duration(conf.ForceReattachAfterMin) * time.Minute)
	conf.nextPromoTime = nowis.Add(time.Duration(conf.PromoteEverySec) * time.Second)
	ret := make(chan *ConfirmerUpdate)
	go func() {
		defer close(ret)
		for {
			incl, err := conf.iotaAPI.GetLatestInclusion([]giota.Trytes{conf.lastBundle[0].Hash()})
			confirmed := err == nil && incl[0]
			if confirmed {
				return // TODO must be update about it
			}

		}
	}()
	return ret
}
