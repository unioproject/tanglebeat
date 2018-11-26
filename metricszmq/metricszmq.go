package metricszmq

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-zeromq/zmq4"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"time"
)

var (
	zmqMetricsTxCounter        prometheus.Counter
	zmqMetricsCtxCounter       prometheus.Counter
	zmqMetricsMilestoneCounter prometheus.Counter
)

func openSocket(uri string, timeoutSec int) (zmq4.Socket, error) {
	logLocal.Debugf("Opening ZMQ socket for %v, timeout %v sec", uri, timeoutSec)

	socket := zmq4.NewSub(context.Background())
	var err error
	dialCh := make(chan error)
	defer close(dialCh)
	go func() {
		err = socket.Dial(uri)
		dialCh <- err
	}()
	select {
	case err = <-dialCh:
	case <-time.After(time.Duration(timeoutSec) * time.Second):
		err = errors.New(fmt.Sprintf("can't open ZMQ socket for %v", uri))
	}
	return socket, err
}

// half-parsed messages from IRI ZMQ
func startReadingIRIZmq(uri string) error {
	socket, err := openSocket(uri, 5)
	if err != nil {
		return err
	}
	logLocal.Debugf("ZMQ socket opened for %v\n", uri)

	topics := []string{"tx", "sn", "lmi"}
	for _, t := range topics {
		err = socket.SetOption(zmq4.OptionSubscribe, t)
		if err != nil {
			return err
		}
	}

	go func() {
		logLocal.Debugf("ZMQ listener created successfully")

		for {
			msg, err := socket.Recv()
			if err != nil {
				logLocal.Errorf("reading ZMQ socket: socket.Recv() returned %v", err)
				time.Sleep(5 * time.Second)
				continue
			}
			message := strings.Split(string(msg.Frames[0]), " ")
			messageType := message[0]
			if !lib.StringInSlice(messageType, topics) {
				continue
			}
			switch messageType {
			case "tx":
				zmqMetricsTxCounter.Inc()
			case "sn":
				zmqMetricsCtxCounter.Inc()
			case "lmi":
				zmqMetricsMilestoneCounter.Inc()
			}
		}
	}()
	return nil
}

var logLocal *logging.Logger

func InitMetricsZMQ(uri string, logParam *logging.Logger) error {
	logLocal = logParam
	zmqMetricsTxCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tanglebeat_tx_counter",
		Help: "Transaction counter",
	})

	zmqMetricsCtxCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tanglebeat_ctx_counter",
		Help: "Confirmed transaction counter",
	})
	zmqMetricsMilestoneCounter = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "tanglebeat_milestone_counter",
		Help: "Milestone counter",
	})

	prometheus.MustRegister(zmqMetricsTxCounter)
	prometheus.MustRegister(zmqMetricsCtxCounter)
	prometheus.MustRegister(zmqMetricsMilestoneCounter)

	err := startReadingIRIZmq(uri)
	if err != nil {
		logLocal.Errorf("cant't initialize zmq metrics updater: %v", err)
	}
	return err
}
