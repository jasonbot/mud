package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	mud "mud/internal"
)

var configFile = "./config.json"

type serverconfig struct {
	Listen string `json:""`
}

func loadConfig(config *serverconfig) {
	data, err := ioutil.ReadFile(configFile)

	if err == nil {
		err = json.Unmarshal(data, config)
	}

	if err != nil {
		log.Printf("Error parsing %s: %v", configFile, err)
	}
}

func main() {
	log.Println("Starting")
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}

	if _, err := os.Stat(configFile); err != nil {
		executablePath, err := filepath.Abs(filepath.Dir(executable))
		if err != nil {
			panic(err)
		}

		log.Printf("Going to folder %v...", executablePath)

		os.Chdir(executablePath)
	}

	var config serverconfig

	loadConfig(&config)

	mud.LoadResources()
	mud.ServeSSH(config.Listen)
}
