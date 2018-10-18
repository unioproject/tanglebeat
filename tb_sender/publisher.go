package main

import (
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/lunfardo314/tanglebeat/metrics"
	"github.com/lunfardo314/tanglebeat/pubsub"
	"github.com/op/go-logging"
	"path"
)

func initAndRunPublisher() {
	configPublisherLogging()
	err := pubsub.RunPublisher(Config.Publisher.OutPort)
	if err != nil {
		logPub.Errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		Config.Publisher.Disabled = true
		return
	}
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

func publishUpdate(upd *pubsub.SenderUpdate) error {
	if !Config.MetricsUpdater.Disabled {
		logPub.Debugf("Update metrics '%v' for %v(%v), index = %v",
			upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index)
		metrics.UpdateMetrics(upd)
	}
	if !Config.Publisher.Disabled {
		logPub.Debugf("Publish '%v' for %v(%v), index = %v",
			upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index)
		if err := pubsub.SendUpdate(upd); err != nil {
			return err
		}
	}
	return nil
}

func confirmerUpdType2Sender(confUpdType confirmer.UpdateType) pubsub.UpdateType {
	switch confUpdType {
	case confirmer.UPD_NO_ACTION:
		return pubsub.UPD_NO_ACTION
	case confirmer.UPD_REATTACH:
		return pubsub.UPD_REATTACH
	case confirmer.UPD_PROMOTE:
		return pubsub.UPD_PROMOTE
	case confirmer.UPD_CONFIRM:
		return pubsub.UPD_CONFIRM
	}
	return pubsub.UPD_UNDEF // can't be
}
