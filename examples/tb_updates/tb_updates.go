package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/sender_update"
	"os"
)

// tanglebeat client example
// reads update stream published by specified tanglebeat instance

func main() {

	if len(os.Args) != 2 {
		fmt.Println("Usage: tb_updates tcp://<host>:<port>\nExample: tb_updates my.host.com:3100")
		os.Exit(0)
	}
	uri := os.Args[1]

	fmt.Printf("Initializing tb_updates for %v\n", uri)
	chIn, err := sender_update.NewUpdateChan(uri)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Start receiving sender updates from %v\n", uri)

	for upd := range chIn {
		fmt.Printf("Received '%v' from %v(%v), idx = %v, addr = %v\n",
			upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index, upd.Addr)
	}
}
