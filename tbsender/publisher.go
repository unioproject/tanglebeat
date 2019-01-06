package main

import (
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
)

var updatePublisher *nanomsg.Publisher

func mustInitAndRunPublisher() {
	var err error
	updatePublisher, err = nanomsg.NewPublisher(
		Config.SenderUpdatePublisher.Enabled, Config.SenderUpdatePublisher.OutputPort, 0, log)
	if err != nil {
		log.Errorf("Failed to create publishing channel: %v", err)
		Config.SenderUpdatePublisher.Enabled = false
		panic(err)
	}
	if Config.SenderUpdatePublisher.Enabled {
		log.Infof("Publisher channel created on port %v", Config.SenderUpdatePublisher.OutputPort)
	} else {
		log.Infof("Publisher channel is DISABLED")
	}
}
