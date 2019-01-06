package main

// Version with iota.go library Version

import (
	"fmt"
	. "github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat/lib/config"
	"github.com/lunfardo314/tanglebeat/lib/multiapi"
	"github.com/lunfardo314/tanglebeat/lib/utils"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"io"
	"os"
	"path"
	"runtime"
	"time"
)

const (
	Version                 = "1.3"
	PREFIX_MODULE           = "tbsender"
	ROTATE_LOG_HOURS        = 12
	ROTATE_LOG_RETAIN_HOURS = 12
	defaultTag              = "TANGLE9BEAT"
	defaultTagPromote       = "TANGLE9BEAT"
	defaultAddressPromote   = "TANGLEBEAT9PROMOTE999999999999999999999999999999999999999999999999999999999999999"
)

var (
	log                  *logging.Logger
	logLevel             logging.Level
	logLevelName         string
	masterLoggingBackend logging.LeveledBackend
	logFormatter         logging.Formatter
	logInitialized       bool
)

type ConfigStructYAML struct {
	siteDataDir           string
	Logging               loggingConfigYAML         `yaml:"logging"`
	Sender                senderYAML                `yaml:"sender"`
	SenderUpdatePublisher senderUpdatePublisherYAML `yaml:"senderUpdatePublisher"`
	ExitProgram           exitProgramYAML           `yaml:"exitProgram"`
}

type exitProgramCondition struct {
	Enabled   bool `yaml:"enabled"`
	Threshold int  `yaml:"threshold"`
	RC        int  `yaml:"rc"`
}

type exitProgramYAML struct {
	Enabled   bool                 `yaml:"enabled"`
	Exit1min  exitProgramCondition `yaml:"errorThreshold1min"`
	Exit10min exitProgramCondition `yaml:"errorThreshold10min"`
}

type loggingConfigYAML struct {
	Debug                  bool   `yaml:"debug"`
	WorkingSubdir          string `yaml:"workingSubdir"`
	LogConsoleOnly         bool   `yaml:"logConsoleOnly"`
	RotateLogs             bool   `yaml:"rotateLogs"`
	LogSequencesSeparately bool   `yaml:"logSequencesSeparately"`
	LogFormat              string `yaml:"logFormat"`
	LogFormatDebug         string `yaml:"logFormatDebug"`
	RuntimeStats           bool   `yaml:"logRuntimeStats"`
	RuntimeStatsInterval   int    `yaml:"logRuntimeStatsInterval"`
}

type senderYAML struct {
	Enabled           bool                        `yaml:"enabled"`
	DisableMultiCalls bool                        `yaml:"disableMultiCalls"`
	DebugMultiCalls   bool                        `yaml:"debugMultiCalls"`
	Globals           senderParamsYAML            `yaml:"globals"`
	Sequences         map[string]senderParamsYAML `yaml:"sequences"`
}

type senderParamsYAML struct {
	externalSource        bool     // tmp TODO
	Enabled               bool     `yaml:"enabled"`
	IOTANode              []string `yaml:"iotaNode"`
	IOTANodeTipsel        []string `yaml:"iotaNodeTipsel"`
	IOTANodePoW           string   `yaml:"iotaNodePOW"`
	TimeoutAPI            uint64   `yaml:"apiTimeout"`
	TimeoutTipsel         uint64   `yaml:"tipselTimeout"`
	TimeoutPoW            uint64   `yaml:"powTimeout"`
	Seed                  string   `yaml:"seed"`
	Index0                uint64   `yaml:"index0"`
	TxTag                 string   `yaml:"txTag"`
	TxTagPromote          string   `yaml:"txTagPromote"`
	AddressPromote        string   `yaml:"addressPromote"`
	ForceReattachAfterMin uint64   `yaml:"forceReattachAfterMin"`
	PromoteChain          bool     `yaml:"promoteChain"`
	PromoteEverySec       uint64   `yaml:"promoteEverySec"`
	PromoteDisable        bool     `yaml:"promoteDisable"`
}

type senderUpdatePublisherYAML struct {
	Enabled    bool `yaml:"enabled"`
	OutputPort int  `yaml:"outputPort"`
}

// main config structure
var Config = ConfigStructYAML{}

func (params *senderParamsYAML) GetUID() string {
	seedT, err := NewTrytes(params.Seed)
	if err != nil {
		panic("can't generate UID")
	}
	hash, err := utils.KerlTrytes(seedT)
	if err != nil {
		panic("can't generate UID")
	}
	ret := string(hash)
	return ret[len(ret)-UID_LEN:]
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

func mustReadMasterConfig(configFilename string) {
	msgBeforeLog := make([]string, 0, 10)
	msgBeforeLog = append(msgBeforeLog, "---- Starting Tanglebeat 'sender' module: tbsender ver. "+Version)
	var siteDataDir string
	var success bool
	msgBeforeLog, siteDataDir, success = config.ReadYAML(configFilename, msgBeforeLog, &Config)
	if !success {
		flushMsgBeforeLog(msgBeforeLog)
		os.Exit(1)
	}
	Config.siteDataDir = siteDataDir
	msgBeforeLog, success = configMasterLogging(msgBeforeLog)
	flushMsgBeforeLog(msgBeforeLog)
	if !success {
		os.Exit(1)
	}

	configDebugging()

	if Config.Sender.DisableMultiCalls {
		multiapi.DisableMultiAPI()
		log.Infof("Multi calls to IOTA API are DISABLED")
	} else {
		log.Infof("Multi calls to IOTA API are ENABLED")
	}

	if Config.ExitProgram.Enabled {
		log.Infof("Program exit conditions are ENABLED")
		if Config.ExitProgram.Exit1min.Enabled {
			log.Infof("Program will exit with RC = %d if error count exceeds %d in 1 minute",
				Config.ExitProgram.Exit1min.RC, Config.ExitProgram.Exit1min.Threshold)
		}
		if Config.ExitProgram.Exit10min.Enabled {
			log.Infof("Program will exit with RC = %d if error count exceeds %d in 10 minutes",
				Config.ExitProgram.Exit10min.RC, Config.ExitProgram.Exit10min.Threshold)
		}
	} else {
		log.Infof("Program exit conditions are DISABLED")

	}
}

func configDebugging() {
	if Config.Logging.Debug && Config.Logging.RuntimeStats {
		sl := utils.Max(5, Config.Logging.RuntimeStatsInterval)
		go func() {
			for {
				logRuntimeStats()
				time.Sleep(time.Duration(sl) * time.Second)
			}
		}()
		log.Infof("Will be logging RuntimeStats every %v sec", sl)
	}
}

func configMasterLogging(msgBeforeLog []string) ([]string, bool) {
	var logWriter io.Writer

	if Config.Logging.Debug {
		logLevel = logging.DEBUG
		logLevelName = "DEBUG"
		logFormatter = logging.MustStringFormatter(Config.Logging.LogFormatDebug)
	} else {
		logLevel = logging.INFO
		logLevelName = "INFO"
		logFormatter = logging.MustStringFormatter(Config.Logging.LogFormat)
	}

	// opening log file if necessary
	if Config.Logging.LogConsoleOnly {
		logWriter = os.Stderr
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Will be logging at %v level to stderr only", logLevelName))
	} else {
		var fout io.Writer
		var err error
		logFname := path.Join(Config.siteDataDir, Config.Logging.WorkingSubdir, PREFIX_MODULE+".log")

		if Config.Logging.RotateLogs {
			msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Creating rotating log: %v", logFname))
			dir := path.Join(Config.siteDataDir, Config.Logging.WorkingSubdir)
			fout, err = utils.NewRotateWriter(dir, PREFIX_MODULE+".log",
				time.Duration(ROTATE_LOG_HOURS)*time.Hour,
				time.Duration(ROTATE_LOG_RETAIN_HOURS)*time.Hour)
		} else {
			fout, err = os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		}
		if err != nil {
			msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Failed to open logfile %v: %v", logFname, err))
			return msgBeforeLog, false
		}
		logWriter = io.MultiWriter(os.Stderr, fout)
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Will be logging at %v level to stderr and %v\n", logLevelName, logFname))
	}
	// creating master logger 'log'
	log = logging.MustGetLogger("main")

	logBackend := logging.NewLogBackend(logWriter, "", 0)
	logBackendFormatter := logging.NewBackendFormatter(logBackend, logFormatter)
	masterLoggingBackend = logging.AddModuleLevel(logBackendFormatter)
	masterLoggingBackend.SetLevel(logLevel, "main")

	log.SetBackend(masterLoggingBackend)

	var msgtmp string
	if Config.Sender.DebugMultiCalls {
		multiapi.SetLog(log)
		msgtmp = "MultiAPI module: debugging/logging is ENABLED"
	} else {
		msgtmp = "MultiAPI module: debugging/logging is DISABLED"
	}
	msgBeforeLog = append(msgBeforeLog, msgtmp)
	logInitialized = true
	return msgBeforeLog, true
}

// creates child logger with the given name. It always writes to the file
// everything with is logged to this logger, will go to the master logger as well
func createChildLogger(name string, subdir string, masterBackend *logging.LeveledBackend) (*logging.Logger, error) {
	var logger *logging.Logger
	var logWriter io.Writer
	var err error
	logFname := path.Join(Config.siteDataDir, subdir, PREFIX_MODULE+"."+name+".log")
	if Config.Logging.RotateLogs {
		dir := path.Join(Config.siteDataDir, Config.Logging.WorkingSubdir)
		logWriter, err = utils.NewRotateWriter(dir, PREFIX_MODULE+"."+name+".log",
			time.Duration(ROTATE_LOG_HOURS)*time.Hour,
			time.Duration(ROTATE_LOG_RETAIN_HOURS)*time.Hour)

	} else {
		logWriter, err = os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	}

	if err != nil {
		return nil, err
	}

	logBackend := logging.NewLogBackend(logWriter, "", 0)
	logBackendFormatter := logging.NewBackendFormatter(logBackend, logFormatter)
	childBackend := logging.AddModuleLevel(logBackendFormatter)
	childBackend.SetLevel(logLevel, name) // not needed ?
	logger = logging.MustGetLogger(name)
	mlogger := logging.MultiLogger(*masterBackend, childBackend)
	mlogger.SetLevel(logLevel, name)
	logger.SetBackend(mlogger)

	logger.Infof("Created child logger '%v' -> %v, level: '%v'", name, logFname, logLevelName)
	return logger, nil
}

func getSeqParams(name string) (*senderParamsYAML, error) {
	stru, ok := Config.Sender.Sequences[name]
	if !ok {
		return &senderParamsYAML{}, errors.New(fmt.Sprintf("TransferSequence '%v' doesn't exist in config file\n", name))
	}
	// doing inheritance
	ret := stru // a copy
	ret.Enabled = Config.Sender.Globals.Enabled && ret.Enabled
	if len(ret.IOTANode) == 0 {
		ret.IOTANode = Config.Sender.Globals.IOTANode
		if len(ret.IOTANode) == 0 {
			return &ret, errors.New(fmt.Sprintf("Default IOTA node is undefined in sequence '%v'\n", name))
		}
	}
	if len(ret.IOTANodeTipsel) == 0 {
		ret.IOTANodeTipsel = Config.Sender.Globals.IOTANodeTipsel
		if len(ret.IOTANodeTipsel) == 0 {
			ret.IOTANodeTipsel = ret.IOTANode
		}
	}
	if len(ret.IOTANodePoW) == 0 {
		ret.IOTANodePoW = Config.Sender.Globals.IOTANodePoW
		if len(ret.IOTANodePoW) == 0 {
			ret.IOTANodePoW = ret.IOTANode[0] // by default using first of nodes
		}
	}
	if len(ret.TxTag) == 0 {
		ret.TxTag = Config.Sender.Globals.TxTag
	}
	if len(ret.TxTag) == 0 {
		ret.TxTag = defaultTag
	}
	if len(ret.TxTagPromote) == 0 {
		ret.TxTagPromote = Config.Sender.Globals.TxTagPromote
	}
	if len(ret.TxTagPromote) == 0 {
		ret.TxTagPromote = defaultTagPromote
	}
	if len(ret.AddressPromote) == 0 {
		ret.AddressPromote = Config.Sender.Globals.AddressPromote
	}
	if len(ret.AddressPromote) == 0 {
		ret.AddressPromote = defaultAddressPromote
	}

	var err error

	if _, err = NewTrytes(ret.Seed); err != nil {
		return &ret, fmt.Errorf("Wrong seed in sequence '%v': %v", name, err)
	}
	if _, err = NewTrytes(ret.TxTag); err != nil {
		return &ret, fmt.Errorf("Wrong tx tag in sequence '%v': %v", name, err)
	}
	if _, err = NewTrytes(ret.TxTagPromote); err != nil {
		return &ret, fmt.Errorf("Wrong tx tag promote in sequence '%v': %v", name, err)
	}
	if _, err = NewTrytes(ret.AddressPromote); err != nil {
		return &ret, fmt.Errorf("Wrong promotion address in sequence '%v': %v", name, err)
	}

	if ret.TimeoutAPI == 0 {
		ret.TimeoutAPI = Config.Sender.Globals.TimeoutAPI
	}
	if ret.TimeoutTipsel == 0 {
		ret.TimeoutTipsel = Config.Sender.Globals.TimeoutTipsel
	}
	if ret.TimeoutPoW == 0 {
		ret.TimeoutPoW = Config.Sender.Globals.TimeoutPoW
	}
	if ret.ForceReattachAfterMin == 0 {
		ret.ForceReattachAfterMin = Config.Sender.Globals.ForceReattachAfterMin
	}
	if ret.PromoteEverySec == 0 {
		ret.PromoteEverySec = Config.Sender.Globals.PromoteEverySec
	}
	if Config.Sender.Globals.PromoteDisable {
		ret.PromoteDisable = true
	}
	return &ret, nil
}

func getEnabledSeqNames() []string {
	ret := make([]string, 0)
	for name, params := range Config.Sender.Sequences {
		if params.Enabled {
			ret = append(ret, name)
		}
	}
	return ret
}

func logRuntimeStats() {
	var mem runtime.MemStats

	runtime.ReadMemStats(&mem)
	log.Debugf("------- DEBUG:RuntimeStats MB: Alloc = %v TotalAlloc = %v Sys = %v NumGC = %v NumGoroutines = %d\n",
		bToMb(mem.Alloc),
		bToMb(mem.TotalAlloc),
		bToMb(mem.Sys),
		mem.NumGC,
		runtime.NumGoroutine(),
	)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
