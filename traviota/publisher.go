package main

import (
	"github.com/lunfardo314/tanglebeat/comm"
	"github.com/op/go-logging"
	"path"
	"sync"
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
		if upd.UpdType == comm.UPD_CONFIRM {
			logPub.Infof("Published event '%v' for %v(%v), index = %v",
				upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index)
		} else {
			logPub.Debugf("Published event '%v' for %v(%v), index = %v",
				upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index)
		}
		if err := comm.SendUpdate(upd); err != nil {
			return err
		}
	}
	return nil
}
