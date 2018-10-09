package main

import (
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	readConfig("tanglebeat.yaml")
	log.Infof("Will be receiving transaction data from '%v'", Config.SenderURI)
	initDB()

	runUpdateDb()
}
