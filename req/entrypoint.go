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
	Done chan struct{}
}

type scenarioGenerator interface {
	hasNext() bool
	next() ([]ResponseInfo, error)
}

func Execute(scen RequestScenario, writer io.Writer) error {

	setOptions(scen.Options)

	explainScenario(scen)

	gen, err := newScenarioGenerator(scen)
	if err != nil {
		return err
	}

	for {
		if !gen.hasNext() {
			break
		}
		resps, err := gen.next()
		if err != nil {
			return err
		}

		for _, resp := range resps {
			if err = printResponse(resp, writer); err != nil {
				return err
			}
		}
	}

	return nil
}

func newScenarioGenerator(scen RequestScenario) (scenarioGenerator, error) {
	if len(scen.Bots) == 0 {
		return newLocalScenarioGenerator(scen)
	} else {
		return newDistributedScenarioGenerator(scen)
	}
}

func explainScenario(scen RequestScenario) error {
	data, err := json.MarshalIndent(scen, "", "  ")

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
