package cfg

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/config"
	"github.com/op/go-logging"
	"os"
)

const (
	Version                = "19.02.05-3"
	logFormat              = "%{time:2006-01-02 15:04:05.000} %{level:.4s} [%{module:.6s}|%{shortfunc:.12s}] %{message}"
	level                  = logging.DEBUG
	onHoldThresholdDefault = 50
)

var (
	log            *logging.Logger
	logInitialized bool
)

func GetLog() *logging.Logger {
	return log
}

type inputsOutput struct {
	OutputEnabled bool     `yaml:"outputEnabled"`
	OutputPort    int      `yaml:"outputPort"`
	Inputs        []string `yaml:"inputs"`
}

type ConfigStructYAML struct {
	siteDataDir        string
	WebServerPort      int          `yaml:"webServerPort"`
	IriMsgStream       inputsOutput `yaml:"iriMsgStream"`
	SenderMsgStream    inputsOutput `yaml:"senderMsgStream"`
	RepeatToAccept     int          `yaml:"repeatToAccept"`
	RetentionPeriodMin int          `yaml:"retentionPeriodMin"`
	OnHoldThreshold    uint64       `yaml:"onHoldThreshold"`
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

func MustReadConfig(cfgfile string) {
	msgBeforeLog := make([]string, 0, 10)
	msgBeforeLog = append(msgBeforeLog, "---- Starting Tanglebeat hub module ver. "+Version)
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
	if Config.RepeatToAccept == 0 {
		Config.RepeatToAccept = 2
	}
	if Config.RetentionPeriodMin == 0 {
		Config.RetentionPeriodMin = 60
	}
	if Config.OnHoldThreshold == 0 {
		Config.OnHoldThreshold = onHoldThresholdDefault
	}
	infof("OnHold threshold for zmq routines is %v%%", Config.OnHoldThreshold)
	infof("TX message will be accepted after received %v times from different sources ('repeatToAcceptTX' parameter, default is 2)",
		Config.RepeatToAccept)
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
