package req

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
)

type execChans struct {
	Out  chan ResponseInfo
	Errs chan error
	Done chan bool
}

type scenarioExecutor interface {
	execute(scen RequestScenario, chans execChans)
}

func Execute(scen RequestScenario, writer io.Writer) error {

	setOptions(scen.Options)

	explainScenario(scen)

	chans := execChans{
		Out:  make(chan ResponseInfo),
		Errs: make(chan error),
		Done: make(chan bool),
	}
	execScenario(scen, chans)

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

func execScenario(scen RequestScenario, chans execChans) {
	if len(scen.Bots) == 0 {
		execScenarioLocally(scen, chans)
	} else {
		execScenarioDistributed(scen, chans)
	}
}

func explainScenario(scen RequestScenario) error {
	var data []byte
	var err error

	if options.Pretty {
		data, err = json.MarshalIndent(scen, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("unable to format %v to json: %v", scen, err)
	}

	log.Printf("Executing scenario:\n%v\n==================\n\n", string(data))

	return nil
}

func printResponse(res ResponseInfo, writer io.Writer) error {
	var data []byte
	var err error

	if options.Pretty {
		data, err = json.MarshalIndent(res, "", "  ")
	} else {
		data, err = json.Marshal(res)
	}
	if err != nil {
		return fmt.Errorf("unable to format %v to json: %v", res, err)
	}
	_, err = writer.Write(append(data, '\n'))
	return err
}
