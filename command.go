package main

import (
	"os"
)

type Command interface {
	Run(args []string) error
}

var commands = map[string]Command{
	"exec":  ExecCommand{}, // the default
	"stats": StatsCommand{},
	"bot":   BotCommand{},
}

func parseMainCommand() (Command, []string) {
	args := os.Args[1:]
	if len(args) > 0 {
		firstArg := args[0]
		for name, command := range commands {
			if firstArg == name {
				return command, args[1:]
			}
		}
	}
	return commands["exec"], args
}
