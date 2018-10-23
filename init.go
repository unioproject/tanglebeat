package main

import (
	"fmt"
	"github.com/lunfardo314/giota"
	"github.com/lunfardo314/tanglebeat/lib"
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

const PREFIX_MODULE = "tanglebeat"

var (
	log                  *logging.Logger
	logLevel             logging.Level
	logLevelName         string
	masterLoggingBackend logging.LeveledBackend
	logFormatter         logging.Formatter
	logInitialized       bool
)

type ConfigStructYAML struct {
	siteDataDir string
	Logging     loggingConfigYAML `yaml:"logging"`
	Sender      senderYAML        `yaml:"sender"`
	Publisher   publisherYAML     `yaml:"publisher"`
	Prometheus  prometheusYAML    `yaml:"prometheus"`
	ZmqMetrics  zmqMetricsYAML    `yaml:"zmqMetrics"`
}

type loggingConfigYAML struct {
	Debug                  bool   `yaml:"debug"`
	WorkingSubdir          string `yaml:"workingSubdir"`
	LogConsoleOnly         bool   `yaml:"logConsoleOnly"`
	LogSequencesSeparately bool   `yaml:"logSequencesSeparately"`
	LogFormat              string `yaml:"logFormat"`
	LogFormatDebug         string `yaml:"logFormatDebug"`
	MemStats               bool   `yaml:"memStats"`
	MemStatsInterval       int    `yaml:"memStatsInterval"`
}

type senderYAML struct {
	Enabled        bool                        `yaml:"enabled"`
	Globals        senderParamsYAML            `yaml:"globals"`
	Sequences      map[string]senderParamsYAML `yaml:"sequences"`
	MetricsEnabled bool                        `yaml:"metricsEnabled"`
}

type senderParamsYAML struct {
	externalSource        bool     // tmp TODO
	Enabled               bool     `yaml:"enabled"`
	IOTANode              []string `yaml:"iotaNode"`
	IOTANodeGTTA          []string `yaml:"iotaNodeTipsel"`
	IOTANodeATT           []string `yaml:"iotaNodePOW"`
	TimeoutAPI            int      `yaml:"apiTimeout"`
	TimeoutGTTA           int      `yaml:"GTTATimeout"`
	TimeoutATT            int      `yaml:"ATTTimeout"`
	Seed                  string   `yaml:"seed"`
	Index0                int      `yaml:"index0"`
	TxTag                 string   `yaml:"txTag"`
	TxTagPromote          string   `yaml:"txTagPromote"`
	ForceReattachAfterMin int      `yaml:"forceReattachAfterMin"`
	PromoteChain          bool     `yaml:"promoteChain"`
	PromoteEverySec       int      `yaml:"promoteEverySec"`
}

type publisherYAML struct {
	Enabled         bool                           `yaml:"enabled"`
	OutPort         int                            `yaml:"outPort"`
	LocalDisabled   bool                           `yaml:"localDisabled"`
	ExternalSources map[string]publisherSourceYAML `yaml:"externalSources"`
}

type publisherSourceYAML struct {
	Enabled bool   `yaml:"enabled"`
	Target  string `yaml:"target"`
}

type prometheusYAML struct {
	Enabled          bool `yaml:"enabled"`
	ScrapeTargetPort int  `yaml:"scrapeTargetPort"`
}

type zmqMetricsYAML struct {
	Enabled bool   `yaml:"enabled"`
	ZMQUri  string `yaml:"zmqUri"`
}

// main config structure
var Config = ConfigStructYAML{}

var msgBeforeLog = []string{"----- Starting TangleBeat "}

func (params *senderParamsYAML) GetUID() string {
	seedT, err := giota.ToTrytes(params.Seed)
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
	// currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
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
		logFname := path.Join(Config.siteDataDir, Config.Logging.WorkingSubdir, PREFIX_MODULE+".log")
		fout, err := os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
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

	logFname := path.Join(Config.siteDataDir, subdir, PREFIX_MODULE+"."+name+".log")
	fout, err := os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	} else {
		logWriter := io.Writer(fout)

		logBackend := logging.NewLogBackend(logWriter, "", 0)
		logBackendFormatter := logging.NewBackendFormatter(logBackend, logFormatter)
		childBackend := logging.AddModuleLevel(logBackendFormatter)
		childBackend.SetLevel(logLevel, name) // not needed ?
		logger = logging.MustGetLogger(name)
		mlogger := logging.MultiLogger(*masterBackend, childBackend)
		mlogger.SetLevel(logLevel, name)
		logger.SetBackend(mlogger)
	}

	logger.Infof("Created child logger '%v' -> %v, level: '%v'", name, logFname, logLevelName)
	return logger, nil
}

func getSeqParams(name string) (*senderParamsYAML, error) {
	stru, ok := Config.Sender.Sequences[name]
	if !ok {
		return &senderParamsYAML{}, errors.New(fmt.Sprintf("Sequence '%v' doesn't exist in config file\n", name))
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
	if len(ret.IOTANodeGTTA) == 0 {
		ret.IOTANodeGTTA = Config.Sender.Globals.IOTANodeGTTA
		if len(ret.IOTANodeGTTA) == 0 {
			ret.IOTANodeGTTA = ret.IOTANode
		}
	}
	if len(ret.IOTANodeATT) == 0 {
		ret.IOTANodeATT = Config.Sender.Globals.IOTANodeATT
		if len(ret.IOTANodeATT) == 0 {
			ret.IOTANodeATT = ret.IOTANode
		}
	}
	if len(ret.TxTag) == 0 {
		ret.TxTag = Config.Sender.Globals.TxTag
	}
	if len(ret.TxTagPromote) == 0 {
		ret.TxTagPromote = Config.Sender.Globals.TxTagPromote
	}
	if _, err := giota.ToTrytes(ret.Seed); err != nil || len(ret.Seed) != 81 {
		return &ret, errors.New(fmt.Sprintf("Wrong seed in sequence '%v'. Must be exactly 81 long trytes string\n", name))
	}
	if _, err := giota.ToTrytes(ret.TxTag); err != nil || len(ret.TxTag) > 27 {
		return &ret, errors.New(fmt.Sprintf("Wrong tx tag in sequence '%v'. Must be no more than 27 long trytes string\n", name))
	}
	if _, err := giota.ToTrytes(ret.TxTagPromote); err != nil || len(ret.TxTagPromote) > 27 {
		return &ret, errors.New(fmt.Sprintf("Wrong tx tag promote in sequence '%v'. Must be no more than 27 long trytes string\n", name))
	}
	if ret.TimeoutAPI == 0 {
		ret.TimeoutAPI = Config.Sender.Globals.TimeoutAPI
	}
	if ret.TimeoutGTTA == 0 {
		ret.TimeoutGTTA = Config.Sender.Globals.TimeoutGTTA
	}
	if ret.TimeoutATT == 0 {
		ret.TimeoutATT = Config.Sender.Globals.TimeoutATT
	}
	if ret.ForceReattachAfterMin == 0 {
		ret.ForceReattachAfterMin = Config.Sender.Globals.ForceReattachAfterMin
	}
	if ret.PromoteEverySec == 0 {
		ret.PromoteEverySec = Config.Sender.Globals.PromoteEverySec
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
