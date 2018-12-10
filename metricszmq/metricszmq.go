package metricszmq

import (
	"context"
	"fmt"
	"github.com/go-zeromq/zmq4"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"sync"
	"time"
)

// TODO detect dead ZMQ streams

var (
	zmqMetricsTxCounter        *prometheus.CounterVec
	zmqMetricsCtxCounter       *prometheus.CounterVec
	zmqMetricsMilestoneCounter *prometheus.CounterVec
)

var logLocal *logging.Logger

func debugf(format string, args ...interface{}) {
	if logLocal != nil {
		logLocal.Debugf(format, args...)
	}
}

func errorf(format string, args ...interface{}) {
	if logLocal != nil {
		logLocal.Errorf(format, args...)
	}
}

func OpenSocket(uri string, timeoutSec int) (zmq4.Socket, error) {
	debugf("Opening ZMQ socket for %v, timeout %v sec", uri, timeoutSec)

	//ctx, _ := context.WithTimeout(context.Background(), time.Duration(timeoutSec) * time.Second)
	ctx := context.Background()
	socket := zmq4.NewSub(ctx, zmq4.WithDialerTimeout(time.Duration(timeoutSec)*time.Second))
	err := socket.Dial(uri)
	return socket, err
	//var err error
	//dialCh := make(chan error)
	//defer close(dialCh)
	//go func() {
	//	err = socket.Dial(uri)
	//	dialCh <- err
	//}()
	//select {
	//case err = <-dialCh:
	//case <-time.After(time.Duration(timeoutSec) * time.Second):
	//	err = fmt.Errorf("can't open ZMQ socket for %v", uri)
	//}
}

type zmqRoutineStatus struct {
	running bool
	reading bool
}

var zmqRoutines = make(map[string]zmqRoutineStatus)
var zmqRoutinesMutex sync.Mutex

func InitMetricsZMQ(logParam *logging.Logger, aec lib.ErrorCounter) {
	logLocal = logParam
	initMetrics()
	if aec == nil {
		aec = &lib.DummyAEC{}
	}
	go zmqStarter(aec)
}

func RunZMQMetricsFor(uri string) {
	zmqRoutinesMutex.Lock()
	defer zmqRoutinesMutex.Unlock()

	if _, ok := zmqRoutines[uri]; ok {
		return
	}
	zmqRoutines[uri] = zmqRoutineStatus{}
}

func getZmqRoutineStatus(uri string) (*zmqRoutineStatus, error) {
	zmqRoutinesMutex.Lock()
	defer zmqRoutinesMutex.Unlock()
	status, ok := zmqRoutines[uri]
	if ok {
		return &status, nil
	}
	return nil, fmt.Errorf("not registered %v", uri)
}

func setZmqRoutineStatus(uri string, running bool, reading bool) error {
	zmqRoutinesMutex.Lock()
	defer zmqRoutinesMutex.Unlock()
	status, ok := zmqRoutines[uri]
	if !ok {
		return fmt.Errorf("zmq routine '%v' doesn't exist", uri)
	}
	status.running = running
	status.reading = reading
	zmqRoutines[uri] = status
	return nil
}

var topics = []string{"tx", "sn", "lmi"}

func openSocketAndSubscribe(uri string) (zmq4.Socket, error) {
	socket, err := OpenSocket(uri, 5)
	if err != nil {
		return nil, err
	}
	debugf("ZMQ socket opened for %v\n", uri)
	for _, t := range topics {
		err = socket.SetOption(zmq4.OptionSubscribe, t)
		if err != nil {
			return nil, err
		}
	}
	return socket, nil
}

// half-parsed messages from IRI ZMQ
func runZmqRoutine(uri string, aec lib.ErrorCounter) {
	debugf("ZMQ listener for %v started", uri)
	defer setZmqRoutineStatus(uri, false, false)
	defer errorf("+++++++++++ stopping ZMQ listener for %v due to errors", uri)

	socket, err := openSocketAndSubscribe(uri)
	if err != nil {
		errorf("openSocketAndSubscribe for '%v' returned: %v", uri, err)
		time.Sleep(5 * time.Second)
		return
	}
	defer func() {
		err := socket.Close()
		errorf("socket for %v closed. Err = %v", uri, err)
	}()

	setZmqRoutineStatus(uri, true, true)

	var txcount uint64
	var ctxcount uint64
	var lmicount uint64
	var prevtxcount uint64
	var prevctxcount uint64
	var prevlmicount uint64

	st := time.Now()

	for {
		msg, err := socket.Recv()
		if aec.CheckError("ZMQ", err) {
			errorf("reading ZMQ socket for '%v': socket.Recv() returned %v", uri, err)
			return // exit routine
		}
		if len(msg.Frames) == 0 {
			errorf("+++++++++ empty zmq message for '%v': %+v", uri, msg)
			return
		}
		message := strings.Split(string(msg.Frames[0]), " ")
		messageType := message[0]
		if !lib.StringInSlice(messageType, topics) {
			continue
		}
		switch messageType {
		case "tx":
			zmqMetricsTxCounter.With(prometheus.Labels{"host": uri}).Inc()
			txcount++
		case "sn":
			zmqMetricsCtxCounter.With(prometheus.Labels{"host": uri}).Inc()
			ctxcount++
		case "lmi":
			zmqMetricsMilestoneCounter.With(prometheus.Labels{"host": uri}).Inc()
			lmicount++
		}
		if time.Since(st) > 10*time.Second {
			logLocal.Infof("%v since start: tx = %d (+%d) ctx = %d (+%d) lmi = %d (+%d)",
				uri, txcount, txcount-prevtxcount, ctxcount, ctxcount-prevctxcount, lmicount, lmicount-prevlmicount)
			prevtxcount, prevctxcount, prevlmicount = txcount, ctxcount, lmicount
			st = time.Now()
		}
	}
}

func getUris() []string {
	zmqRoutinesMutex.Lock()
	defer zmqRoutinesMutex.Unlock()
	ret := make([]string, 0, len(zmqRoutines))
	for uri := range zmqRoutines {
		ret = append(ret, uri)
	}
	return ret
}

func zmqStarter(aec lib.ErrorCounter) {
	countRunning := 0

	for {
		countRunning = 0

		for _, uri := range getUris() {
			status, err := getZmqRoutineStatus(uri)
			if err == nil && !status.running {
				setZmqRoutineStatus(uri, true, false)
				go runZmqRoutine(uri, aec)
			} else {
				countRunning++
			}
		}
		time.Sleep(5 * time.Second)
	}
}

func initMetrics() {
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
}
