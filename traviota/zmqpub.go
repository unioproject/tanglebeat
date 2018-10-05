package main

import (
	"context"
	"github.com/go-zeromq/zmq4"
)

func ZMQPubChan() (chan []byte, error) {
	chIn := make(chan []byte)
	pub := zmq4.NewPub(context.Background())
	err := pub.Listen("tcp://*:3000")
	if err != nil {
		return nil, err
	}
	go func() {
		defer pub.Close()

		for data := range chIn {
			log.Debugf("---------> Received: %v", string(data))

			msg := zmq4.NewMsg(data)
			err := pub.Send(msg)
			if err != nil {
				log.Errorf("---------> pub.Send error: %v", err)
			} else {
				log.Debugf("---------> Published: %v", string(data))
			}
		}
	}()
	return chIn, err
}
