package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
)

const (
	DefaultPort   = 3210
	VersionString = "httpow 1.0"
	ApiKeyHeader  = "API-KEY"
)

type BotCommand struct{}

func (botCmd BotCommand) Run(args []string) error {
	cmd := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	apiKey := cmd.String("api-key", "", "the api key (required)")
	port := cmd.Uint("port", DefaultPort, fmt.Sprintf("port to bind to (default: %v)", DefaultPort))
	tlsCert := cmd.String("tls-cert", "", "the TLS cert .pem file")
	tlsKey := cmd.String("tls-key", "", "the TLS cert .pem file")
	cmd.Parse(args)

	return startBot(*apiKey, *port, *tlsCert, *tlsKey)
}

func startBot(apiKey string, port uint, tlsCert string, tlsKey string) error {
	log.Printf("Starting with apiKey=%v, port=%v, tlsCert=%v, tlsKey=%v", apiKey, port, tlsCert, tlsKey)

	if apiKey == "" {
		return fmt.Errorf("api key is required")
	}

	if port == 0 || port > 65535 {
		return fmt.Errorf("invalid port %v", port)
	}

	setupRoutes(apiKey)

	listenAddr := fmt.Sprintf(":%v", port)

	var err error
	if tlsKey != "" {
		err = http.ListenAndServeTLS(listenAddr, tlsCert, tlsKey, nil)
	} else {
		err = http.ListenAndServe(listenAddr, nil)
	}

	if err != nil {
		return fmt.Errorf("Failed to create server at address %v: %v", listenAddr, err)
	}

	log.Println("Server listening at ", listenAddr)
	return nil
}

func setupRoutes(apiKey string) {

	http.HandleFunc("/", func(res http.ResponseWriter, req *http.Request) {
		if !authorize(apiKey, req) {
			handleUnauthorized(res, req)
		} else if req.Method == "GET" {
			handleGetVersion(res, req)
		} else if req.Method == "POST" {
			handleScenario(res, req)
		} else {
			handleNotFound(res, req)
		}
	})
}

func handleUnauthorized(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusUnauthorized)
	res.Write([]byte("Unauthorized"))
}

func authorize(apiKey string, req *http.Request) bool {
	reqKey := req.Header.Get(ApiKeyHeader)
	return reqKey == apiKey
}

func handleGetVersion(res http.ResponseWriter, req *http.Request) {
	res.Write([]byte(VersionString))
}

func handleScenario(res http.ResponseWriter, req *http.Request) {
	//prevent concurrent execution because it would mess up measurements
	if !tryAcquireExecLock() {
		handleServiceUnavailable(res, req)
		return
	}
	defer releaseExecLock()

	res.Header().Set("Content-Type", "application/json-row")

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		handleInternalError("Unable to read request body", err, res)
		return
	}

	var scenario RequestScenario
	err = ParseScenarioFromJson(body, &scenario)
	if err != nil {
		handleInternalError("Unable to parse body as request scenario", err, res)
		return
	}

	err = ExecScenarioToFile(scenario, res)
	if err != nil {
		handleMidstreamInternalError("Failed to execute scenario", err, res)
	}
}

var (
	execFlag = false
	execLock = &sync.Mutex{}
)

func tryAcquireExecLock() bool {
	execLock.Lock()
	defer execLock.Unlock()
	success := !execFlag
	if success {
		execFlag = true
	}
	return success
}

func releaseExecLock() {
	execLock.Lock()
	defer execLock.Unlock()
	if !execFlag {
		panic("BUG: tried releasing execFlag without owning it")
	}
	execFlag = false
}

func handleServiceUnavailable(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusServiceUnavailable)
	res.Write([]byte("Service Unavailable: already serving request"))
}

func handleInternalError(msg string, err error, res http.ResponseWriter) {
	res.WriteHeader(http.StatusInternalServerError)
	log.Printf("%v: %v", msg, err)
	res.Write([]byte(msg))
}

func handleMidstreamInternalError(msg string, err error, res http.ResponseWriter) {
	//encode the error message as json in the repsonse stream
	fullMsg := fmt.Sprintf("%v: %v", msg, err)
	log.Println(fullMsg)
	errObj := map[string]string{"fatalError": fullMsg}
	data, jsonErr := json.Marshal(errObj)
	if jsonErr != nil {
		log.Fatalf("#### ERROR ####\nFailed to encode '%#v' to json: %v\n####\n", errObj, jsonErr)
	} else {
		res.Write(data)
	}

}

func handleNotFound(res http.ResponseWriter, req *http.Request) {
	res.WriteHeader(http.StatusNotFound)
	res.Write([]byte("Not Found"))
}
