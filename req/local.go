package req

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const maxRetries = 3

type AuthScheme string

const (
	AuthSchemeBasic AuthScheme = "Basic"
)

type RequestInfo struct {
	Idx        uint `json:"idx"`
	Url        string
	Method     string
	Auth       string
	AuthScheme AuthScheme
	Headers    map[string]string
	Body       string
	Captures   []ResponseCapture
}

func execScenarioLocally(scenario RequestScenario, chans execChans) {
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
	requests, err := generateRequestBatches(tmpl, *vars)
	if err != nil {
		return err
	}

	for res := range execRequests(requests) {
		out <- res
		*vars = mergeVariables(*vars, res.Variables)
	}

	return nil
}

func execRequests(batches [][]RequestInfo) chan ResponseInfo {
	outCh := make(chan ResponseInfo)
	go func() {
		for _, batch := range batches {
			for res := range execParallelRequests(batch) {
				outCh <- res
			}
		}
		close(outCh)
	}()

	return outCh
}

func execParallelRequests(reqs []RequestInfo) chan ResponseInfo {
	out := make(chan ResponseInfo)
	in := make(chan ResponseInfo)

	for _, req := range reqs {
		go func(req RequestInfo) {
			res, err := execRequest(req)
			if err != nil {
				log.Printf("*** ERROR *** Unable to execute request: %v\n", err)
			}
			in <- res
		}(req)
	}

	go func() {
		defer close(out)

		for range reqs {
			res := <-in
			out <- res
		}
	}()

	return out
}

func execRequest(reqInfo RequestInfo) (ResponseInfo, error) {
	started := time.Now()

	reqBody := bytes.NewBufferString(reqInfo.Body)
	req, err := http.NewRequest(reqInfo.Method, reqInfo.Url, reqBody)
	if err != nil {
		return badResponse(reqInfo, fmt.Errorf("Error creating request %#v: %v", reqInfo, err))
	}

	for name, val := range reqInfo.Headers {
		req.Header.Add(name, val)
	}

	if reqInfo.Auth != "" {
		err = addAuthHeaders(req, reqInfo)
		if err != nil {
			return badResponse(reqInfo, fmt.Errorf("Error creating request %#v: %v", reqInfo, err))
		}
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		return badResponse(reqInfo, fmt.Errorf("Error executing request %#v: %v", reqInfo, err))
	}
	defer resp.Body.Close()

	connElapsed := time.Now().Sub(started)
	bodyInfo, err := parseResponseBody(resp, reqInfo.Captures)
	if err != nil {
		return badResponse(reqInfo, fmt.Errorf("Error reading response %#v of request %#v: %v", resp, reqInfo, err))
	}

	elapsed := connElapsed + bodyInfo.Elapsed

	resInfo := ResponseInfo{
		Idx:        reqInfo.Idx,
		Url:        reqInfo.Url,
		Timestamp:  started.Unix(),
		Elapsed:    elapsed.Seconds() * 1000,
		Length:     bodyInfo.Length,
		StatusCode: resp.StatusCode,
		Variables:  bodyInfo.Variables,
	}

	return resInfo, nil
}

func badResponse(reqInfo RequestInfo, err error) (ResponseInfo, error) {
	resInfo := ResponseInfo{
		Url:       reqInfo.Url,
		Timestamp: time.Now().Unix(),
		Error:     err.Error(),
	}

	return resInfo, err
}

func addAuthHeaders(req *http.Request, reqInfo RequestInfo) error {
	switch reqInfo.AuthScheme {
	case AuthSchemeBasic:
		return setBasicAuth(req, reqInfo)
	default:
		return fmt.Errorf("Unimplemented: auth scheme %v", reqInfo.AuthScheme)
	}

}

func setBasicAuth(req *http.Request, reqInfo RequestInfo) error {
	creds := strings.SplitN(reqInfo.Auth, ":", 2)
	if len(creds) != 2 {
		return fmt.Errorf("Expected <username>:<password>, got: %v", reqInfo.Auth)
	}
	req.SetBasicAuth(creds[0], creds[1])
	return nil
}
