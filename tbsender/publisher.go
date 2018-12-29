package main

import (
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
)

var updatePublisher *nanomsg.Publisher

func mustInitAndRunPublisher() {
	var err error
	updatePublisher, err = nanomsg.NewPublisher(Config.SenderUpdateCollector.OutPort, 0, log)
	if err != nil {
		log.Errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		Config.SenderUpdateCollector.Publish = false
		panic(err)
	}
}
