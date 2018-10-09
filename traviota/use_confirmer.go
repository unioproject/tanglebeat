package main

import (
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/lunfardo314/tanglebeat/pubsub"
	"github.com/op/go-logging"
	"time"
)

func (seq *Sequence) NewConfirmerChan(bundle giota.Bundle, log *logging.Logger) chan *confirmer.ConfirmerUpdate {
	ret := confirmer.Confirmer{
		IOTANode:              seq.Params.IOTANode[0],
		IOTANodeGTTA:          seq.Params.IOTANodeGTTA[0],
		IOTANodeATT:           seq.Params.IOTANodeATT[0],
		TimeoutAPI:            seq.Params.TimeoutAPI,
		TimeoutGTTA:           seq.Params.TimeoutGTTA,
		TimeoutATT:            seq.Params.TimeoutATT,
		TxTagPromote:          seq.TxTagPromote,
		ForceReattachAfterMin: seq.Params.ForceReattachAfterMin,
		PromoteChain:          seq.Params.PromoteChain,
		PromoteEverySec:       seq.Params.PromoteEverySec,
	}
	return ret.Run(bundle, log)
}

func (seq *Sequence) confirmerUpdateToPub(updConf *confirmer.ConfirmerUpdate, addr giota.Address, index int, sendingStarted time.Time) {
	upd := pubsub.SenderUpdate{
		SeqUID:                seq.UID,
		SeqName:               seq.Name,
		UpdType:               confirmerUpdType2Sender(updConf.UpdateType),
		Index:                 index,
		Addr:                  addr,
		SendingStartedTs:      lib.UnixMs(sendingStarted),
		NumAttaches:           updConf.Stats.NumAttaches,
		NumPromotions:         updConf.Stats.NumPromotions,
		NodeATT:               seq.Params.IOTANodeATT[0],
		NodeGTTA:              seq.Params.IOTANodeGTTA[0],
		PromoteEveryNumSec:    seq.Params.PromoteEverySec,
		ForceReattachAfterSec: seq.Params.ForceReattachAfterMin,
		PromoteChain:          seq.Params.PromoteChain,
		TotalPoWMsec:          updConf.Stats.TotalDurationATTMsec,
		TotalTipselMsec:       updConf.Stats.TotalDurationGTTAMsec,
	}
	upd.UpdateTs = lib.UnixMs(time.Now())
	securityLevel := 2
	upd.BundleSize = securityLevel + 2
	upd.PromoBundleSize = 1
	publishUpdate(&upd)
}

func (seq *Sequence) initSendUpdateToPub(addr giota.Address, index int, sendingStarted time.Time, initStats *pubsub.SendingStats) {
	upd := pubsub.SenderUpdate{
		SeqUID:                seq.UID,
		SeqName:               seq.Name,
		UpdType:               pubsub.UPD_SEND,
		Index:                 index,
		Addr:                  addr,
		SendingStartedTs:      lib.UnixMs(sendingStarted),
		NumAttaches:           initStats.NumAttaches,
		NumPromotions:         initStats.NumPromotions,
		NodeATT:               seq.Params.IOTANodeATT[0],
		NodeGTTA:              seq.Params.IOTANodeGTTA[0],
		PromoteEveryNumSec:    seq.Params.PromoteEverySec,
		ForceReattachAfterSec: seq.Params.ForceReattachAfterMin,
		PromoteChain:          seq.Params.PromoteChain,
		TotalPoWMsec:          initStats.TotalDurationATTMsec,
		TotalTipselMsec:       initStats.TotalDurationGTTAMsec,
	}
	upd.UpdateTs = lib.UnixMs(time.Now())
	securityLevel := 2
	upd.BundleSize = securityLevel + 2
	upd.PromoBundleSize = 1
	publishUpdate(&upd)
}
