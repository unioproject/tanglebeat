package main

import (
	"github.com/lunfardo314/tanglebeat/confirmer"
	"github.com/op/go-logging"
	"path"
)

func initAndRunPublisher() {
	configPublisherLogging()
	err := RunPublisher(Config.Publisher.OutPort)
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

func publishUpdate(upd *SenderUpdate) error {
	if !Config.MetricsUpdater.Disabled {
		logPub.Debugf("Update metrics '%v' for %v(%v), index = %v",
			upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index)
		updateMetrics(upd)
	}
	if !Config.Publisher.Disabled {
		logPub.Debugf("Publish '%v' for %v(%v), index = %v",
			upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index)
		if err := SendUpdate(upd); err != nil {
			return err
		}
	}
	return nil
}

func confirmerUpdType2Sender(confUpdType confirmer.UpdateType) SenderUpdateType {
	switch confUpdType {
	case confirmer.UPD_NO_ACTION:
		return SENDER_UPD_NO_ACTION
	case confirmer.UPD_REATTACH:
		return SENDER_UPD_REATTACH
	case confirmer.UPD_PROMOTE:
		return SENDER_UPD_PROMOTE
	case confirmer.UPD_CONFIRM:
		return SENDER_UPD_CONFIRM
	}
	return SENDER_UPD_UNDEF // can't be
}
