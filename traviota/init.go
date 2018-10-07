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

const PREFIX_MODULE = "traviota"

var (
	log                  *logging.Logger
	masterLoggingBackend logging.LeveledBackend
	logInitialized       = false
)

type ConfigStructYAML struct {
	SiteDataDir string
	Sender      SenderYAML      `yaml:"sender"`
	Publisher   PublisherParams `yaml:"publisher"`
}

type SenderYAML struct {
	LogDir           string                  `yaml:"logDir"`
	LogConsoleOnly   bool                    `yaml:"logConsoleOnly"`
	LogFormat        string                  `yaml:"logFormat"`
	Pprof            bool                    `yaml:"pprof"`
	MemStats         bool                    `yaml:"memStats"`
	MemStatsInterval int                     `yaml:"memStatsInterval"`
	Globals          SenderParams            `yaml:"globals"`
	Sequences        map[string]SenderParams `yaml:"sequences"`
}

type SenderParams struct {
	Disabled              bool     `yaml:"disabled"`
	IOTANode              []string `yaml:"iotaNode"`
	IOTANodeGTTA          []string `yaml:"iotaNodeGTTA"`
	IOTANodeATT           []string `yaml:"iotaNodeATT"`
	TimeoutAPI            int      `yaml:"apiTimeout"`
	TimeoutGTTA           int      `yaml:"gttaTimeout"`
	TimeoutATT            int      `yaml:"attTimeout"`
	Nodebug               bool     `yaml:"nodebug"`
	Seed                  string   `yaml:"seed"`
	Index0                int      `yaml:"index0"`
	TxTag                 string   `yaml:"txTag"`
	TxTagPromote          string   `yaml:"txTagPromote"`
	ForceReattachAfterMin int      `yaml:"forceReattachAfterMin"`
	PromoteNoChain        bool     `yaml:"promoteNoChain"`
	PromoteEverySec       int      `yaml:"promoteEverySec"`
}

type PublisherParams struct {
	Disabled bool `yaml:"disabled"`
	OutPort  int  `yaml:"outPort"`
}

//  create config structure with default values
//  other default values are nil values
var Config = ConfigStructYAML{
	SiteDataDir: ".\\",
	Sender: SenderYAML{
		Globals: SenderParams{
			IOTANode: []string{"https://field.deviota.com:443"},
		},
	},
}

var msgBeforeLog = []string{"----- TangleBeat project. Starting Traviota module"}

func beforeLog() {
	for _, msg := range msgBeforeLog {
		if logInitialized {
			log.Info(msg)
		} else {
			log.Error(msg)
		}
	}
}

func ReadConfig(configFilename string) {
	defer beforeLog()

	currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(fmt.Sprintf("Can't get current current dir. Error: %v", err))
	}
	msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Current directory is %v\n", currentDir))

	Config.SiteDataDir = os.Getenv("SITE_DATA_DIR")
	if Config.SiteDataDir == "" {
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Environment variable SITE_DATA_DIR is undefined. Taking current directory: %v\n", currentDir))
		Config.SiteDataDir = currentDir
	} else {
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("SITE_DATA_DIR = %v\n", Config.SiteDataDir))
	}
	configFilePath := path.Join(Config.SiteDataDir, configFilename)
	msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Reading config values from %v\n", configFilePath))

	yamlFile, err := os.Open(configFilePath)
	if err != nil {
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Failed: %v.\nUsing default config values: %+v\n", err, &Config))
		return
	}
	defer yamlFile.Close()

	yamlbytes, _ := ioutil.ReadAll(yamlFile)

	err = yaml.Unmarshal(yamlbytes, &Config)
	if err != nil {
		panic(fmt.Sprintf("Failed to unmarshal config file. Error: %v\n", err))
	}
	ConfigLogging()

	if Config.Sender.MemStats {
		sl := lib.Max(5, Config.Sender.MemStatsInterval)
		go func() {
			for {
				LogMemStats()
				time.Sleep(time.Duration(sl) * time.Second)
			}
		}()
		log.Infof("Will be logging MemStats every %v sec", sl)
	}

	msgBeforeLog = append(msgBeforeLog, "Traviota initialized successfully")
}

//	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,

func getFormatter() logging.Formatter {
	if len(Config.Sender.LogFormat) == 0 {
		return logging.MustStringFormatter(
			`%{time:2006-01-02 15:04:05.000} [%{shortfunc}] %{level:.4s} %{message}`,
		)
	} else {
		return logging.MustStringFormatter(Config.Sender.LogFormat)
	}
}

func ConfigLogging() {
	var logWriter io.Writer
	var level logging.Level
	var levelName string

	if Config.Sender.Globals.Nodebug {
		level = logging.INFO
		levelName = "INFO"
	} else {
		level = logging.DEBUG
		levelName = "DEBUG"
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
	loggerName := "main"
	log = logging.MustGetLogger(loggerName)

	logBackend := logging.NewLogBackend(logWriter, "", 0)
	formatter := getFormatter()
	logBackendFormatter := logging.NewBackendFormatter(logBackend, formatter)
	masterLoggingBackend = logging.AddModuleLevel(logBackendFormatter)
	masterLoggingBackend.SetLevel(level, loggerName)

	log.SetBackend(masterLoggingBackend)
	logInitialized = true
}

// creates child logger with the given name. It always writes to the file
// everything with is logged to this logger, will go to the master logger as well
func createChildLogger(name string, masterBackend *logging.LeveledBackend, formatter *logging.Formatter) (*logging.Logger, error) {
	var logger *logging.Logger

	logFname := path.Join(Config.SiteDataDir, Config.Sender.LogDir, PREFIX_MODULE+"."+name+".log")
	fout, err := os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		return nil, err
	} else {
		logWriter := io.Writer(fout)

		logBackend := logging.NewLogBackend(logWriter, "", 0)
		logBackendFormatter := logging.NewBackendFormatter(logBackend, *formatter)
		seqLoggingBackend := logging.AddModuleLevel(logBackendFormatter)
		if Config.Sender.Globals.Nodebug {
			masterLoggingBackend.SetLevel(logging.INFO, name)
		} else {
			masterLoggingBackend.SetLevel(logging.DEBUG, name)
		}
		logger = logging.MustGetLogger(name)
		logger.SetBackend(logging.MultiLogger(*masterBackend, seqLoggingBackend))
	}
	logger.Infof("Created child logger '%v' -> %v", name, logFname)
	return logger, nil
}

func GetSeqParams(name string) (SenderParams, error) {
	stru, ok := Config.Sender.Sequences[name]
	if !ok {
		return SenderParams{}, errors.New(fmt.Sprintf("Sequence '%v' doesn't exist\n", name))
	}
	// doing inheritance
	ret := stru // a copy
	ret.Disabled = Config.Sender.Globals.Disabled || ret.Disabled
	if len(ret.IOTANode) == 0 {
		ret.IOTANode = Config.Sender.Globals.IOTANode
		if len(ret.IOTANode) == 0 {
			return ret, errors.New(fmt.Sprintf("Default IOTA node is undefined in sequence '%v'\n", name))
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
	if Config.Sender.Globals.Nodebug {
		ret.Nodebug = true
	}
	if len(ret.TxTag) == 0 {
		ret.TxTag = Config.Sender.Globals.TxTag
	}
	if len(ret.TxTagPromote) == 0 {
		ret.TxTagPromote = Config.Sender.Globals.TxTagPromote
	}
	if _, err := giota.ToTrytes(ret.Seed); err != nil || len(ret.Seed) != 81 {
		return ret, errors.New(fmt.Sprintf("Wrong seed in sequence '%v'. Must be exactly 81 long trytes string\n", name))
	}
	if _, err := giota.ToTrytes(ret.TxTag); err != nil || len(ret.TxTag) > 27 {
		return ret, errors.New(fmt.Sprintf("Wrong tx tag in sequence '%v'. Must be no more than 27 long trytes string\n", name))
	}
	if _, err := giota.ToTrytes(ret.TxTagPromote); err != nil || len(ret.TxTagPromote) > 27 {
		return ret, errors.New(fmt.Sprintf("Wrong tx tag promote in sequence '%v'. Must be no more than 27 long trytes string\n", name))
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
	return ret, nil
}

func GetEnabledSeqNames() []string {
	ret := make([]string, 0)
	for name, params := range Config.Sender.Sequences {
		if !params.Disabled {
			ret = append(ret, name)
		}
	}
	return ret
}

func LogMemStats() {
	var mem runtime.MemStats

	runtime.ReadMemStats(&mem)
	log.Debugf("--- DEBUG:MemStats: Alloc = %v MB  TotalAlloc = %v MB Sys = %v MB  NumGC = %v\n",
		bToMb(mem.Alloc),
		bToMb(mem.TotalAlloc),
		bToMb(mem.Sys),
		mem.NumGC,
	)
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
