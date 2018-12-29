package utils

import (
	"context"
	"github.com/go-zeromq/zmq4"
	"time"
)

func OpenSocket(uri string, timeoutSec int) (zmq4.Socket, error) {
	ctx := context.Background()
	socket := zmq4.NewSub(ctx, zmq4.WithDialerTimeout(time.Duration(timeoutSec)*time.Second))
	err := socket.Dial(uri)
	return socket, err
}

const openSockTimeoutSec = 5

func OpenSocketAndSubscribe(uri string, topics []string) (zmq4.Socket, error) {
	socket, err := OpenSocket(uri, openSockTimeoutSec)
	if err != nil {
		return nil, err
	}
	for _, t := range topics {
		err = socket.SetOption(zmq4.OptionSubscribe, t)
		if err != nil {
			return nil, err
		}
	}
	return socket, nil
}
