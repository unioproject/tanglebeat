package main

import (
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
)

var updatePublisher *nanomsg.Publisher

func mustInitAndRunPublisher() {
	var err error
	updatePublisher, err = nanomsg.NewPublisher(Config.SenderUpdatePublisher.OutputPort, 0, log)
	if err != nil {
		log.Errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		Config.SenderUpdatePublisher.Enabled = false
		panic(err)
	}
}
