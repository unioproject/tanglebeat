package main

import (
	"fmt"
	"github.com/lunfardo314/tanglebeat/lib"
	"github.com/op/go-logging"
	"gopkg.in/yaml.v2"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"
)

const PREFIX_MODULE = "tb_metrics"

var (
	log            *logging.Logger
	logInitialized bool = false
)

type ConfigStructYAML struct {
	SiteDataDir        string
	LogDir             string `yaml:"logDir"`
	LogConsoleOnly     bool   `yaml:"logConsoleOnly"`
	LogFormat          string `yaml:"logFormat"`
	Debug              bool   `yaml:"debug"`
	MemStats           bool   `yaml:"memStats"`
	MemStatsInterval   int    `yaml:"memStatsInterval"`
	DbFile             string `yaml:"dbfile"`
	SenderURI          string `yaml:"senderURI"`
	ListenAndServePort int    `yaml:"listenAndServePort"`
}

//  create config structure with default values
//  other default values are nil values
var Config = ConfigStructYAML{
	SiteDataDir:        ".\\",
	LogConsoleOnly:     true,
	LogFormat:          "%{time:2006-01-02 15:04:05.000} [%{shortfunc}] %{level:.4s} %{message}",
	DbFile:             "tb_metrics.db",
	SenderURI:          "tcp://localhost:3000",
	ListenAndServePort: 8080,
}

var msgBeforeLog = []string{"----- Starting Tanglebeat module"}

func flushMsgBeforeLog() {
	for _, msg := range msgBeforeLog {
		if logInitialized {
			log.Info(msg)
		} else {
			fmt.Println(msg)
		}
	}
}

func readConfig(configFilename string) {

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
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Failed init logging: %v.\nExit", err))
		flushMsgBeforeLog()
		os.Exit(1)
	}
	defer yamlFile.Close()

	yamlbytes, _ := ioutil.ReadAll(yamlFile)

	err = yaml.Unmarshal(yamlbytes, &Config)
	if err != nil {
		panic(fmt.Sprintf("Failed to unmarshal config file. Error: %v", err))
	}
	configLogging()
	flushMsgBeforeLog()
	if !logInitialized {
		os.Exit(1)
	}
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

//	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
// TODO rotating log

func configLogging() {
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
	if Config.LogConsoleOnly {
		logWriter = os.Stderr
		msgBeforeLog = append(msgBeforeLog, fmt.Sprintf("Will be logging at %v level to stderr only", levelName))
	} else {
		logFname := path.Join(Config.SiteDataDir, Config.LogDir, PREFIX_MODULE+".log")
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
	formatter := logging.MustStringFormatter(Config.LogFormat)
	logBackendFormatter := logging.NewBackendFormatter(logBackend, formatter)
	loggingBackend := logging.AddModuleLevel(logBackendFormatter)
	loggingBackend.SetLevel(level, "main")

	log.SetBackend(loggingBackend)
	logInitialized = true
}

func logMemStats() {
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
