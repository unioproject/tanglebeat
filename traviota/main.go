package main

import (
	log "github.com/sirupsen/logrus"
)

func main() {
	ReadConfig("traviota.yaml")
	log.Info("KUKU info")
}
