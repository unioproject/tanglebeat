package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-zeromq/zmq4"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
	"strings"
	"time"
)

var (
	zmqMetricsCurrentMilestone     prometheus.Gauge
	zmqMetricsSecBetweenMilestones prometheus.Gauge
	zmqMetricsTxcountGauge         prometheus.Gauge
	zmqMetricsCtxcountGauge        prometheus.Gauge
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
		txcount := 0
		ctxcount := 0
		var lastMilestoneUnixTs int64

		nexUpdate := time.Now().Add(1 * time.Minute)
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
				txcount += 1
			case "sn":
				ctxcount += 1
			case "lmi":
				if len(message) < 3 {
					log.Errorf("reading ZMQ socket: unexpected 'lmi' message format")
				}
				if nextMilestone, err := strconv.Atoi(message[2]); err != nil {
					log.Errorf("reading ZMQ socket: 'lmi' error: %v", err)
				} else {
					zmqMetricsCurrentMilestone.Set(float64(nextMilestone))
					nowis := lib.UnixMs(time.Now())
					if lastMilestoneUnixTs != 0 {
						zmqMetricsSecBetweenMilestones.Set(float64(nowis-lastMilestoneUnixTs) / 1000)
					}
					lastMilestoneUnixTs = nowis
					log.Debugf("ZMQ metrics updater: milestone changed: %v --> %v", message[1], message[2])
				}
			}
			if time.Now().After(nexUpdate) {
				zmqMetricsTxcountGauge.Set(float64(txcount))
				zmqMetricsCtxcountGauge.Set(float64(ctxcount))
				nexUpdate = nexUpdate.Add(1 * time.Minute)
				log.Debugf("ZMQ metrics updater: in 1 min: tx = %v ctx = %v", txcount, ctxcount)
				txcount = 0
				ctxcount = 0
			}
		}
	}()
	return nil
}

func initMetricsZMQ() error {
	zmqMetricsCurrentMilestone = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tanglebeat_current_milestone",
		Help: "Current milestone",
	})
	zmqMetricsSecBetweenMilestones = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tanglebeat_sec_between_milestones",
		Help: "Duration is seconds between last and the previous milestone in seconds",
	})

	zmqMetricsTxcountGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tanglebeat_tx_count_1_minute",
		Help: "Transaction count in one minute",
	})

	zmqMetricsCtxcountGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "tanglebeat_ctx_count_1_minute",
		Help: "Confirmed transaction count in one minute",
	})
	prometheus.MustRegister(zmqMetricsCurrentMilestone)
	prometheus.MustRegister(zmqMetricsSecBetweenMilestones)
	prometheus.MustRegister(zmqMetricsTxcountGauge)
	prometheus.MustRegister(zmqMetricsCtxcountGauge)

	err := startReadingIRIZmq(Config.Prometheus.ZmqMetrics.ZMQUri)
	if err != nil {
		log.Errorf("cant't initialize zmq metrics uodater: %v", err)
	}
	return err
}
