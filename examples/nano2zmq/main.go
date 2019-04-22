package main

import (
	"flag"
	"fmt"
	"github.com/pebbe/zmq4"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"os"
	"time"
)

const defaultNanoInput = "tcp://tanglebeat.com:5550"
const defaultZmqOutputPort = 5557

func main() {
	pstrInp := flag.String("from", defaultNanoInput, "Nanomsg input")
	pintOutp := flag.Int("to", defaultZmqOutputPort, "ZMQ output port")
	flag.Parse()
	inputUri := *pstrInp
	outputUri := fmt.Sprintf("tcp://*:%v", *pintOutp)

	fmt.Printf("Nanomsg input: %v\n", inputUri)
	fmt.Printf("ZMQ output: %v\n", outputUri)

	subSock := mustOpenInput(inputUri)
	pubSock := mustOpenOutput(outputUri)
	var msg []byte
	var err error
	startInterv := time.Now()
	counter := 0
	for {
		msg, err = subSock.Recv()
		if err != nil {
			fmt.Printf("nanomsg Recv: %v\nSleep 5 sec.", err)
			time.Sleep(5 * time.Second)
		}
		_, err = pubSock.SendBytes(msg, 0)
		if err != nil {
			fmt.Printf("zmq SendBytes: %v\n", err)
		}
		counter++
		if time.Since(startInterv) > 10*time.Second {
			startInterv = time.Now()
			fmt.Printf("%v  %v messages passed\n", startInterv.Format(time.RFC3339), counter)
			counter = 0
		}
	}
}

func mustOpenInput(uri string) mangos.Socket {
	var sock mangos.Socket
	var err error

	if sock, err = sub.NewSocket(); err != nil {
		fmt.Printf("can't create new Nanomsg sub socket: %v\n", err)
		os.Exit(1)
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(uri); err != nil {
		fmt.Printf("can't dial Nanomsg sub socket at %v: %v\n", uri, err)
		os.Exit(1)
	}
	err = sock.SetOption(mangos.OptionSubscribe, "")
	if err != nil {
		fmt.Printf("failed to subscribe to all messages at %v: %v", uri, err)
		os.Exit(1)
	}
	return sock
}

func mustOpenOutput(uri string) *zmq4.Socket {
	sock, err := zmq4.NewSocket(zmq4.PUB)
	if err != nil {
		fmt.Printf("can't create new ZMQ pub socket: %v\n", err)
		os.Exit(1)
	}
	err = sock.Bind(uri)
	if err != nil {
		fmt.Printf("can't bind ZMQ sub socket: %v\n", err)
		os.Exit(1)
	}
	return sock
}
