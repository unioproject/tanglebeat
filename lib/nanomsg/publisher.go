package nanomsg

import (
	"encoding/json"
	"fmt"
	"github.com/op/go-logging"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/pub"
	"nanomsg.org/go-mangos/transport/tcp"
	"time"
)

type Publisher struct {
	chIn chan []byte
	sock mangos.Socket
	url  string
	log  *logging.Logger
}

func (p *Publisher) errorf(format string, args ...interface{}) {
	if p.log != nil {
		p.log.Errorf(format, args...)
	}
}

func (p *Publisher) infof(format string, args ...interface{}) {
	if p.log != nil {
		p.log.Infof(format, args...)
	}
}

// reads input stream of byte arrays and sends them to publish channel
func NewPublisher(port int, bufflen int, localLog *logging.Logger) (*Publisher, error) {
	ret := Publisher{
		log: localLog,
	}
	var err error
	if ret.sock, err = pub.NewSocket(); err != nil {
		return nil, fmt.Errorf("can't get new sub socket: %v", err)
	}

	ret.chIn = make(chan []byte, bufflen)
	ret.sock.AddTransport(tcp.NewTransport())
	ret.url = fmt.Sprintf("tcp://:%v", port)
	if err = ret.sock.Listen(ret.url); err != nil {
		return nil, fmt.Errorf("can't listen new pub socket: %v", err)
	}
	ret.infof("Publisher: PUB socket listening on %v", ret.url)
	go func() {
		ret.loop()
		ret.sock.Close()
	}()
	return &ret, nil
}

func (p *Publisher) loop() {
	for data := range p.chIn {
		err := p.sock.Send(data)
		if err != nil {
			p.errorf("Nanomsg publisher of %v: %v", p.url, err)
		}
	}
}

func (p *Publisher) PublishData(data []byte) error {
	select {
	case p.chIn <- data:
	case <-time.After(5 * time.Second):
		return fmt.Errorf("----- Timeout 5 sec on sending to publish channel at %v", p.url)
	}
	return nil
}

func (p *Publisher) PublishAsJSON(obj interface{}) error {
	data, err := json.Marshal(obj)
	if err != nil {
		p.errorf("Publisher: marshal error %v", err)
		return err
	}
	return p.PublishData(data)
}
