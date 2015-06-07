package req

type Variables map[string]interface{}

type ResponseInfo struct {
	Idx        uint      `json:"idx"`
	Url        string    `json:"url"`
	Timestamp  int64     `json:"timestamp"`
	Elapsed    float64   `json:"elapsed"`
	Length     int64     `json:"length"`
	StatusCode int       `json:"statusCode"`
	Error      string    `json:"error,omitempty"`
	Variables  Variables `json:"-"`
}
