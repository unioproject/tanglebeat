package main

import (
	"flag"
	"fmt"
	"github.com/op/go-logging"
	"github.com/pebbe/zmq4"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"os"
	"time"
)

// program converts Nanomsg/Mangos data stream (SUB) into ZMQ data stream (PUB).
// usage: nano2zmq [-from <input Nanomsg URI> [-to <ZMQ output port>]]

const defaultNanoInput = "tcp://tanglebeat.com:5550"
const defaultZmqOutputPort = 5556

func main() {
	initLogging()

	infof("nano2zmq: converts Nanomsg/Mangos data stream (SUB) into ZMQ data stream (PUB)\n")

	pstrInp := flag.String("from", defaultNanoInput, "Nanomsg input")
	pintOutp := flag.Int("to", defaultZmqOutputPort, "ZMQ output port")
	flag.Parse()
	inputUri := *pstrInp
	outputUri := fmt.Sprintf("tcp://*:%v", *pintOutp)

	infof("Nanomsg input: %v\n", inputUri)
	infof("ZMQ output: %v\n", outputUri)

	subSock := mustOpenInput(inputUri)
	pubSock := mustOpenOutput(outputUri)
	var msg []byte
	var err error
	startInterv := time.Now()
	counter := 0
	for {
		msg, err = subSock.Recv()
		if err != nil {
			errorf("nanomsg Recv: %v\nSleep 5 sec.", err)
			time.Sleep(5 * time.Second)
		}
		_, err = pubSock.SendBytes(msg, 0)
		if err != nil {
			errorf("zmq SendBytes: %v\n", err)
		}
		counter++
		if time.Since(startInterv) > 10*time.Second {
			startInterv = time.Now()
			infof("%v messages passed\n", counter)
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
		criticalf("can't create new ZMQ pub socket: %v\n", err)
		os.Exit(1)
	}
	err = sock.Bind(uri)
	if err != nil {
		criticalf("can't bind ZMQ sub socket: %v\n", err)
		os.Exit(1)
	}
	return sock
}

// logging

const (
	logFormat = "%{time:2006-01-02 15:04:05.000} %{level:.4s} [%{module}] %{message}"
	logLevel  = logging.INFO
)

var log *logging.Logger

func initLogging() {
	log = logging.MustGetLogger("nano2zmq")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	logFormat := logging.MustStringFormatter(logFormat)
	backendFormatter := logging.NewBackendFormatter(backend, logFormat)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(logLevel, "nano2zmq")
	log.SetBackend(backendLeveled)
}

func errorf(format string, args ...interface{}) {
	log.Errorf("ERRO "+format, args...)
}

func infof(format string, args ...interface{}) {
	log.Infof("INFO "+format, args...)
}

func criticalf(format string, args ...interface{}) {
	log.Criticalf("INFO "+format, args...)
}
