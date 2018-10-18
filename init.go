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
	"path/filepath"
	"runtime"
	"time"
)

const PREFIX_MODULE = "tanglebeat"

var (
	log                  *logging.Logger
	logPub               *logging.Logger
	masterLoggingBackend logging.LeveledBackend
	logInitialized       = false
)

type ConfigStructYAML struct {
	SiteDataDir      string
	Sender           SenderYAML           `yaml:"sender"`
	Publisher        PublisherParams      `yaml:"publisher"`
	MetricsUpdater   MetricsUpdaterParams `yaml:"metricsUpdater"`
	Debug            bool                 `yaml:"debug"`
	MemStats         bool                 `yaml:"memStats"`
	MemStatsInterval int                  `yaml:"memStatsInterval"`
}

type SenderYAML struct {
	Disabled       bool                    `yaml:"disabled"`
	LogDir         string                  `yaml:"logDir"`
	LogConsoleOnly bool                    `yaml:"logConsoleOnly"`
	LogFormat      string                  `yaml:"logFormat"`
	Globals        SenderParams            `yaml:"globals"`
	Sequences      map[string]SenderParams `yaml:"sequences"`
}

type SenderParams struct {
	externalSource        bool
	Disabled              bool     `yaml:"disabled"`
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

type PublisherParams struct {
	Disabled       bool   `yaml:"disabled"`
	LogDir         string `yaml:"logDir"`
	LogConsoleOnly bool   `yaml:"logConsoleOnly"`
	LogFormat      string `yaml:"logFormat"`
	OutPort        int    `yaml:"outPort"`
}

type MetricsUpdaterParams struct {
	Disabled             bool `yaml:"disabled"`
	PrometheusTargetPort int  `yaml:"prometheusTargetPort"`
}

//  create config structure with default values
//  other default values are nil values
//	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,

var Config = ConfigStructYAML{
	SiteDataDir: ".\\",
	Sender: SenderYAML{
		LogConsoleOnly: true,
		LogFormat:      "%{time:2006-01-02 15:04:05.000} [%{shortfunc}] %{level:.4s} %{message}",
		Globals: SenderParams{
			IOTANode: []string{"https://field.deviota.com:443"},
		},
	},
	Publisher: PublisherParams{
		LogConsoleOnly: true,
		LogFormat:      "%{time:2006-01-02 15:04:05.000} [%{shortfunc}] %{level:.4s} %{message}",
		OutPort:        3000,
	},
	MetricsUpdater: MetricsUpdaterParams{
		Disabled:             false,
		PrometheusTargetPort: 8080,
	},
}

var msgBeforeLog = []string{"----- TangleBeat project. Starting 'tb_sender' module"}

func (params *SenderParams) GetUID() string {
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
	currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(fmt.Sprintf("Can't get current current dir. Error: %v", err))
	}
	msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Current directory is %v", currentDir))

	Config.SiteDataDir = os.Getenv("SITE_DATA_DIR")
	if Config.SiteDataDir == "" {
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Environment variable SITE_DATA_DIR is undefined. Taking current directory: %v", currentDir))
		Config.SiteDataDir = currentDir
	} else {
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("SITE_DATA_DIR = %v", Config.SiteDataDir))
	}
	configFilePath := path.Join(Config.SiteDataDir, configFilename)
	msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Reading config values from %v", configFilePath))

	yamlFile, err := os.Open(configFilePath)
	if err != nil {
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Failed init logging %v:\nExit", err))
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
	if Config.MemStats {
		sl := lib.Max(5, Config.MemStatsInterval)
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
	var level logging.Level
	var levelName string

	if Config.Debug {
		level = logging.DEBUG
		levelName = "DEBUG"
	} else {
		level = logging.INFO
		levelName = "INFO"
	}

	// opening log file if necessary
	if Config.Sender.LogConsoleOnly {
		logWriter = os.Stderr
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Will be logging at %v level to stderr only", levelName))
	} else {
		logFname := path.Join(Config.SiteDataDir, Config.Sender.LogDir, PREFIX_MODULE+".log")
		fout, err := os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
		if err != nil {
			panic(fmt.Sprintf("Failed to open logfile %v: %v", logFname, err))
		}
		logWriter = io.MultiWriter(os.Stderr, fout)
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Will be logging at %v level to stderr and %v\n", levelName, logFname))
	}
	// creating master logger 'log'
	log = logging.MustGetLogger("main")

	logBackend := logging.NewLogBackend(logWriter, "", 0)
	formatter := logging.MustStringFormatter(Config.Sender.LogFormat)
	logBackendFormatter := logging.NewBackendFormatter(logBackend, formatter)
	masterLoggingBackend = logging.AddModuleLevel(logBackendFormatter)
	masterLoggingBackend.SetLevel(level, "main")

	log.SetBackend(masterLoggingBackend)
	logInitialized = true
}

// creates child logger with the given name. It always writes to the file
// everything with is logged to this logger, will go to the master logger as well
func createChildLogger(name string, dir string, masterBackend *logging.LeveledBackend,
	formatter *logging.Formatter, level logging.Level) (*logging.Logger, error) {
	var logger *logging.Logger

	logFname := path.Join(dir, PREFIX_MODULE+"."+name+".log")
	fout, err := os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	} else {
		logWriter := io.Writer(fout)

		logBackend := logging.NewLogBackend(logWriter, "", 0)
		logBackendFormatter := logging.NewBackendFormatter(logBackend, *formatter)
		childBackend := logging.AddModuleLevel(logBackendFormatter)
		childBackend.SetLevel(level, name) // not needed
		logger = logging.MustGetLogger(name)
		mlogger := logging.MultiLogger(*masterBackend, childBackend)
		mlogger.SetLevel(level, name)
		logger.SetBackend(mlogger)
	}

	ln := lib.Iff(level == logging.INFO, "INFO", "DEBUG")
	logger.Infof("Created child logger '%v' -> %v, level: '%v'", name, logFname, ln)
	return logger, nil
}

func getSeqParams(name string) (*SenderParams, error) {
	stru, ok := Config.Sender.Sequences[name]
	if !ok {
		return &SenderParams{}, errors.New(fmt.Sprintf("Sequence '%v' doesn't exist\n", name))
	}
	// doing inheritance
	ret := stru // a copy
	ret.Disabled = Config.Sender.Globals.Disabled || ret.Disabled
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
	// not inherited
	if ret.PromoteEverySec == 0 {
		ret.PromoteEverySec = Config.Sender.Globals.PromoteEverySec
	}
	// other remaining are not inherited or doesn't make sense on sequence level
	return &ret, nil
}

func getEnabledSeqNames() []string {
	ret := make([]string, 0)
	for name, params := range Config.Sender.Sequences {
		if !params.Disabled {
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
