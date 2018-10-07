package comm

import (
	"encoding/json"
	"errors"
	"fmt"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
)

func OpenTraviotaChan(traviotaURI string) (chan *SenderUpdate, error) {
	var sock mangos.Socket
	var err error

	if sock, err = sub.NewSocket(); err != nil {
		return nil, errors.New(fmt.Sprintf("can't get new sub socket: %v", err))
	}
	//sock.AddTransport(ipc.NewTransport())
	sock.AddTransport(tcp.NewTransport())

	if err = sock.Dial(traviotaURI); err != nil {
		return nil, errors.New(fmt.Sprintf("can't dial sub socket: %v", err))
	}
	err = sock.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		return nil, errors.New(fmt.Sprintf("can't subscribe to all topics: %v", err))
	}
	chanUpd := make(chan *SenderUpdate)
	var msg []byte
	var upd *SenderUpdate
	go func() {
		for {
			// TODO error logging
			msg, err = sock.Recv()
			if err == nil {
				upd = &SenderUpdate{}
				err = json.Unmarshal(msg, &upd)
				if err == nil {
					chanUpd <- upd
				}
			}
		}
	}()
	return chanUpd, nil
}
