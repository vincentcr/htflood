package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

var arrayIndexRe *regexp.Regexp

func init() {
	arrayIndexRe = regexp.MustCompile("\\[(\\d+)\\]")
}

func ParseResponse(res *http.Response, captures []ResponseCapture) (int, Variables, error) {
	vars := Variables{}
	saveBody := shouldSaveBody(captures)
	length, body, err := readContent(res, saveBody)
	if err != nil {
		return 0, vars, err
	}

	var parsedBody interface{} = nil
	if saveBody {
		err := parseBody(res, body, &parsedBody)
		if err != nil {
			return 0, vars, err
		}
	}

	for _, capture := range captures {
		name := capture.Name
		if capture.Source == ResponseCaptureHeader {
			val := res.Header.Get(name)
			vars[name] = val
		} else if capture.Source == ResponseCaptureBody {
			val, err := traverseObject(parsedBody, capture.Expression)
			if err != nil {
				return 0, vars, err
			} else {
				vars[name] = fmt.Sprintf("%v", val)
			}
		}
	}

	return length, vars, nil
}

func shouldSaveBody(captures []ResponseCapture) bool {
	for _, capture := range captures {
		if capture.Source == ResponseCaptureBody {
			return true
		}
	}
	return false
}

func readContent(res *http.Response, saveBody bool) (int, []byte, error) {
	reader := bufio.NewReader(res.Body)
	var body []byte
	var length int
	var err error

	if saveBody {
		body, err = ioutil.ReadAll(reader)
		length = len(body)
	} else {
		var len64 int64
		len64, err = io.Copy(ioutil.Discard, reader)
		length = int(len64)
	}

	return length, body, err
}

func parseBody(res *http.Response, body []byte, data interface{}) error {
	contentType := res.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("unable to parse content type '%s': %#v", contentType, err)
	}

	switch mediaType {
	case "application/json":
		err = json.Unmarshal(body, data)
	default:
		err = fmt.Errorf("unsupported media type")
	}

	if err != nil {
		return fmt.Errorf("Unable to parse '%v' body:\n%v\nError:%v", mediaType, string(body), err)
	} else {
		return nil
	}
}

func traverseObject(object interface{}, path string) (interface{}, error) {
	hierarchy := strings.Split(path, ".")

	var err error

	for _, accessor := range hierarchy {
		object, err = getChild(object, accessor)
		if err != nil {
			return nil, err
		}
	}

	return object, nil
}

func getChild(parent interface{}, accessor string) (interface{}, error) {
	arrayIndexRe := regexp.MustCompile("\\[\\-?(\\d+)\\]")
	idxMatch := arrayIndexRe.FindStringSubmatch(accessor)
	var err error = nil
	if len(idxMatch) == 0 {
		return getChildByKey(parent, accessor)
	} else {
		var index int
		index, err = strconv.Atoi(idxMatch[1])
		if err != nil {
			return nil, err
		} else {
			return getChildByIndex(parent, index)
		}
	}
}

func getChildByIndex(parent interface{}, index int) (interface{}, error) {
	asArray := (parent).([]interface{})
	length := len(asArray)
	if index < 0 {
		index = length - index
	}

	if index >= length {
		return nil, fmt.Errorf("Index %v out of range in %v", index, parent)
	}

	child := asArray[index]

	return child, nil
}

func getChildByKey(parent interface{}, key string) (interface{}, error) {
	asMap := (parent).(map[string]interface{})
	child := asMap[key]
	if child == nil {
		return nil, fmt.Errorf("Key %v not found in %v", key, parent)
	}
	// fmt.Printf("getChildByKey %v of %v => %v\n", key, parent, child)

	return child, nil
}
