package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib"
	"runtime"
	"strings"
	"sync"
	"time"
)

/*
173.249.16.125,
207.180.197.79,
188.68.58.32,
165.227.24.40,
173.249.46.93,
173.249.43.185,
173.249.34.67,
167.99.167.136,
51.158.66.236,
121.138.60.212,
193.30.120.67,
85.214.88.177,
79.13.107.8,
213.136.94.51,
206.189.64.81,
37.59.132.147,
83.169.42.179,
85.214.105.196,
185.144.100.99,
94.23.30.39,
node06.iotamexico.com,
159.69.207.216,
5.189.154.131,
207.180.228.135,
85.214.227.149,
node06.iotatoken.nl,
5.45.111.83,
159.69.54.69,
173.249.19.206,
94.16.120.108,
173.212.200.186,
173.212.204.177,
167.99.134.249,
173.249.16.62,
85.214.148.21,
173.249.42.88,
0v0.science,
207.180.231.49,
173.212.214.95,
5.189.152.244,
173.249.50.233,
185.228.137.166,
37.120.174.20,
nodes.tangled.it,
195.201.38.175,
5.189.133.22,
207.180.245.104,
163.172.180.44,
node04.iotatoken.nl,
46.163.78.156,
5.189.157.6,
node.deviceproof.org,
144.76.138.212,
173.249.52.236,
207.180.199.184,
173.249.23.161,
178.128.172.20,
89.163.242.213,
5.189.140.162,
144.76.106.187,
173.249.17.151,
173.249.16.55,
iotanode1.dyndns.biz
*/

var hosts = []string{
	"tcp://snap1.iota.partners:5556",
	"tcp://snap2.iota.partners:5556",
	"tcp://snap3.iota.partners:5556",
	"tcp://node.iotalt.com:31415",
}

var routines = make(map[string]bool)
var mutexRoutines = &sync.Mutex{}

var txcount int
var ctxcount int
var mutexCounts = &sync.Mutex{}

func main() {
	var wg sync.WaitGroup
	go runZmqStarter()
	wg.Add(1)
	for _, uri := range hosts {
		runZmq(uri)
	}
	wg.Wait()
}

func errorf(format string, args ...interface{}) {
	fmt.Printf("ERRO "+format+"\n", args...)
}

func debugf(format string, args ...interface{}) {
	fmt.Printf("DEBU "+format+"\n", args...)
}

func infof(format string, args ...interface{}) {
	fmt.Printf("INFO "+format+"\n", args...)
}

func runZmq(uri string) {
	mutexRoutines.Lock()
	defer mutexRoutines.Unlock()

	_, ok := routines[uri]
	if ok {
		return
	}
	routines[uri] = false
}

func runZmqStarter() {
	for {
		time.Sleep(10 * time.Second)
		mutexRoutines.Lock()
		for uri, running := range routines {
			if !running {
				routines[uri] = true
				go zmqRoutine(uri)
			}
		}
		mutexRoutines.Unlock()
	}
}

func incTx() {
	mutexCounts.Lock()
	defer mutexCounts.Unlock()
	txcount++
}

func incCtx() {
	mutexCounts.Lock()
	defer mutexCounts.Unlock()
	ctxcount++
}

func getTxCtx() (int, int) {
	mutexCounts.Lock()
	defer mutexCounts.Unlock()
	return txcount, ctxcount
}

var topics = []string{"tx", "sn", "lmi"}

func zmqRoutine(uri string) {
	defer func() {
		mutexRoutines.Lock()
		defer mutexRoutines.Unlock()
		routines[uri] = false
	}()
	defer infof("Leaving zmq routine for %v", uri)

	socket, err := lib.OpenSocketAndSubscribe(uri, topics)
	if err != nil {
		errorf("Error while starting zmq channel for %v", uri)
		return
	}
	infof("Opened zmq channel for %v", uri)

	//var seen bool
	//var before time.Time
	var hash string
	for {
		msg, err := socket.Recv()
		if err != nil {
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
			incTx()
			hash = message[1]
			_, _ = seenBeforeTX(hash)
		case "sn":
			incCtx()
			hash = message[2]
			_, _ = seenBeforeSN(hash)
		case "lmi":
		}
	}
}

func init() {
	go runtest()
}

func runtest() {
	var numseqTX, mumtxTX int
	var numseqSN, mumtxSN int
	for {
		time.Sleep(5 * time.Second)

		numseqTX, mumtxTX = sizeTX()
		numseqSN, mumtxSN = sizeSN()
		txc, ctxc := getTxCtx()
		debugf("numsegTX: %d numtxTX:%d  numsegSN: %d numtxSN: %d tx: %d ctx: %d",
			numseqTX, mumtxTX, numseqSN, mumtxSN, txc, ctxc)
		logRuntimeStats()
	}
}

func logRuntimeStats() {
	var mem runtime.MemStats

	runtime.ReadMemStats(&mem)
	debugf("------- DEBUG:RuntimeStats MB: Alloc = %v TotalAlloc = %v Sys = %v NumGC = %v NumGoroutines = %d",
		bToMb(mem.Alloc),
		bToMb(mem.TotalAlloc),
		bToMb(mem.Sys),
		mem.NumGC,
		runtime.NumGoroutine(),
	)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
