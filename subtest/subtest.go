package main

import (
	"context"
	"fmt"
	"github.com/go-zeromq/zmq4"
)

var zmqPort = 3000

func main() {
	socket := zmq4.NewSub(context.Background())
	err := socket.Dial(fmt.Sprintf("tcp://localhost:%v", zmqPort))
	if err != nil {
		fmt.Println(err)
		return
	}
	err = socket.SetOption(zmq4.OptionSubscribe, "")
	if err != nil {
		fmt.Println(err)
		return
	}
	for {
		msg, err := socket.Recv()
		if err == nil {
			fmt.Printf("%v\n", string(msg.Frames[0]))
		} else {
			fmt.Println(err)
		}
	}
}
