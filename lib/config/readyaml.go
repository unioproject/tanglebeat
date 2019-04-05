package config

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
)

func ReadYAML(configFilename string, outMsg []string, outStruct interface{}) ([]string, string, bool) {
	var configFilePath string
	wd, _ := os.Getwd()
	outMsg = append(outMsg, fmt.Sprintf("Current working directory is %v", wd))

	siteDataDir := os.Getenv("SITE_DATA_DIR")

	if _, err := os.Stat(configFilename); err == nil {
		configFilePath = configFilename
	} else {
		if siteDataDir != "" {
			outMsg = append(outMsg, fmt.Sprintf("SITE_DATA_DIR is %v", siteDataDir))
		}
		if siteDataDir == "" {
			outMsg = append(outMsg, fmt.Sprintf("can't find config file %v", configFilename))
			return outMsg, "", false
		}
		configFilePath = path.Join(siteDataDir, configFilename)
	}

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
