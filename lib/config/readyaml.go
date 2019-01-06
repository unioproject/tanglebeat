package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
)

func ReadYAML(configFilename string, outMsg []string, outStruct interface{}) ([]string, string, bool) {
	currentDir, err := os.Getwd()
	if err != nil {
		outMsg = append(outMsg, fmt.Sprintf("Can't get current current dir. Error: %v", err))
		return outMsg, "", false
	}
	outMsg = append(outMsg, fmt.Sprintf("Current directory is %v", currentDir))

	siteDataDir := os.Getenv("SITE_DATA_DIR")
	if siteDataDir == "" {
		siteDataDir = currentDir
	} else {
		outMsg = append(outMsg, fmt.Sprintf("SITE_DATA_DIR = %v", siteDataDir))
	}
	configFilePath := path.Join(siteDataDir, configFilename)

	outMsg = append(outMsg, fmt.Sprintf("reading config values from %v", configFilePath))

	yamlFile, err := os.Open(configFilePath)
	if err != nil {
		outMsg = append(outMsg, fmt.Sprintf("Failed init %v:\nExit", err))
		return outMsg, "", false
	}
	defer yamlFile.Close()

	yamlbytes, _ := ioutil.ReadAll(yamlFile)

	err = yaml.Unmarshal(yamlbytes, outStruct)
	if err != nil {
		outMsg = append(outMsg, fmt.Sprintf("Error while reading config file %v: %v\n", yamlFile, err))
		return outMsg, "", false
	}
	return outMsg, siteDataDir, true
}
