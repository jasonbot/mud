package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/andlabs/ui"
	"github.com/jasonbot/mud"
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
	go mud.ServeSSH(config.Listen)

	uierr := ui.Main(func() {
		box := ui.NewVerticalBox()
		box.SetPadded(true)
		box.Append(ui.NewLabel(fmt.Sprintf("Running SSH server on %v", config.Listen)), false)
		window := ui.NewWindow("MUD SSH Server", 400, 50, false)
		window.SetChild(box)
		window.OnClosing(func(*ui.Window) bool {
			ui.Quit()
			return true
		})
		window.Show()
	})
	if uierr != nil {
		panic(err)
	}
}
