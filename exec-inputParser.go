package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"
)

const Usage = "Usage: %s [options] [METHOD] url [HEADER:VAL...] [NAME=VAL...] \n"

type ArgType int

const (
	ArgTypeNone ArgType = iota
	ArgTypeHeader
	ArgTypeBody
	// ArgTypeFile
)

var cmd *flag.FlagSet
var methodRegex *regexp.Regexp
var urlRegex *regexp.Regexp
var argRegexes map[ArgType]*regexp.Regexp

func init() {
	methodRegex = regexp.MustCompile("^([A-Z+])$")
	urlRegex = regexp.MustCompile("^https?://+")
	argRegexes = map[ArgType]*regexp.Regexp{
		ArgTypeHeader: regexp.MustCompile("^(\\w+)?:([^=].*)"),
		ArgTypeBody:   regexp.MustCompile("^(\\w+)?(\\:?)=(\\@?)(.+)"),
	}

	cmd = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

	cmd.Usage = func() {
		fmt.Fprintf(os.Stderr, Usage, os.Args[0])
		cmd.PrintDefaults()
	}

}

func ParseScenarioFromInput(args []string) (RequestScenario, error) {
	var scenario *RequestScenario = nil

	bytes, err := readStdin()
	if err != nil {
		return RequestScenario{}, err
	}

	if len(bytes) > 0 {
		err = ParseScenarioFromJson(bytes, scenario)
		if err != nil {
			return RequestScenario{}, err
		}
	}

	err = parseScenarioFromCommandLine(args, &scenario)

	return *scenario, err
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

func ParseScenarioFromJson(bytes []byte, scenario *RequestScenario) error {
	err := json.Unmarshal(bytes, scenario)
	return err
}

func parseScenarioFromCommandLine(args []string, scenario **RequestScenario) error {

	count := cmd.Int("count", 1, "count")
	concurrency := cmd.Int("concurrency", 1, "concurrency")
	auth := cmd.String("auth", "", "auth credentials (username:password)")
	authScheme := cmd.String("auth-scheme", string(AuthSchemeBasic), "the auth scheme to use (default: basic)")
	debug := cmd.Bool("debug", false, "enables debug output")
	botList := cmd.String("bots", "", "bot list, comma-separated")
	botFile := cmd.String("bots-file", "", "bot list (json) file")
	apiKey := cmd.String("bots-api-key", "", "bots api key")

	cmd.Parse(args)

	bots, err := parseBots(*botFile, *botList, *apiKey)
	if err != nil {
		return err
	}

	if nil == *scenario {

		req := RequestTemplate{
			Count:       *count,
			Concurrency: *concurrency,
			Auth:        *auth,
			AuthScheme:  AuthScheme(*authScheme),
			Debug:       *debug,
		}

		err = parseRemainingArgs(cmd.Args(), &req)

		*scenario = &RequestScenario{
			Init:     Variables{},
			Bots:     bots,
			Requests: []RequestTemplate{req},
		}
	} else if len(bots) > 0 {
		(*scenario).Bots = bots
	}

	return err
}

func parseBots(botFile string, botList string, apiKey string) ([]BotInfo, error) {
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

	bots := make([]BotInfo, 0, len(botUrls))
	for _, url := range botUrls {
		bot := BotInfo{Url: url, ApiKey: apiKey}
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

func parseRemainingArgs(args []string, req *RequestTemplate) error {
	if len(args) == 0 {
		usageAndExit("")
	}

	var err error
	req.Method, args = parseMethod(args)
	req.Url, args = parseUrl(args)
	req.Headers, req.Body, err = parseHeadersAndBody(args)
	return err
}

func parseMethod(args []string) (string, []string) {
	if methodRegex.FindString(args[0]) != "" {
		return args[0], args[1:]
	} else {
		return "GET", args
	}
}

func parseUrl(args []string) (string, []string) {
	url := args[0]
	if urlRegex.FindString(url) != "" {
		return url, args[1:]
	} else {
		usageAndExit("Invalid url: %v", url)
		return "", args
	}
}

func parseHeadersAndBody(args []string) (Headers, string, error) {
	headers := Headers{}
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

	bodyBytes, err := json.Marshal(bodyMap)
	if err != nil {
		return Headers{}, "", err
	}

	body := string(bodyBytes)

	return headers, body, err
}

func splitKeyValueArg(arg string) (ArgType, []string) {
	for name, re := range argRegexes {
		matches := re.FindStringSubmatch(arg)
		if len(matches) > 0 {
			return name, matches[1:]
		}
	}
	usageAndExit("Unable to parse argument: %v", arg)
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
			usageAndExit("unable to read file %v: %v", strVal, err)
		}
		val = string(bytes)
	}
	if jsonFlag != "" {
		var parsed interface{}
		err := json.Unmarshal([]byte(strVal), &parsed)
		if err != nil {
			usageAndExit("unable to parse '%v' as json: %v", strVal, err)
		}
		val = parsed
	}

	return key, val
}

func usageAndExit(format string, args ...interface{}) {
	if format != "" {
		msg := fmt.Sprintf(format, args...)
		fmt.Fprintf(os.Stderr, "\nError: %v\n\n", msg)
	}
	cmd.Usage()
	os.Exit(1)
}
