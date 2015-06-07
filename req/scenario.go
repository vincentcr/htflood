package req

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	"github.com/imdario/mergo"
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

type RequestTemplate struct {
	Url         string
	Method      string
	Auth        string
	AuthScheme  AuthScheme
	Headers     map[string]string
	Body        string
	Captures    []ResponseCapture
	Count       uint
	Concurrency uint
	Debug       bool
	StartIdx    uint
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
		Headers: map[string]string{
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

func generateRequestBatches(tmpl RequestTemplate, vars Variables) ([][]RequestInfo, error) {
	if err := mergeTemplateWithDefaults(&tmpl); err != nil {
		return nil, err
	}

	reqs, err := generateRequests(tmpl, vars)
	if err != nil {
		return nil, err
	}

	batches := batchify(reqs, tmpl.Concurrency)
	return batches, nil
}

func mergeTemplateWithDefaults(tmpl *RequestTemplate) error {
	if err := mergo.Merge(tmpl, requestTemplateDefaults); err != nil {
		return fmt.Errorf("Failed to merge template '%#v' with defaults: %v", tmpl, err)
	}
	return nil
}

func generateRequests(tmpl RequestTemplate, vars Variables) ([]RequestInfo, error) {
	totalCount := tmpl.Count * tmpl.Concurrency
	reqs := make([]RequestInfo, 0, totalCount)

	tmplJsonBytes, err := json.Marshal(tmpl)
	if err != nil {
		return nil, fmt.Errorf("unable to jsonify %#v: %v", tmpl, err)
	}
	tmplJson := string(tmplJsonBytes)

	for i := uint(0); i < totalCount; i++ {
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

func batchify(reqs []RequestInfo, batchSize uint) [][]RequestInfo {
	totalCount := uint(len(reqs))
	batchCount := uint(0.5 + float64(totalCount)/float64(batchSize))

	batches := make([][]RequestInfo, 0, batchCount)
	for i := uint(0); i < batchCount; i++ {
		lo := i * batchSize
		hi := umin((i+1)*batchSize, totalCount)
		if hi-lo <= 0 {
			panic(fmt.Sprintf("batchify error! totalCount: %v; batchSize: %v; batchCount: %v; i: %v; lo: %v; hi: %v",
				totalCount, batchSize, batchCount, i, lo, hi))
		}
		batch := reqs[lo:hi]
		batches = append(batches, batch)
	}

	return batches
}

func umin(x, y uint) uint {
	if x < y {
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
