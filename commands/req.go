package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vincentcr/htflood/req"
)

var reqCommand = &cobra.Command{
	Use:   "req [METHOD] url [HEADER:VAL...] [NAME=VAL...]",
	Short: "execute requests",
	Long: `req execute the requests from either the parameters in the command-line,
				or a scenario json in standard input.`,
	Run: checkedRun(run),
}

var reqOptions struct {
	count       uint
	concurrency uint
	auth        string
	authScheme  string
	debug       bool
	botList     string
	botFile     string
	botApiKey   string
}

type ArgType int

const (
	ArgTypeNone ArgType = iota
	ArgTypeHeader
	ArgTypeBody
	// ArgTypeFile
)

var argPatterns struct {
	method *regexp.Regexp
	url    *regexp.Regexp
	param  map[ArgType]*regexp.Regexp
}

func init() {
	reqCommand.Flags().UintVar(&reqOptions.count, "count", 1, "count")
	reqCommand.Flags().UintVar(&reqOptions.concurrency, "concurrency", 1, "count")
	reqCommand.Flags().StringVar(&reqOptions.auth, "auth", "", "auth credentials (username:password)")
	reqCommand.Flags().StringVar(&reqOptions.authScheme, "auth-scheme", string(req.AuthSchemeBasic), "the auth scheme to use (default: basic)")
	reqCommand.Flags().BoolVar(&reqOptions.debug, "debug", false, "enables debug output")
	reqCommand.Flags().StringVar(&reqOptions.botList, "bots", "", "bot list, comma-separated")
	reqCommand.Flags().StringVar(&reqOptions.botFile, "bots-file", "", "bot list (json) file")
	reqCommand.Flags().StringVar(&reqOptions.botApiKey, "bots-api-key", "", "bots api key")

	argPatterns.method = regexp.MustCompile("^[A-Z]+$")
	argPatterns.url = regexp.MustCompile("^https?://+")
	argPatterns.param = map[ArgType]*regexp.Regexp{
		ArgTypeHeader: regexp.MustCompile("^(\\w+)?:([^=].*)"),
		ArgTypeBody:   regexp.MustCompile("^(\\w+)?(\\:?)=(\\@?)(.+)"),
	}
}

func run(cmd *cobra.Command, args []string) error {
	scenario, err := parseScenarioFromInput(args)
	if err != nil {
		return err
	}

	return req.Execute(scenario, os.Stdout)
}

func parseScenarioFromInput(args []string) (req.RequestScenario, error) {
	var scenario *req.RequestScenario = nil

	bytes, err := readStdin()
	if err != nil {
		return req.RequestScenario{}, err
	}

	if len(bytes) > 0 {
		scenario = &req.RequestScenario{}
		err = parseScenarioFromJson(bytes, scenario)
		if err != nil {
			return req.RequestScenario{}, err
		}
	}

	err = parseScenarioFromCommandLine(args, &scenario)
	if err != nil {
		return req.RequestScenario{}, err
	}

	return *scenario, nil
}

func readStdin() ([]byte, error) {
	fstat, err := os.Stdin.Stat()
	if err != nil {
		return []byte{}, err
	}
	size := fstat.Size()
	mode := fstat.Mode()
	if size > 0 || mode&os.ModeNamedPipe != 0 {
		return ioutil.ReadAll(os.Stdin)
	} else {
		return []byte{}, nil
	}
}

func parseScenarioFromJson(bytes []byte, scenario *req.RequestScenario) error {
	err := json.Unmarshal(bytes, scenario)
	if err != nil {
		return fmt.Errorf("Failed to parse data to scenario: %v", err)
	}
	return nil
}

func parseScenarioFromCommandLine(args []string, scenario **req.RequestScenario) error {
	bots, err := parseBots(reqOptions.botFile, reqOptions.botList, reqOptions.botApiKey)
	if err != nil {
		return err
	}

	if nil == *scenario {

		tmpl := req.RequestTemplate{
			Count:       reqOptions.count,
			Concurrency: reqOptions.concurrency,
			Auth:        reqOptions.auth,
			AuthScheme:  req.AuthScheme(reqOptions.authScheme),
			Debug:       reqOptions.debug,
		}

		err = parseRemainingArgs(args, &tmpl)
		if err != nil {
			return err
		}

		*scenario = &req.RequestScenario{
			Init:     req.Variables{},
			Bots:     bots,
			Requests: []req.RequestTemplate{tmpl},
		}

	} else if len(bots) > 0 {
		(*scenario).Bots = bots
	}

	return err
}

func parseBots(botFile string, botList string, apiKey string) ([]req.BotInfo, error) {
	var botUrls []string
	var err error

	if botList != "" {
		botUrls, err = parseBotsString(botList)
	} else if botFile != "" {
		botUrls, err = parseBotsFile(botFile)
	}

	if err != nil || botUrls == nil {
		return nil, err
	}

	bots := make([]req.BotInfo, 0, len(botUrls))
	for _, url := range botUrls {
		bot := req.BotInfo{Url: url, ApiKey: apiKey}
		bots = append(bots, bot)
	}

	return bots, nil
}

func parseBotsString(str string) ([]string, error) {
	return strings.Split(str, ","), nil
}

func parseBotsFile(filename string) ([]string, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("Failed to read bots file %v: %v", filename, err)
	}
	var bots []string
	err = json.Unmarshal(data, &bots)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse bots file %v as json: %v", filename, err)
	}

	return bots, nil
}

func parseRemainingArgs(args []string, tmpl *req.RequestTemplate) error {
	if len(args) == 0 {
		invalidUsage("No url")
	}

	var err error
	tmpl.Method, args = parseMethod(args)
	tmpl.Url, args = parseUrl(args)
	tmpl.Headers, tmpl.Body, err = parseHeadersAndBody(args)
	return err
}

func parseMethod(args []string) (string, []string) {
	if argPatterns.method.FindString(args[0]) != "" {
		return args[0], args[1:]
	} else {
		return "GET", args
	}
}

func parseUrl(args []string) (string, []string) {
	url := args[0]
	if argPatterns.url.FindString(url) != "" {
		return url, args[1:]
	} else {
		invalidUsage("Invalid url: %v", url)
		return "", args
	}
}

func parseHeadersAndBody(args []string) (map[string]string, string, error) {
	headers := map[string]string{}
	bodyMap := map[string]interface{}{}

	for _, arg := range args {
		argType, vals := splitKeyValueArg(arg)
		fmt.Println(arg, argType, vals)
		switch argType {

		case ArgTypeHeader:
			key, val := parseHeaderKeyVal(vals)
			headers[key] = val

		case ArgTypeBody:
			key, val := parseBodyKeyVal(vals)
			bodyMap[key] = val
		}
	}

	var body string
	var err error
	if len(bodyMap) > 0 {
		bodyBytes, err := json.Marshal(bodyMap)
		if err != nil {
			return nil, "", err
		}

		body = string(bodyBytes)
	} else {
		body = ""
	}

	return headers, body, err
}

func splitKeyValueArg(arg string) (ArgType, []string) {
	for name, re := range argPatterns.param {
		matches := re.FindStringSubmatch(arg)
		if len(matches) > 0 {
			return name, matches[1:]
		}
	}
	invalidUsage("Unable to parse argument: %v", arg)
	return ArgTypeNone, []string{}
}

func parseHeaderKeyVal(vals []string) (string, string) {
	return vals[0], vals[1]
}

func parseBodyKeyVal(vals []string) (string, interface{}) {
	key := vals[0]
	jsonFlag := vals[1]
	fileFlag := vals[2]
	strVal := vals[3]

	var val interface{} = strVal

	if fileFlag != "" {
		bytes, err := ioutil.ReadFile(strVal)
		if err != nil {
			invalidUsage("unable to read file %v: %v", strVal, err)
		}
		val = string(bytes)
	}
	if jsonFlag != "" {
		var parsed interface{}
		err := json.Unmarshal([]byte(strVal), &parsed)
		if err != nil {
			invalidUsage("unable to parse '%v' as json: %v", strVal, err)
		}
		val = parsed
	}

	return key, val
}

func invalidUsage(format string, args ...interface{}) {
	if format != "" {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", msg)
	}
	fmt.Fprintf(os.Stderr, "Please use --help for usage information\n")
	os.Exit(1)
}
