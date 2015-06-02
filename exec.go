package main

import (
	"os"
)

type ExecCommand struct{}

func (cmd ExecCommand) Run(args []string) error {
	scenario, err := ParseScenarioFromInput(args)
	if err != nil {
		return err
	}

	return ExecScenarioToFile(scenario, os.Stdout)
}
