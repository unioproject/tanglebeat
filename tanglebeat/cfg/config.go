package cfg

import (
	"fmt"
	"github.com/op/go-logging"
	"github.com/unioproject/tanglebeat/lib/config"
	"os"
)

const (
	Version   = "unio 19.06.06-1"
	logFormat = "%{time:2006-01-02 15:04:05.000} %{level:.4s} [%{module:.8s}|%{shortfunc:.12s}] %{message}"
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
	Debug                               bool         `yaml:"debug"`
	WebServerPort                       int          `yaml:"webServerPort"`
	IriMsgStream                        inputsOutput `yaml:"iriMsgStream"`
	SenderMsgStream                     inputsOutput `yaml:"senderMsgStream"`
	RetentionPeriodMin                  int          `yaml:"retentionPeriodMin"`
	QuorumTxToPass                      int          `yaml:"quorumToPass"`
	QuorumMilestoneHashToPass           int          `yaml:"quorumMilestoneHashToPass"`
	TimeIntervalMilestoneHashToPassMsec uint64       `yaml:"timeIntervalMilestoneHashToPassMsec"`
	MultiQuorumMetricsEnabled           bool         `yaml:"multiQuorumMetricsEnabled"`
	QuorumUpdatesEnabled                bool         `yaml:"quorumUpdatesEnabled"`
	QuorumUpdatesFrom                   int          `yaml:"quorumUpdatesFrom"`
	QuorumUpdatesTo                     int          `yaml:"quorumUpdatesTo"`
	SpawnCmd                            []string     `yaml:"spawnCmd"`
}

var Config = ConfigStructYAML{}

func initLogging(msgBeforeLog []string) ([]string, bool) {
	log = logging.MustGetLogger("tanglebeat")
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	logFormat := logging.MustStringFormatter(logFormat)
	backendFormatter := logging.NewBackendFormatter(backend, logFormat)
	backendLeveled := logging.AddModuleLevel(backendFormatter)
	backendLeveled.SetLevel(level, "tanglebeat")
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

	infof("Debug = %v", Config.Debug)
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
	infof("MultiQuorum metrics enabled = %v", Config.MultiQuorumMetricsEnabled)
	infof("QuorumUpdatesEnabled = %v", Config.QuorumUpdatesEnabled)
	if Config.QuorumUpdatesEnabled {
		if Config.QuorumUpdatesFrom == 0 {
			Config.QuorumUpdatesFrom = 1
		}
		if Config.QuorumUpdatesTo == 0 {
			Config.QuorumUpdatesTo = 5
		}
		infof("QuorumUpdatesFrom = %d, QuorumUpdatesTo = %d",
			Config.QuorumUpdatesFrom, Config.QuorumUpdatesTo)
	}
}

func infof(format string, args ...interface{}) {
	log.Infof(format, args...)
}
