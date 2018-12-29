package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/pub"
	"nanomsg.org/go-mangos/transport/tcp"
	"time"
)

var chanDataToPub chan []byte

func initAndRunPublisher() {
	err := runPublisher()
	if err != nil {
		log.Errorf("Failed to create publishing channel. Publisher is disabled: %v", err)
		Config.SenderUpdateCollector.Publish = false
		return
	}
}

// reads input stream of byte arrays and sends them to publish channel
func runPublisher() error {
	var sock mangos.Socket
	var err error
	if sock, err = pub.NewSocket(); err != nil {
		return errors.New(fmt.Sprintf("can't get new sub socket: %v", err))
	}

	chanDataToPub = make(chan []byte)
	// sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())
	url := fmt.Sprintf("tcp://:%v", Config.SenderUpdateCollector.OutPort)
	if err = sock.Listen(url); err != nil {
		return errors.New(fmt.Sprintf("can't listen new pub socket: %v", err))
	}
	log.Infof("Publisher: PUB socket listening on %v", url)
	go func() {
		defer sock.Close()
		for data := range chanDataToPub {
			err := sock.Send(data)
			if err != nil {
				log.Error(err)
			}
		}
	}()
	return nil
}

func publishUpdate(upd *sender_update.SenderUpdate) error {
	data, err := json.Marshal(upd)
	if err != nil {
		log.Errorf("Publisher:publishUpdate %v", err)
		return err
	}
	select {
	case chanDataToPub <- data:
	case <-time.After(5 * time.Second):
		log.Error("----- Timeout 5 sec on sending to publish channel")
	}
	return nil
}
