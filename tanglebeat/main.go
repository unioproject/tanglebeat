package main

import (
	"github.com/lunfardo314/tanglebeat/pubsub"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	readConfig("tanglebeat.yaml")
	log.Infof("Will be receiving transaction data from '%v'", Config.TraviotaURI)
	initDB()

	chanUpdate, err := pubsub.OpenTraviotaChan(Config.TraviotaURI)
	if err != nil {
		log.Criticalf("can't get new sub socket: %v", err)
	}

	for upd := range chanUpdate {
		log.Debugf("Received update '%v'", upd.UpdType)
	}
}
