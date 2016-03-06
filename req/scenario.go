package req

import "time"

type RequestScenario struct {
	Init      Variables
	Bots      []BotInfo
	Requests  []RequestTemplate
	Options   Options
	WorkerIdx int
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
	Count       int
	Concurrency int
	MaxDuration time.Duration
	MaxReqSec   float64
	Randomize   bool
}

var requestTemplateDefaults = RequestTemplate{
	Headers: map[string]string{
		"accept":       "application/json",
		"content-type": "application/json",
	},
	Method:      "GET",
	AuthScheme:  AuthSchemeBasic,
	Count:       1,
	Concurrency: 1,
}

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
