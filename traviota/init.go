package main

import (
	"fmt"
	//log "github.com/sirupsen/logrus"
	"github.com/lunfardo314/giota"
	"github.com/op/go-logging"
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
)

var log = logging.MustGetLogger(Config.Sender.Prefix)
var logInitialized = false

type ConfigStructYAML struct {
	SiteDataDir string
	Sender      SenderYAML `yaml:"sender"`
	Publisher   string     `yaml:"publisher"`
}

type SenderYAML struct {
	Prefix    string                  `yaml:"prefix"`
	LogDir    string                  `yaml:"logDir"`
	Globals   SenderParams            `yaml:"globals"`
	Sequences map[string]SenderParams `yaml:"sequences"`
}

type SenderParams struct {
	Disabled     bool     `yaml:"disabled"`
	IOTANode     []string `yaml:"iotaNode"`
	IOTANodeGTTA []string `yaml:"iotaNodeGTTA"`
	IOTANodeATT  []string `yaml:"iotaNodeATT"`
	Nodebug      bool     `yaml:"nodebug"`
	Pprof        bool     `yaml:"pprof"`
	MemStats     bool     `yaml:"memStats"`
	Seed         string   `yaml:"seed"`
	Index0       int      `yaml:"index0"`
}

//  create config structure with default values
//  other default values are nil values
var Config = ConfigStructYAML{
	SiteDataDir: ".\\",
	Sender: SenderYAML{
		Prefix: "traviota",
		Globals: SenderParams{
			IOTANode: []string{"https://field.deviota.com:443"},
		},
	},
}

var msgBeforeLog = []string{"TangleBeat project. Starting Traviota module"}

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
	msgBeforeLog = append(msgBeforeLog, "Traviota initialized successfully")
}

//var logFormat = logging.MustStringFormatter(
//	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
//)

var logFormat = logging.MustStringFormatter(
	`%{time:2006-01-02 15:04:05.000} [%{shortfunc}] %{level:.4s} %{message}`,
)

func ConfigLogging() {

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logFname := path.Join(Config.SiteDataDir, Config.Sender.LogDir, Config.Sender.Prefix+".log")
	fout, err := os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(fmt.Sprintf("Failed to open logfile %v: %v", logFname, err))
	}
	logBackend := logging.NewLogBackend(io.MultiWriter(os.Stderr, fout), "", 0)
	logBackendFormatter := logging.NewBackendFormatter(logBackend, logFormat)
	logBackendLeveled := logging.AddModuleLevel(logBackendFormatter)
	if Config.Sender.Globals.Nodebug {
		logBackendLeveled.SetLevel(logging.INFO, "")
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Will be logging at INFO level to stderr and %v\n", logFname))
	} else {
		logBackendLeveled.SetLevel(logging.DEBUG, "")
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Will be logging at DEBUG level to stderr and %v\n", logFname))
	}
	logging.SetBackend(logBackendLeveled)
	logInitialized = true
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
	if _, err := giota.ToTrytes(ret.Seed); err != nil || len(ret.Seed) != 81 {
		return ret, errors.New(fmt.Sprintf("Wrong seed in sequence '%v'. Must be exactly 81 long trytes string\n", name))
	}

	// other remaining are not inherited or doesn't make sense on sequence level
	return ret, nil
}

func GetEnabledSeqParams() (map[string]SenderParams, error) {
	ret := make(map[string]SenderParams)
	var err error
	for _, name := range GetEnabledSeqNames() {
		ret[name], err = GetSeqParams(name)
		if err != nil {
			return nil, err
		}
	}
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
