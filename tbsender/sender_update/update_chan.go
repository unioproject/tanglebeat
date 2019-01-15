package sender_update

import (
	"encoding/json"
	"errors"
	"fmt"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"time"
)

// uri must be like "tcp://my.host:3100"

func NewUpdateChan(uri string) (chan *SenderUpdate, error) {
	var sock mangos.Socket
	var err error

	if sock, err = sub.NewSocket(); err != nil {
		return nil, errors.New(fmt.Sprintf("can't create new sub socket: %v", err))
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(uri); err != nil {
		return nil, errors.New(fmt.Sprintf("can't dial sub socket at %v: %v", uri, err))
	}
	err = sock.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed to subscribe to all topics at %v: %v", uri, err))
	}
	chOut := make(chan *SenderUpdate)

	var msg []byte
	var upd *SenderUpdate
	go func() {
		defer sock.Close()
		for {
			msg, err = sock.Recv()
			if err == nil {
				upd = &SenderUpdate{}
				err = json.Unmarshal(msg, &upd)
				if err == nil {
					chOut <- upd
				} else {
					fmt.Printf("Error while unmarshaling sender update from %v: %v\n", uri, err)
					time.Sleep(5 * time.Second)
				}
			}
		}
	}()
	return chOut, nil
}
