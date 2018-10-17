package main

import (
	_ "github.com/mattn/go-sqlite3"
	"os"
)

func main() {
	readConfig("tb_metrics.yml")
	log.Infof("Will be receiving transaction data from '%v'", Config.SenderURI)
	initDB()
	err := read1hConfirmedFromDB()
	if err != nil {
		log.Criticalf("read1hConfirmedFromDB: %v", err)
		os.Exit(1)
	}
	go runUpdateDb()
	go exposeMetrics()
	refreshMetrics(5)
}
