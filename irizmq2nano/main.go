package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib"
	"runtime"
	"strings"
	"sync"
	"time"
)

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
