package cfg

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib/config"
	"github.com/op/go-logging"
	"os"
)

const (
	Version   = "19.04.08"
	logFormat = "%{time:2006-01-02 15:04:05.000} %{level:.4s} [%{module:.6s}|%{shortfunc:.12s}] %{message}"
	level     = logging.DEBUG
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
	InputsZMQ     []string `yaml:"inputsZMQ"`
	InputsNanomsg []string `yaml:"inputsNanomsg"`
}

type ConfigStructYAML struct {
	WebServerPort                       int          `yaml:"webServerPort"`
	IriMsgStream                        inputsOutput `yaml:"iriMsgStream"`
	SenderMsgStream                     inputsOutput `yaml:"senderMsgStream"`
	RetentionPeriodMin                  int          `yaml:"retentionPeriodMin"`
	QuorumTxToPass                      int          `yaml:"quorumToPass"`
	QuorumMilestoneHashToPass           int          `yaml:"quorumMilestoneHashToPass"`
	TimeIntervalMilestoneHashToPassMsec uint64       `yaml:"timeIntervalMilestoneHashToPassMsec"`
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
	msgBeforeLog, _, success = config.ReadYAML(cfgfile, msgBeforeLog, &Config)
	if !success {
		flushMsgBeforeLog(msgBeforeLog)
		os.Exit(1)
	}
	msgBeforeLog, success = initLogging(msgBeforeLog)
	flushMsgBeforeLog(msgBeforeLog)

	if !success {
		os.Exit(1)
	}
	// default values
	if Config.QuorumTxToPass == 0 {
		Config.QuorumTxToPass = 2
	}
	if Config.RetentionPeriodMin == 0 {
		Config.RetentionPeriodMin = 60
	}
	infof("Quorum to pass a message: TX message will be accepted after received %v times from different sources",
		Config.QuorumTxToPass)

	if Config.QuorumMilestoneHashToPass == 0 {
		Config.QuorumMilestoneHashToPass = 3
	}
	if Config.TimeIntervalMilestoneHashToPassMsec == 0 {
		Config.TimeIntervalMilestoneHashToPassMsec = 5000
	}
}

//func errorf(format string, args ...interface{}) {
//	log.Errorf(format, args...)
//}
//
//func debugf(format string, args ...interface{}) {
//	log.Debugf(format, args...)
//}

func infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}
