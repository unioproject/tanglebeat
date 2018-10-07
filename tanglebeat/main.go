package main

import (
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
)

func main() {
	readConfig("tanglebeat.yaml")
	log.Infof("Will be receiving transaction data from '%v'", Config.TraviotaURI)
	log.Infof("Database file: '%v'", Config.DbFile)

	var sock mangos.Socket
	var err error
	var msg []byte

	if sock, err = sub.NewSocket(); err != nil {
		log.Criticalf("can't get new sub socket: %v", err)
	}
	//sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())

	if err = sock.Dial(Config.TraviotaURI); err != nil {
		log.Criticalf("can't dial sub socket: %v", err)
	}
	err = sock.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		log.Criticalf("can't subscribe to all topics: %v", err)
	}
	log.Debugf("Start receiving messages..")
	for {
		if msg, err = sock.Recv(); err != nil {
			log.Errorf("Cannot receive: %v", err)
		}
		log.Debugf("RECEIVED: %v", string(msg))
	}
}
