package main

import (
	"flag"
	"fmt"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"os"
)

// program reads Nanomsg stream for testing

const defaultUri = "tcp://tanglebeat.com:5550"

var topics = []string{"lmi", "lmhs", "seen", "tx"}

func main() {
	pstr := flag.String("uri", defaultUri, "Nanomsg stream URI")
	flag.Parse()
	uri := *pstr
	fmt.Printf("Will be reading from %v\n", uri)
	var sock mangos.Socket
	var err error

	if sock, err = sub.NewSocket(); err != nil {
		fmt.Printf("can't create new sub socket: %v\n", err)
		os.Exit(1)
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(uri); err != nil {
		fmt.Printf("can't dial sub socket at %v: %v\n", uri, err)
		os.Exit(1)
	}
	for _, tpc := range topics {
		err = sock.SetOption(mangos.OptionSubscribe, []byte(tpc))
	}
	if err != nil {
		fmt.Printf("failed to subscribe to all topics at %v: %v", uri, err)
		os.Exit(1)
	}
	var msg []byte
	for {
		msg, err = sock.Recv()
		if err != nil {
			fmt.Printf("recv: %v\n", err)
		} else {
			fmt.Printf("%v\n", string(msg))
		}
	}
}
