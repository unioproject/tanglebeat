package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-zeromq/zmq4"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"time"
)

var (
	//zmqMetricsCurrentMilestone     prometheus.Gauge
	//zmqMetricsSecBetweenMilestones prometheus.Gauge
	zmqMetricsTxCounter        prometheus.Counter
	zmqMetricsCtxCounter       prometheus.Counter
	zmqMetricsMilestoneCounter prometheus.Counter
)

func openSocket(uri string, timeoutSec int) (zmq4.Socket, error) {
	log.Debugf("Opening ZMQ socket for %v, timeout %v sec", uri, timeoutSec)

	socket := zmq4.NewSub(context.Background())
	var err error
	dial_ch := make(chan error)
	defer close(dial_ch)
	go func() {
		err = socket.Dial(uri)
		dial_ch <- err
	}()
	select {
	case err = <-dial_ch:
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
	log.Debugf("ZMQ socket opened for %v\n", uri)

	topics := []string{"tx", "sn", "lmi"}
	for _, t := range topics {
		err = socket.SetOption(zmq4.OptionSubscribe, t)
		if err != nil {
			return err
		}
	}

	go func() {
		log.Debugf("ZMQ listener created succesfully")
		//var lastMilestoneUnixTs int64

		for {
			msg, err := socket.Recv()
			if err != nil {
				log.Errorf("reading ZMQ socket: socket.Recv() returned %v", err)
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

				//if len(message) < 3 {
				//	log.Errorf("reading ZMQ socket: unexpected 'lmi' message format")
				//}
				//if nextMilestone, err := strconv.Atoi(message[2]); err != nil {
				//	log.Errorf("reading ZMQ socket: 'lmi' error: %v", err)
				//} else {
				//	zmqMetricsCurrentMilestone.Set(float64(nextMilestone))
				//	nowis := lib.UnixMs(time.Now())
				//	if lastMilestoneUnixTs != 0 {
				//		zmqMetricsSecBetweenMilestones.Set(float64(nowis-lastMilestoneUnixTs) / 1000)
				//		log.Debugf("ZMQ metrics updater: milestone changed: %v --> %v", message[1], message[2])
				//	}
				//	lastMilestoneUnixTs = nowis
				//}
			}
		}
	}()
	return nil
}

func initMetricsZMQ() error {
	//zmqMetricsCurrentMilestone = prometheus.NewGauge(prometheus.GaugeOpts{
	//	Name: "tanglebeat_current_milestone",
	//	Help: "Current milestone",
	//})
	//zmqMetricsSecBetweenMilestones = prometheus.NewGauge(prometheus.GaugeOpts{
	//	Name: "tanglebeat_sec_between_milestones",
	//	Help: "Duration between last and previous milestone in seconds",
	//})

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

	//prometheus.MustRegister(zmqMetricsCurrentMilestone)
	//prometheus.MustRegister(zmqMetricsSecBetweenMilestones)
	prometheus.MustRegister(zmqMetricsTxCounter)
	prometheus.MustRegister(zmqMetricsCtxCounter)
	prometheus.MustRegister(zmqMetricsMilestoneCounter)

	err := startReadingIRIZmq(Config.Prometheus.ZmqMetrics.ZMQUri)
	if err != nil {
		log.Errorf("cant't initialize zmq metrics updater: %v", err)
	}
	return err
}
