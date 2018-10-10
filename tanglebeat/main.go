package main

import (
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	readConfig("tanglebeat.yaml")
	log.Infof("Will be receiving transaction data from '%v'", Config.SenderURI)
	initDB()
	//r, err := sumUpBySequence(24 * 60 * 60 * 1000)
	//if err != nil{
	//	log.Panic(err)
	//}
	//log.Infof("SUMS: %+v", r)
	runUpdateDb()
}
