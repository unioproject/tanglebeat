package main

import (
	"github.com/lunfardo314/tanglebeat/lib/nanomsg"
)

var compoundOutPublisher *nanomsg.Publisher

func mustInitAndRunPublishers() {
	var err error
	compoundOutPublisher, err = nanomsg.NewPublisher(Config.NanomsgPort, 0, nil)
	if err != nil {
		errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		panic(err)
	}
	infof("Publisher for zmq compound out stream initialized successfully on port %v", Config.NanomsgPort)
}
