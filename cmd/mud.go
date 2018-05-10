package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/jasonbot/mud"
)

func main() {
	log.Println("Starting")
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}

	if _, err := os.Stat("./terrain.json"); err != nil {
		executablePath, err := filepath.Abs(filepath.Dir(executable))
		if err != nil {
			panic(err)
		}

		log.Printf("Going to folder %v...", executablePath)

		os.Chdir(executablePath)
	}

	mud.LoadResources()
	mud.Serve()
}
