package main

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"io"
)

type ConfigStruct struct {
	Version      string `yaml:"version"`
	SiteDataDir  string
	ModuleName   string `yaml:"moduleName"`
	IOTANode     string `yaml:"iotaNode"`
	IOTANodeGTTA string `yaml:"iotaNodeGTTA"`
	IOTANodeATT  string `yaml:"iotaNodeATT"`
	Debug        bool   `yaml:"debug"`
	Pprof        bool   `yaml:"pprof"`
	Sequences    map[string]struct {
		Seed         string `yaml:"seed"`
		Index0       int    `yaml:"index0"`
		IOTANode     string `yaml:"iotaNode"`
		IOTANodeGTTA string `yaml:"iotaNodeGTTA"`
		IOTANodeATT  string `yaml:"iotaNodeATT"`
	} `yaml:"sequences"`
}

var Config = ConfigStruct{
	Version:      "ver 0.0",
	ModuleName:   "traviota",
	IOTANode:     "dummy.com:314",
	IOTANodeGTTA: "",
	IOTANodeATT:  "",
	Debug:        true,
	Pprof:        false,
}

func ReadConfig(configFilename string) {
	currentDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		panic(fmt.Sprintf("Can't get current current dir. Error: %v", err))
	}
	fmt.Printf("---- Current directory is %v\n", currentDir)

	Config.SiteDataDir = os.Getenv("SITE_DATA_DIR")
	if Config.SiteDataDir == "" {
		fmt.Printf("---- Environment variable SITE_DATA_DIR is undefined. Taking current directory: %v\n", currentDir)
		Config.SiteDataDir = currentDir
	} else {
		fmt.Printf("---- SITE_DATA_DIR = %v\n", Config.SiteDataDir)
	}
	configFilePath := path.Join(Config.SiteDataDir, configFilename)
	fmt.Printf("---- Reading config values from %v\n", configFilePath)
	yamlFile, err := os.Open(configFilePath)
	if err != nil {
		fmt.Printf("---- Failed: %v.\nUsing default config values: %+v\n", err, &Config)
		return
	}
	defer yamlFile.Close()

	yamlbytes, _ := ioutil.ReadAll(yamlFile)

	err = yaml.Unmarshal(yamlbytes, &Config)
	if err != nil {
		panic(fmt.Sprintf("Failed to unmarshal config file. Error: %v\n", err))
	}
	fmt.Printf("Config:\n%+v\n\n", &Config)
	ConfigLogging()
}

func ConfigLogging() {
	// Log as JSON instead of the default ASCII formatter.
	//log.SetFormatter(&log.JSONFormatter{})
	formatter := log.TextFormatter{}
	formatter.TimestampFormat = "2006-02-02 15:04:06"
	formatter.FullTimestamp = true
	log.SetFormatter(&formatter)

	// Output to stdout instead of the default stderr
	// Can be any io.Writer, see below for File example
	logFname := path.Join(Config.SiteDataDir, Config.ModuleName+".log")
	fout, err := os.OpenFile(logFname, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		panic(fmt.Sprintf("Failed to open logfile %v: %v", logFname, err))
	} else {
		log.SetOutput(io.MultiWriter(os.Stdout, fout))
		log.WithFields(
			log.Fields{"logfile": logFname}).Infof("Will be logging to logfile AND stdout")
	}

	// Only log the warning severity or above.
	if Config.Debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}
