package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib"
)

var hosts = []string{
	"tcp://snap1.iota.partners:5556",
	"tcp://snap2.iota.partners:5556",
	"tcp://snap3.iota.partners:5556",
	"tcp://node.iotalt.com:31416",
}

func main() {
	for _, host := range hosts {
		_, err := lib.OpenSocket(host, 5)
		if err != nil {
			fmt.Printf("%v --> can't open socket: %v\n", host, err)
		} else {
			fmt.Printf("%v --> socket was opened succesfully\n", host)

		}
	}
}
