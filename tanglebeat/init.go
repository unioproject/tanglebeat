package main

// version with iota.go library version

import (
	"fmt"
	"github.com/iotaledger/iota.go/trinary"
	"github.com/lunfardo314/tanglebeat1/lib"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"time"
)

const (
	version                 = "1.0"
	PREFIX_MODULE           = "tanglebeat1"
	ROTATE_LOG_HOURS        = 12
	ROTATE_LOG_RETAIN_HOURS = 36
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
	Prometheus            prometheusYAML            `yaml:"prometheus"`
	SenderUpdateCollector senderUpdateCollectorYAML `yaml:"senderUpdateCollector"`
}

type loggingConfigYAML struct {
	Debug                  bool   `yaml:"debug"`
	WorkingSubdir          string `yaml:"workingSubdir"`
	LogConsoleOnly         bool   `yaml:"logConsoleOnly"`
	RotateLogs             bool   `yaml:"rotateLogs"`
	LogSequencesSeparately bool   `yaml:"logSequencesSeparately"`
	LogFormat              string `yaml:"logFormat"`
	LogFormatDebug         string `yaml:"logFormatDebug"`
	MemStats               bool   `yaml:"memStats"`
	MemStatsInterval       int    `yaml:"memStatsInterval"`
}

type senderYAML struct {
	Enabled   bool                        `yaml:"enabled"`
	Globals   senderParamsYAML            `yaml:"globals"`
	Sequences map[string]senderParamsYAML `yaml:"sequences"`
}

type senderParamsYAML struct {
	externalSource        bool   // tmp TODO
	Enabled               bool   `yaml:"enabled"`
	IOTANode              string `yaml:"iotaNode"`
	IOTANodeTipsel        string `yaml:"iotaNodeTipsel"`
	IOTANodePoW           string `yaml:"iotaNodePOW"`
	TimeoutAPI            int    `yaml:"apiTimeout"`
	TimeoutTipsel         int    `yaml:"tipselTimeout"`
	TimeoutPoW            int    `yaml:"powTimeout"`
	Seed                  string `yaml:"seed"`
	Index0                uint64 `yaml:"index0"`
	TxTag                 string `yaml:"txTag"`
	TxTagPromote          string `yaml:"txTagPromote"`
	ForceReattachAfterMin int    `yaml:"forceReattachAfterMin"`
	PromoteChain          bool   `yaml:"promoteChain"`
	PromoteEverySec       int    `yaml:"promoteEverySec"`
	SeqRestartAfterErr    int    `yaml:"seqRestartAfterErr"`
}

type senderUpdateCollectorYAML struct {
	Publish bool                           `yaml:"publish"`
	OutPort int                            `yaml:"outPort"`
	Sources map[string]collectorSourceYAML `yaml:"sources"`
}

type collectorSourceYAML struct {
	Enabled bool   `yaml:"enabled"`
	Target  string `yaml:"target"`
}

type prometheusYAML struct {
	Enabled              bool           `yaml:"enabled"`
	ScrapeTargetPort     int            `yaml:"scrapeTargetPort"`
	ZmqMetrics           zmqMetricsYAML `yaml:"zmqMetrics"`
	SenderMetricsEnabled bool           `yaml:"senderMetricsEnabled"`
}

type zmqMetricsYAML struct {
	Enabled bool   `yaml:"enabled"`
	ZMQUri  string `yaml:"zmqUri"`
}

// main config structure
var Config = ConfigStructYAML{}

var msgBeforeLog = []string{"*********** Starting TangleBeat ver. " + version}

func (params *senderParamsYAML) GetUID() string {
	seedT, err := trinary.NewTrytes(params.Seed)
	if err != nil {
		panic("can't generate UID")
	}
	hash, err := lib.KerlTrytes(seedT)
	if err != nil {
		panic("can't generate UID")
	}
	ret := string(hash)
	return ret[len(ret)-UID_LEN:]
}

func flushMsgBeforeLog() {
	for _, msg := range msgBeforeLog {
		if logInitialized {
			log.Info(msg)
		} else {
			fmt.Println(msg)
		}
	}
}

func masterConfig(configFilename string) {
	currentDir, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("Can't get current current dir. Error: %v", err))
	}
	msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Current directory is %v", currentDir))

	Config.siteDataDir = os.Getenv("SITE_DATA_DIR")
	if Config.siteDataDir == "" {
		Config.siteDataDir = currentDir
	} else {
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("SITE_DATA_DIR = %v", Config.siteDataDir))
	}
	configFilePath := path.Join(Config.siteDataDir, configFilename)

	msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Reading config values from %v", configFilePath))

	yamlFile, err := os.Open(configFilePath)
	if err != nil {
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Failed init %v:\nExit", err))
		flushMsgBeforeLog()
		os.Exit(1)
	}
	defer yamlFile.Close()

	yamlbytes, _ := ioutil.ReadAll(yamlFile)

	err = yaml.Unmarshal(yamlbytes, &Config)
	if err != nil {
		fmt.Printf("Error while reading config file %v: %v\n", yamlFile, err)
		os.Exit(1)
	}
	configMasterLogging()
	flushMsgBeforeLog()
	configDebugging()
}

func configDebugging() {
	if Config.Logging.Debug && Config.Logging.MemStats {
		sl := lib.Max(5, Config.Logging.MemStatsInterval)
		go func() {
			for {
				logMemStats()
				time.Sleep(time.Duration(sl) * time.Second)
			}
		}()
		log.Infof("Will be logging MemStats every %v sec", sl)
	}
}

// TODO rotating log

func configMasterLogging() {
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
			fout, err = lib.NewRotateWriter(dir, PREFIX_MODULE+".log",
				time.Duration(ROTATE_LOG_HOURS)*time.Hour,
				time.Duration(ROTATE_LOG_RETAIN_HOURS)*time.Hour)
		} else {
			fout, err = os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		}
		if err != nil {
			panic(fmt.Sprintf("Failed to open logfile %v: %v", logFname, err))
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
	logInitialized = true
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
		logWriter, err = lib.NewRotateWriter(dir, PREFIX_MODULE+"."+name+".log",
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
			ret.IOTANodePoW = ret.IOTANode
		}
	}
	if len(ret.TxTag) == 0 {
		ret.TxTag = Config.Sender.Globals.TxTag
	}
	if len(ret.TxTagPromote) == 0 {
		ret.TxTagPromote = Config.Sender.Globals.TxTagPromote
	}
	if _, err := trinary.NewTrytes(ret.Seed); err != nil || len(ret.Seed) != 81 {
		return &ret, errors.New(fmt.Sprintf("Wrong seed in sequence '%v'. Must be exactly 81 long trytes string\n", name))
	}
	if _, err := trinary.NewTrytes(ret.TxTag); err != nil || len(ret.TxTag) > 27 {
		return &ret, errors.New(fmt.Sprintf("Wrong tx tag in sequence '%v'. Must be no more than 27 long trytes string\n", name))
	}
	if _, err := trinary.NewTrytes(ret.TxTagPromote); err != nil || len(ret.TxTagPromote) > 27 {
		return &ret, errors.New(fmt.Sprintf("Wrong tx tag promote in sequence '%v'. Must be no more than 27 long trytes string\n", name))
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
	if ret.SeqRestartAfterErr == 0 {
		ret.SeqRestartAfterErr = Config.Sender.Globals.SeqRestartAfterErr
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

func logMemStats() {
	var mem runtime.MemStats

	runtime.ReadMemStats(&mem)
	log.Debugf("-------------- DEBUG:MemStats: Alloc = %v MB  TotalAlloc = %v MB Sys = %v MB  NumGC = %v\n",
		bToMb(mem.Alloc),
		bToMb(mem.TotalAlloc),
		bToMb(mem.Sys),
		mem.NumGC,
	)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
