package req

import (
	"bytes"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const maxRetries = 3

type AuthScheme string

const (
	AuthSchemeBasic AuthScheme = "Basic"
)

type RequestInfo struct {
	Idx        int `json:"idx"`
	Url        string
	Method     string
	Auth       string
	AuthScheme AuthScheme
	Headers    map[string]string
	Body       string
	Captures   []ResponseCapture
}

type requestExecutorPool struct {
	respCh    chan ResponseInfo
	reqCh     chan RequestInfo
	errCh     chan error
	executors []requestExecutor
}

func newRequestExecutorPool() *requestExecutorPool {
	p := &requestExecutorPool{
		respCh: make(chan ResponseInfo, 16),
		reqCh:  make(chan RequestInfo, 16),
		errCh:  make(chan error),
	}
	p.setcap(16)
	return p
}

func (p *requestExecutorPool) setcap(n int) {
	rem := n - len(p.executors)
	if rem > 0 {
		for i := 0; i < rem; i++ {
			e := requestExecutor{pool: p}
			e.run()
			p.executors = append(p.executors, e)
		}
	}
}

func (p *requestExecutorPool) execRequests(reqs []RequestInfo) ([]ResponseInfo, error) {

	for _, r := range reqs {
		p.reqCh <- r
	}

	resps := make([]ResponseInfo, 0, len(reqs))

	for range reqs {
		select {
		case err := <-p.errCh:
			return nil, err
		case resp := <-p.respCh:
			resps = append(resps, resp)
		}
	}

	return resps, nil
}

type requestExecutor struct {
	pool *requestExecutorPool
}

func (e *requestExecutor) run() {
	go func() {
		for {
			req := <-e.pool.reqCh
			resp, err := execRequest(req)
			if err != nil {
				e.pool.errCh <- err
			} else {
				e.pool.respCh <- resp
			}
		}
	}()
}

func execRequests(reqs []RequestInfo) ([]ResponseInfo, error) {
	var wg sync.WaitGroup
	resps := make([]ResponseInfo, len(reqs))

	for i, req := range reqs {
		wg.Add(1)
		go func(i int, req RequestInfo) {
			defer wg.Done()
			resp, err := execRequest(req)
			if err != nil {
				log.Printf("*** ERROR *** Unable to execute request: %v\n", err)
			}
			resps[i] = resp
		}(i, req)
	}

	// respCh := make(chan ResponseInfo)
	// resps := make([]ResponseInfo, 0, len(reqs))

	// //send requests in parallel
	// for _, req := range reqs {
	// 	wg.Add(1)
	// 	go func(req RequestInfo) {
	// 		resp, err := execRequest(req)
	// 		if err != nil {
	// 			log.Printf("*** ERROR *** Unable to execute request: %v\n", err)
	// 		}
	// 		respCh <- resp
	// 	}(req)
	// }

	// //wait for responses
	// go func() {
	// 	for i := 0; i < len(reqs); i += 1 {
	// 		resp := <-respCh
	// 		defer wg.Done()
	// 		resps = append(resps, resp)
	// 	}
	// }()

	wg.Wait()

	return resps, nil
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
