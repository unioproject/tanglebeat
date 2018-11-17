package main

import (
	"flag"
	"fmt"
	"github.com/lunfardo314/tanglebeat1/sender_update"
)

// tanglebeat client example
// reads update stream published by specified tanglebeat instance

func main() {

	puri := flag.String("target", "tcp://tanglebeat.com:3100", "target to listen")
	pshowall := flag.Bool("all", false, "show all updates. By default shows only 'confirm'")
	flag.Parse()

	uri := *puri
	showall := *pshowall

	fmt.Printf("Initializing tb_updates for %v, showall = %v\n", uri, showall)
	chIn, err := sender_update.NewUpdateChan(uri)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Start receiving sender updates from %v\n", uri)

	for upd := range chIn {
		if showall || upd.UpdType == sender_update.SENDER_UPD_CONFIRM {
			fmt.Printf("Received '%v' from %v(%v), idx = %v, addr = %v\n",
				upd.UpdType, upd.SeqUID, upd.SeqName, upd.Index, upd.Addr)
		}
	}
}
