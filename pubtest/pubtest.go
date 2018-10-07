package main

import (
	"context"
	"fmt"
	"github.com/go-zeromq/zmq4"
	"time"
)

func main() {
	// ctx, _ := context.WithTimeout(context.Background(), 100*time.Millisecond)
	pub := zmq4.NewPub(context.Background())
	uri := "tcp://*:3000"
	fmt.Println(uri)
	err := pub.Listen(uri)
	for i := 0; ; i++ {
		s := fmt.Sprintf("%d", i)
		msg := zmq4.NewMsg([]byte(s))
		fmt.Printf("Sending: %v\n", s)
		err = pub.Send(msg)
		if err != nil {
			fmt.Printf("PUB after Send: Data='%v' -- %v\n", s, err)
		} else {
			fmt.Printf("PUB after Send: Data='%v'\n", s)
		}
		time.Sleep(2 * time.Second)
	}
}
