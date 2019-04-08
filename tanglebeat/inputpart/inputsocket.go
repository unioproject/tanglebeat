package inputpart

import (
	"fmt"
	"github.com/go-zeromq/zmq4"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"strings"
)

type inSocket interface {
	RecvMsg() ([]byte, []string, error)
	Close()
}

type zmqInSocket struct {
	uri    string
	socket zmq4.Socket
}

type nanomsgInSocket struct {
	uri    string
	socket mangos.Socket
}

func NewZmqSocket(uri string, topics []string) (inSocket, error) {
	sock, err := utils.OpenSocketAndSubscribe(uri, topics)
	if err == nil {
		return &zmqInSocket{uri: uri, socket: sock}, nil
	}
	return nil, err
}

func NewNanomsgSocket(uri string, topics []string) (inSocket, error) {
	sock, err := sub.NewSocket()
	if err != nil {
		return nil, fmt.Errorf("can't create new Mangos sub socket for %v: %v", uri, err)
	}
	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(uri); err != nil {
		return nil, fmt.Errorf("can't dial sub socket for %v: %v", uri, err)
	}
	for _, tpc := range topics {
		err = sock.SetOption(mangos.OptionSubscribe, []byte(tpc))
		if err != nil {
			return nil, fmt.Errorf("failed to subscribe to topics %v at %v: %v", topics, uri, err)
		}
	}
	return &nanomsgInSocket{uri: uri, socket: sock}, nil
}

func (s *zmqInSocket) RecvMsg() ([]byte, []string, error) {
	msg, err := s.socket.Recv()

	if err != nil {
		return nil, nil, fmt.Errorf("reading ZMQ socket for '%v': socket.Recv() returned %v", s.uri, err)
	}
	if len(msg.Frames) == 0 {
		return nil, nil, fmt.Errorf("+++++++++ empty msg from zmq '%v': %+v", s.uri, msg)
	}
	msgSplit := strings.Split(string(msg.Frames[0]), " ")
	return msg.Frames[0], msgSplit, nil
}

func (s *zmqInSocket) Close() {
	// better leak go routine than block or leak socket
	go func() {
		_ = s.socket.Close()
	}()
}

func (s *nanomsgInSocket) RecvMsg() ([]byte, []string, error) {
	msg, err := s.socket.Recv()

	if err != nil {
		return nil, nil, fmt.Errorf("reading Nanomsg socket for '%v': socket.Recv() returned %v", s.uri, err)
	}
	if len(msg) == 0 {
		return nil, nil, fmt.Errorf("+++++++++ empty msg from nanomsg '%v': %+v", s.uri, msg)
	}
	msgSplit := strings.Split(string(msg), " ")
	return msg, msgSplit, nil
}

func (s *nanomsgInSocket) Close() {
	// better leak go routine than block or leak socket
	go func() {
		_ = s.socket.Close()
	}()
}
