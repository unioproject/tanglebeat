package main

import (
	"flag"
	"fmt"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"os"
)

const defaultNanoInput = "tcp://tanglebeat.com:5550"
const defaultZmqOutputPort = 5557

func main() {
	pstrInp := flag.String("from", defaultNanoInput, "Nanomsg inpout")
	pintOutp := flag.Int("to", defaultZmqOutputPort, "Zmq output port")
	flag.Parse()
	inputUri := *pstrInp
	outputUri := fmt.Sprintf("tcp://:%v", *pintOutp)
	fmt.Printf("Nanomsg input: %v\n", inputUri)
	fmt.Printf("ZMQ output: %v\n", outputUri)

	var sock mangos.Socket
	var err error

	if sock, err = sub.NewSocket(); err != nil {
		fmt.Printf("can't create new sub socket: %v\n", err)
		os.Exit(1)
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(inputUri); err != nil {
		fmt.Printf("can't dial sub socket at %v: %v\n", inputUri, err)
		os.Exit(1)
	}
	err = sock.SetOption(mangos.OptionSubscribe, "")
	if err != nil {
		fmt.Printf("failed to subscribe to all topics at %v: %v", inputUri, err)
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
