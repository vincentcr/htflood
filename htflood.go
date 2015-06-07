package main

import (
	"github.com/vincentcr/htflood/commands"
	"runtime"
)

func main() {
	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	commands.Execute()
}
