package main

import (
	"encoding/json"
	"fmt"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/sub"
	"nanomsg.org/go-mangos/transport/tcp"
	"os"
	"time"
)

// tanglebeat client example
// reads update stream published by specified tanglebeat instance

func main() {
	var sock mangos.Socket
	var err error

	if len(os.Args) != 2 {
		fmt.Println("Usage: tb_updates <host>:<port>\nExample: tb_updates my.host.com:3100")
		os.Exit(0)
	}
	host_port := os.Args[1]
	uri := fmt.Sprintf("tcp://%v", host_port)

	fmt.Printf("Initializing tb_updates for %v\n", uri)
	if sock, err = sub.NewSocket(); err != nil {
		panic(fmt.Sprintf("can't create new sub socket: %v", err))
	}
	defer sock.Close()

	sock.AddTransport(tcp.NewTransport())
	if err = sock.Dial(uri); err != nil {
		panic(fmt.Sprintf("can't dial sub socket at %v: %v", uri, err))
	}
	err = sock.SetOption(mangos.OptionSubscribe, []byte(""))
	if err != nil {
		panic(fmt.Sprintf("failed to subscribe to all topics at %v: %v", uri, err))
	}
	fmt.Printf("Start reading updates from %v\n", uri)

	var msg []byte
	var upd sender_update.SenderUpdate
	for {
		msg, err = sock.Recv()
		if err == nil {
			err = json.Unmarshal(msg, &upd)
			if err == nil {
				fmt.Printf("Received '%v' from %v(%v), idx = %v, addr = %v\n",
					upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index, upd.Addr)
			} else {
				fmt.Printf("Error while receiving sender update from %v(%v): %v",
					upd.SeqUID, upd.SeqName, err)
				time.Sleep(2 * time.Second)
			}
		}
	}

}
