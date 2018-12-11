package main

import "sync"

var hosts = []string{
	"tcp://snap1.iota.partners:5556",
	"tcp://snap2.iota.partners:5556",
	"tcp://snap3.iota.partners:5556",
	"tcp://node.iotalt.com:31415",
}

type zmqStatus struct {
	running bool
	reading bool
}

var routines = make(map[string]zmqStatus)
var mutexRoutines = &sync.Mutex{}

func main() {

}
