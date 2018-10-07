package main

import (
	"github.com/lunfardo314/tanglebeat/comm"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"path"
	"sync"
	"time"
)

func runPublisher(wg *sync.WaitGroup) {
	configPublisherLogging()
	err := comm.InitUpdatePublisher(Config.Publisher.OutPort)
	if err != nil {
		logPub.Errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		Config.Publisher.Disabled = true
		return
	}
}

func (seq *Sequence) publishState(state *sendingState, updType comm.UpdateType) {
	upd := comm.SenderUpdate{
		SenderUID:             seq.UID,
		UpdType:               updType,
		Index:                 state.index,
		Addr:                  state.addr,
		SendingStartedTs:      lib.UnixMs(state.sendingStarted),
		NumAttaches:           state.numAttach,
		NumPromotions:         state.numPromote,
		NodeATT:               seq.Params.IOTANodeATT[0],
		NodeGTTA:              seq.Params.IOTANodeGTTA[0],
		PromoteEveryNumSec:    seq.Params.PromoteEverySec,
		ForceReattachAfterSec: seq.Params.ForceReattachAfterMin,
		PromoteNochain:        seq.Params.PromoteNoChain,
	}
	timeSinceStart := time.Since(state.sendingStarted)
	timeSinceStartMsec := int64(timeSinceStart / time.Millisecond)
	upd.SinceSendingMsec = timeSinceStartMsec
	securityLevel := 2
	upd.BundleSize = securityLevel + 2
	upd.PromoBundleSize = 1
	totalTx := upd.BundleSize*upd.NumAttaches + upd.PromoBundleSize*upd.NumPromotions
	if state.numATT != 0 {
		upd.AvgPoWDurationPerTxMsec = int64(state.totalDurationATTMsec / (state.numATT * totalTx))
	}
	if state.numGTTA != 0 {
		upd.AvgGTTADurationMsec = int64(state.totalDurationGTTAMsec / state.numGTTA)
	}
	timeSinceStartSec := float32(timeSinceStartMsec) / float32(1000)
	if timeSinceStartSec > 0.1 {
		upd.TPS = float32(totalTx) / timeSinceStartSec
	} else {
		upd.TPS = 0
	}
	publishUpdate(&upd)
}

func configPublisherLogging() {
	if Config.Publisher.LogConsoleOnly {
		logPub = log
		return
	}
	var err error
	var level logging.Level
	if Config.Debug {
		level = logging.DEBUG
	} else {
		level = logging.INFO
	}
	formatter := logging.MustStringFormatter(Config.Publisher.LogFormat)
	logPub, err = createChildLogger(
		"publisher",
		path.Join(Config.SiteDataDir, Config.Sender.LogDir),
		&masterLoggingBackend,
		&formatter,
		level)
	if err != nil {
		log.Panicf("Can't create publisher log")
	}
}

func publishUpdate(upd *comm.SenderUpdate) error {
	if !Config.Publisher.Disabled {
		if upd.UpdType == comm.UPD_START_SENDING ||
			upd.UpdType == comm.UPD_FINISH_SENDING ||
			upd.UpdType == comm.UPD_CONFIRM {
			logPub.Infof("Published event '%v' for SeqID = %v, index = %v",
				upd.UpdType.String(), upd.SenderUID, upd.Index)
		} else {
			logPub.Debugf("Published event '%v' for SeqID = %v, index = %v",
				upd.UpdType.String(), upd.SenderUID, upd.Index)
		}
		if err := comm.SendUpdate(upd); err != nil {
			return err
		}
	}
	return nil
}
