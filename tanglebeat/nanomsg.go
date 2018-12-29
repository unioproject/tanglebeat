package main

import (
	"fmt"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/pub"
	"nanomsg.org/go-mangos/transport/tcp"
)

var chanPub = make(chan []byte)

func runPublisherChannel(port int) error {
	var sock mangos.Socket
	var err error

	if sock, err = pub.NewSocket(); err != nil {
		return fmt.Errorf("can't get new sub socket: '%v'", err)
	}
	sock.AddTransport(tcp.NewTransport())
	url := fmt.Sprintf("tcp://:%v", port)
	if err = sock.Listen(url); err != nil {
		return fmt.Errorf("can't listen new pub socket: '%v'", err)
	}
	infof("Nanomsg PUB socket created on %v", url)
	go func() {
		defer sock.Close()
		for data := range chanPub {
			err := sock.Send(data)
			if err != nil {
				errorf("publisher: sock.Send returned '%v'", err)
			}
		}
	}()
	return nil
}

func publishMessage(msg []byte) {
	chanPub <- msg
}
