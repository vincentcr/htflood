package main

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

const MaxRetries = 3

type AuthScheme string

const (
	AuthSchemeBasic AuthScheme = "Basic"
)

type Headers map[string]string

type RequestInfo struct {
	Idx        int `json:"idx"`
	Url        string
	Method     string
	Auth       string
	AuthScheme AuthScheme
	Headers    Headers
	Body       string
	Captures   []ResponseCapture
}

type ResponseInfo struct {
	Idx        int       `json:"idx"`
	Url        string    `json:"url"`
	Timestamp  int64     `json:"timestamp"`
	Elapsed    float64   `json:"elapsed"`
	Length     int       `json:"length"`
	StatusCode int       `json:"statusCode"`
	Meta       Variables `json:"meta,omitempty"`
	Variables  Variables `json:"-"`
}

func ExecRequests(batches [][]RequestInfo) chan ResponseInfo {
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
			res := tryExecRequest(req, MaxRetries)
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

func tryExecRequest(req RequestInfo, maxRetries int) ResponseInfo {
	var resInfo ResponseInfo
	var err error

	tryIdx := 1
	stop := false
	for !stop {
		resInfo, err = execRequest(req)
		if err == nil {
			stop = true
		} else {
			stop = tryIdx >= maxRetries
			tryIdx++
		}
	}

	if err != nil {
		log.Printf("*** ERROR *** Unable to execute request: %v\n", err)
	}

	return resInfo
}

func execRequest(reqInfo RequestInfo) (ResponseInfo, error) {
	started := time.Now()

	reqBody := bytes.NewBufferString(reqInfo.Body)
	req, err := http.NewRequest(reqInfo.Method, reqInfo.Url, reqBody)
	if err != nil {
		return badResponse(reqInfo, fmt.Errorf("Error creating request %v: %v", reqInfo, err))
	}

	for name, val := range reqInfo.Headers {
		req.Header.Add(name, val)
	}

	if reqInfo.Auth != "" {
		err = addAuthHeaders(req, reqInfo)
		if err != nil {
			return badResponse(reqInfo, fmt.Errorf("Error creating request %v: %v", reqInfo, err))
		}
	}

	resp, err := transport.RoundTrip(req)
	if err != nil {
		return badResponse(reqInfo, fmt.Errorf("Error executing request %v: %v", reqInfo, err))
	}
	defer resp.Body.Close()

	bodyLength, vars, err := ParseResponse(resp, reqInfo.Captures)
	if err != nil {
		return badResponse(reqInfo, fmt.Errorf("Error reading response %v of request %v: %v", resp, reqInfo, err))
	}

	elapsed := time.Now().Sub(started)

	resInfo := ResponseInfo{
		Idx:        reqInfo.Idx,
		Url:        reqInfo.Url,
		Timestamp:  started.Unix(),
		Elapsed:    elapsed.Seconds() * 1000,
		Length:     bodyLength,
		Variables:  vars,
		StatusCode: resp.StatusCode,
		Meta:       Variables{},
	}

	return resInfo, nil
}

func badResponse(reqInfo RequestInfo, err error) (ResponseInfo, error) {
	resInfo := ResponseInfo{
		Url:       reqInfo.Url,
		Timestamp: time.Now().Unix(),
		Meta:      Variables{"error": err.Error()},
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
