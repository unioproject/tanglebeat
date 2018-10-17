package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/pubsub"
	"time"
)

func (seq *Sequence) confirmerUpdateToPub(updConf *confirmer.ConfirmerUpdate,
	addr giota.Address, index int, bundleHash giota.Trytes, sendingStarted time.Time) {
	upd := pubsub.SenderUpdate{
		SeqUID:                seq.params.GetUID(),
		SeqName:               seq.name,
		UpdType:               confirmerUpdType2Sender(updConf.UpdateType),
		Index:                 index,
		Addr:                  addr,
		Bundle:                bundleHash,
		SendingStartedTs:      lib.UnixMs(sendingStarted),
		NumAttaches:           updConf.NumAttaches,
		NumPromotions:         updConf.NumPromotions,
		NodeATT:               seq.params.IOTANodeATT[0],
		NodeGTTA:              seq.params.IOTANodeGTTA[0],
		PromoteEveryNumSec:    seq.params.PromoteEverySec,
		ForceReattachAfterMin: seq.params.ForceReattachAfterMin,
		PromoteChain:          seq.params.PromoteChain,
		TotalPoWMsec:          updConf.TotalDurationATTMsec,
		TotalTipselMsec:       updConf.TotalDurationGTTAMsec,
	}
	upd.UpdateTs = lib.UnixMs(updConf.UpdateTime)
	securityLevel := 2
	upd.BundleSize = securityLevel + 1
	upd.PromoBundleSize = 1
	publishUpdate(&upd)
}
