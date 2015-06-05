package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kr/pretty"
	"github.com/vincentcr/mergo"
	"io"
	"log"
	"text/template"
)

type RequestScenario struct {
	Init     Variables
	Bots     []BotInfo
	Requests []RequestTemplate
}

type BotInfo struct {
	Url    string
	ApiKey string
}

type Variables map[string]interface{}

type RequestTemplate struct {
	Url         string
	Method      string
	Auth        string
	AuthScheme  AuthScheme
	Headers     Headers
	Body        string
	Captures    []ResponseCapture
	Count       int
	Concurrency int
	Debug       bool
	StartIdx    int
}

var requestTemplateDefaults RequestTemplate

type ResponseCaptureSource string

const (
	ResponseCaptureHeader ResponseCaptureSource = "header"
	ResponseCaptureBody   ResponseCaptureSource = "body"
)

type ResponseCapture struct {
	Source     ResponseCaptureSource
	Name       string
	Expression string
}

func init() {
	requestTemplateDefaults = RequestTemplate{
		Headers: Headers{
			"accept":       "application/json",
			"content-type": "application/json",
		},
		Method:      "GET",
		AuthScheme:  AuthSchemeBasic,
		Count:       1,
		Concurrency: 1,
		Debug:       false,
	}
}

type ExecChans struct {
	Out  chan ResponseInfo
	Errs chan error
	Done chan bool
}

func ExecScenarioToFile(scenario RequestScenario, writer io.Writer) error {
	log.Printf("Executing scenario: %# v\n", pretty.Formatter(scenario))

	chans := ExecChans{
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

func printResponse(res ResponseInfo, writer io.Writer) error {
	data, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("unable to format %v to json: %v", res, err)
	}
	_, err = writer.Write(append(data, '\n'))
	return err
}

func execScenario(scenario RequestScenario, chans ExecChans) {
	if scenario.Bots == nil || len(scenario.Bots) == 0 {
		execScenarioLocally(scenario, chans)
	} else {
		execScenarioFromBots(scenario, chans)
	}
}

func execScenarioLocally(scenario RequestScenario, chans ExecChans) {
	vars := scenario.Init
	if vars == nil {
		vars = Variables{}
	}

	go func() {
		for _, tmpl := range scenario.Requests {
			if err := execRequestPlan(tmpl, &vars, chans.Out); err != nil {
				chans.Errs <- err
			}
		}

		chans.Done <- true
	}()

}

func execRequestPlan(tmpl RequestTemplate, vars *Variables, out chan ResponseInfo) error {
	if err := mergeTemplateWithDefaults(&tmpl); err != nil {
		return err
	}

	requests, err := generateRequestBatches(tmpl, *vars)
	if err != nil {
		return err
	}

	for res := range ExecRequests(requests) {
		out <- res
		*vars = mergeVariables(*vars, res.Variables)
	}

	return nil
}

func mergeTemplateWithDefaults(tmpl *RequestTemplate) error {
	if err := mergo.Merge(tmpl, requestTemplateDefaults); err != nil {
		return fmt.Errorf("Failed to merge template '%#v' with defaults: %v", tmpl, err)
	}
	return nil
}

func generateRequestBatches(tmpl RequestTemplate, vars Variables) ([][]RequestInfo, error) {
	reqs, err := generateRequests(tmpl, vars)
	if err != nil {
		return [][]RequestInfo{}, err
	}
	batches := batchify(reqs, tmpl.Concurrency)
	return batches, nil
}

func generateRequests(tmpl RequestTemplate, vars Variables) ([]RequestInfo, error) {
	totalCount := tmpl.Count * tmpl.Concurrency
	reqs := make([]RequestInfo, 0, totalCount)

	tmplJsonBytes, err := json.Marshal(tmpl)
	if err != nil {
		return nil, fmt.Errorf("unable to jsonify %v: %v", tmpl, err)
	}
	tmplJson := string(tmplJsonBytes)

	for i := 0; i < totalCount; i++ {
		idx := tmpl.StartIdx + i
		vars["idx"] = idx
		req, err := renderTemplate(tmplJson, vars)
		if err != nil {
			return nil, err
		}
		req.Idx = idx
		reqs = append(reqs, req)
	}

	return reqs, nil
}

func renderTemplate(tmplText string, vars Variables) (RequestInfo, error) {
	reqInfo := RequestInfo{}

	tmpl, err := template.New("_").Parse(tmplText)
	if err != nil {
		return reqInfo, fmt.Errorf("template parse error: '%v' => %v", tmplText, err)
	}

	buf := bytes.Buffer{}
	err = tmpl.Execute(&buf, vars)
	if err != nil {
		return reqInfo, fmt.Errorf("template render error. text: '%v'; vars '%#v' => %v", tmplText, vars, err)
	}

	err = json.Unmarshal(buf.Bytes(), &reqInfo)
	if err != nil {
		return reqInfo, fmt.Errorf("unable to parse '%v' into request object: %v", buf.String(), err)
	}

	return reqInfo, err
}

func batchify(reqs []RequestInfo, batchSize int) [][]RequestInfo {
	totalCount := len(reqs)
	batchCount := int(0.5 + float64(totalCount)/float64(batchSize))

	batches := make([][]RequestInfo, 0, batchCount)
	for i := 0; i < batchCount; i++ {
		lo := i * batchSize
		hi := min((i+1)*batchSize, totalCount)
		if hi-lo <= 0 {
			panic(fmt.Sprintf("batchify error! totalCount: %v; batchSize: %v; batchCount: %v; i: %v; lo: %v; hi: %v",
				totalCount, batchSize, batchCount, i, lo, hi))
		}
		batch := reqs[lo:hi]
		batches = append(batches, batch)
	}

	return batches
}

func min(x, y int) int {
	if x < y {
		return x
	} else {
		return y
	}
}

func max(x, y int) int {
	if x > y {
		return x
	} else {
		return y
	}
}

func mergeVariables(varsList ...Variables) Variables {

	merged := Variables{}

	for idx := range varsList {
		ridx := len(varsList) - 1 - idx
		vars := varsList[ridx]
		for key, val := range vars {
			merged[key] = val
		}
	}
	return merged
}
