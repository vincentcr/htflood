package req

import (
	"encoding/json"
	"fmt"
	"io"
	"log"

	"github.com/kr/pretty"
)

type execChans struct {
	Out  chan ResponseInfo
	Errs chan error
	Done chan bool
}

func Execute(scenario RequestScenario, writer io.Writer) error {
	log.Printf("Executing scenario: %# v\n", pretty.Formatter(scenario))

	chans := execChans{
		Out:  make(chan ResponseInfo),
		Errs: make(chan error),
		Done: make(chan bool),
	}

	execScenario(scenario, chans)

	for {
		select {
		case res := <-chans.Out:
			if err := printResponse(res, writer); err != nil {
				return err
			}
		case err := <-chans.Errs:
			return err
		case <-chans.Done:
			return nil
		}
	}
}

func execScenario(scenario RequestScenario, chans execChans) {
	if len(scenario.Bots) == 0 {
		execScenarioLocally(scenario, chans)
	} else {
		execScenarioDistributed(scenario, chans)
	}
}

func printResponse(res ResponseInfo, writer io.Writer) error {
	data, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("unable to format %v to json: %v", res, err)
	}
	_, err = writer.Write(append(data, '\n'))
	return err
}
