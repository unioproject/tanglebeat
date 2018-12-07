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

// TODO detect dead ZMQ streams
// TODO 2 dynamic selection of zmq hosts

var (
	zmqMetricsTxCounter        *prometheus.CounterVec
	zmqMetricsCtxCounter       *prometheus.CounterVec
	zmqMetricsMilestoneCounter *prometheus.CounterVec
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
func startReadingIRIZmq(uri string, aec lib.ErrorCounter) error {
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
		logLocal.Debugf("ZMQ listener for %v created successfully", uri)

		for {
			msg, err := socket.Recv()
			if aec.CheckError("ZMQ", err) {
				logLocal.Errorf("reading ZMQ socket %v: socket.Recv() returned %v", uri, err)
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
				zmqMetricsTxCounter.With(prometheus.Labels{"host": uri}).Inc()
			case "sn":
				zmqMetricsCtxCounter.With(prometheus.Labels{"host": uri}).Inc()
			case "lmi":
				zmqMetricsMilestoneCounter.With(prometheus.Labels{"host": uri}).Inc()
			}
		}
	}()
	return nil
}

var logLocal *logging.Logger

func InitMetricsZMQ(uris []string, logParam *logging.Logger, aec lib.ErrorCounter) int {
	logLocal = logParam
	zmqMetricsTxCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_tx_counter_vec",
		Help: "Transaction counter. Labeled by ZMQ host",
	}, []string{"host"})

	zmqMetricsCtxCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_ctx_counter_vec",
		Help: "Confirmed transaction counter",
	}, []string{"host"})
	zmqMetricsMilestoneCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "tanglebeat_milestone_counter",
		Help: "Milestone counter",
	}, []string{"host"})

	prometheus.MustRegister(zmqMetricsTxCounter)
	prometheus.MustRegister(zmqMetricsCtxCounter)
	prometheus.MustRegister(zmqMetricsMilestoneCounter)

	count := 0
	for _, uri := range uris {
		err := startReadingIRIZmq(uri, aec)
		if err != nil {
			logLocal.Errorf("cant't initialize zmq metrics updater: %v", err)
		} else {
			count++
		}
	}
	return count
}
