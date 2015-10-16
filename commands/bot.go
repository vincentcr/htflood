package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"

	"github.com/spf13/cobra"
	"github.com/vincentcr/htflood/req"
)

const (
	defaultPort   = 3210
	versionString = "htflood 1.0"
	apiKeyHeader  = "API-KEY"
)

var botCommand = &cobra.Command{
	Use:   "bot <api-key>",
	Short: "runs as a bot server",
	Long:  `bot starts an http server, waiting for remote commands and executing them`,
	Run:   checkedRun(runBot),
}

var botOptions struct {
	apiKey  string
	port    uint16
	tlsCert string
	tlsKey  string
}

func init() {
	botCommand.Flags().Uint16Var(&botOptions.port, "port", defaultPort, fmt.Sprintf("port to bind to (default: %v)", defaultPort))
	botCommand.Flags().StringVar(&botOptions.tlsCert, "tls-cert", "", "the TLS cert .pem file")
	botCommand.Flags().StringVar(&botOptions.tlsKey, "tls-key", "", "the TLS key .pem file")
}

func runBot(cmd *cobra.Command, args []string) error {

	fmt.Println("run bot", args)

	if len(args) != 1 {
		return fmt.Errorf("api key is required")
	} else {
		botOptions.apiKey = args[0]
	}

	if botOptions.port == 0 || botOptions.port > 65535 {
		return fmt.Errorf("invalid port %v", botOptions.port)
	}

	log.Printf("Starting bot with api-key='%v', port=%v, tlsCert='%v', tlsKey='%v'\n",
		botOptions.apiKey,
		botOptions.port,
		botOptions.tlsCert,
		botOptions.tlsKey,
	)

	setupRoutes(botOptions.apiKey)

	listenAddr := fmt.Sprintf(":%v", botOptions.port)

	var err error
	if botOptions.tlsKey != "" {
		err = http.ListenAndServeTLS(listenAddr, botOptions.tlsCert, botOptions.tlsKey, nil)
	} else {
		err = http.ListenAndServe(listenAddr, nil)
	}

	if err != nil {
		return fmt.Errorf("Failed to create server at address %v: %v", listenAddr, err)
	}

	return nil
}

func setupRoutes(apiKey string) {

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !authorize(apiKey, r) {
			handleUnauthorized(w, r)
		} else if r.Method == "GET" {
			handleGetVersion(w, r)
		} else if r.Method == "POST" {
			handleScenario(w, r)
		} else {
			handleNotFound(w, r)
		}
	})
}

func handleUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusUnauthorized)
	w.Write([]byte("Unauthorized"))
}

func authorize(apiKey string, r *http.Request) bool {
	reqKey := r.Header.Get(apiKeyHeader)
	return reqKey == apiKey
}

func handleGetVersion(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(versionString))
}

func handleScenario(w http.ResponseWriter, r *http.Request) {
	//prevent concurrent execution because it would mess up measurements
	if !tryAcquireExecLock() {
		handleServiceUnavailable(w, r)
		return
	}
	defer releaseExecLock()

	w.Header().Set("Content-Type", "application/json-row")

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		handleInternalError("Unable to read request body", err, w)
		return
	}

	var scenario req.RequestScenario
	err = parseScenarioFromJson(body, &scenario)
	if err != nil {
		handleInternalError("Unable to parse body as request scenario", err, w)
		return
	}

	err = req.Execute(scenario, w)
	if err != nil {
		handleMidstreamInternalError("Failed to execute scenario", err, w)
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
		panic("tried releasing execFlag without owning it")
	}
	execFlag = false
}

func handleServiceUnavailable(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusServiceUnavailable)
	w.Write([]byte("Service Unavailable: already serving request"))
}

func handleInternalError(msg string, err error, w http.ResponseWriter) {
	w.WriteHeader(http.StatusInternalServerError)
	log.Printf("%v: %v", msg, err)
	w.Write([]byte(msg))
}

func handleMidstreamInternalError(msg string, err error, w http.ResponseWriter) {
	//encode the error message as json in the repsonse stream
	fullMsg := fmt.Sprintf("%v: %v", msg, err)
	log.Println(fullMsg)
	errObj := map[string]string{"fatalError": fullMsg}
	data, jsonErr := json.Marshal(errObj)
	if jsonErr != nil {
		log.Fatalf("#### ERROR ####\nFailed to encode '%#v' to json: %v\n####\n", errObj, jsonErr)
	} else {
		w.Write(data)
	}

}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Not Found"))
}
