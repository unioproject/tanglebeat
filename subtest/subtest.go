package main

import (
	"fmt"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"os"
)

var zmqPort = 3000

func main() {
	var sock mangos.Socket
	var err error
	var msg []byte

	if sock, err = sub.NewSocket(); err != nil {
		fmt.Printf("can't get new sub socket: %v\n", err.Error())
		os.Exit(1)
	}
	//sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())

	uri := fmt.Sprintf("tcp://localhost:%v", zmqPort)
	fmt.Println(uri)
	if err = sock.Dial(uri); err != nil {
		fmt.Printf("can't dial sub socket: %v\n", err.Error())
		os.Exit(1)
	}
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	err = sock.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	for {
		if msg, err = sock.Recv(); err != nil {
			fmt.Printf("Cannot receive: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("CLIENT: RECEIVED %s\n", string(msg))
	}
}

//func main() {
//	socket := zmq4.NewSub(context.Background(), zmq4.WithID(zmq4.SocketIdentity("sub0")))
//	//socket := zmq4.NewSub(context.Background())
//	uri := fmt.Sprintf("tcp://localhost:%v", zmqPort)
//	fmt.Println(uri)
//	err := socket.Dial(uri)
//	if err != nil {
//		fmt.Println(err)
//		return
//	}
//	err = socket.SetOption(zmq4.OptionSubscribe, "")
//	if err != nil {
//		fmt.Println(err)
//		return
//	}
//	for {
//		msg, err := socket.Recv()
//		if err == nil {
//			fmt.Printf("%v\n", string(msg.Frames[0]))
//		} else {
//			fmt.Println(err)
//		}
//	}
//}
