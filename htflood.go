package main

import (
	"runtime"

	"github.com/pkg/profile"
	"github.com/vincentcr/htflood/commands"
)

func main() {
	defer profile.Start(profile.MemProfile).Stop()

	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)
	commands.Execute()
}
