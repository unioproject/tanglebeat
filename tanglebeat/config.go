package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/config"
	"github.com/lunfardo314/tanglebeat/lib/ebuffer"
	"github.com/lunfardo314/tanglebeat/tanglebeat/inreaders"
	"github.com/lunfardo314/tanglebeat/tanglebeat/senderpart"
	"github.com/lunfardo314/tanglebeat/tanglebeat/zmqpart"
	"github.com/op/go-logging"
	"os"
)

const (
	version   = "0.21"
	logFormat = "%{time:2006-01-02 15:04:05.000} %{level:.4s} [%{module:.6s}|%{shortfunc:.12s}] %{message}"
	level     = logging.DEBUG
)

var (
	log            *logging.Logger
	logInitialized bool
)

type inputsOutput struct {
	OutputEnabled bool     `yaml:"outputEnabled"`
	OutputPort    int      `yaml:"outputPort"`
	Inputs        []string `yaml:"inputs"`
}

type ConfigStructYAML struct {
	siteDataDir      string
	WebServerPort    int          `yaml:"webServerPort"`
	IriMsgStream     inputsOutput `yaml:"iriMsgStream"`
	SenderMsgStream  inputsOutput `yaml:"senderMsgStream"`
	RepeatToAcceptTX int          `yaml:"repeatToAcceptTX"`
}

var Config = ConfigStructYAML{}

func initLogging(msgBeforeLog []string) ([]string, bool) {
	log = logging.MustGetLogger("main")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	logFormat := logging.MustStringFormatter(logFormat)
	backendFormatter := logging.NewBackendFormatter(backend, logFormat)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "main")
	log.SetBackend(backendLeveled)
	logInitialized = true
	return msgBeforeLog, true
}

func flushMsgBeforeLog(msgBeforeLog []string) {
	for _, msg := range msgBeforeLog {
		if logInitialized {
			log.Info(msg)
		} else {
			fmt.Println(msg)
		}
	}
}

func mustReadConfig(cfgfile string) {
	msgBeforeLog := make([]string, 0, 10)
	msgBeforeLog = append(msgBeforeLog, "---- Starting Tanglebeat hub module ver. "+version)
	var success bool
	var siteDataDir string
	msgBeforeLog, siteDataDir, success = config.ReadYAML(cfgfile, msgBeforeLog, &Config)
	if !success {
		flushMsgBeforeLog(msgBeforeLog)
		os.Exit(1)
	}
	Config.siteDataDir = siteDataDir
	msgBeforeLog, success = initLogging(msgBeforeLog)
	flushMsgBeforeLog(msgBeforeLog)

	if !success {
		os.Exit(1)
	}
	if Config.RepeatToAcceptTX == 0 {
		Config.RepeatToAcceptTX = 2
	}

	zmqpart.SetRepeatToAcceptTX(Config.RepeatToAcceptTX) // global setting for zmq part

	infof("TX message will be accepted after received %v times from different sources ('repeatToAcceptTX' parameter, default is 2)",
		Config.RepeatToAcceptTX)

	inreaders.SetLog(log, true)
	zmqpart.SetLog(log, false)
	senderpart.SetLog(log, false)
	ebuffer.SetLog(log, false)
}

func errorf(format string, args ...interface{}) {
	log.Errorf(format, args...)
}

func debugf(format string, args ...interface{}) {
	log.Debugf(format, args...)
}

func infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}
