package main

import (
	"log"
	"os"
	"runtime"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	command, args := parseMainCommand()

	err := command.Run(args)
	if err != nil {
		log.Fatalf("%v\n", err)
		os.Exit(1)
	}
}
