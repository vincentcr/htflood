// Distribute scenario to bots for remote execution
package req

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
)

func execScenarioDistributed(scenario RequestScenario, chans execChans) {
	go func() {
		var wg sync.WaitGroup

		for idx, _ := range scenario.Bots {
			wg.Add(1)
			go func(idx uint) {
				defer wg.Done()
				execScenarioFromBot(idx, scenario, chans)
			}(uint(idx))
		}

		wg.Wait()
		chans.Done <- true

	}()
}

func execScenarioFromBot(botIdx uint, scenario RequestScenario, chans execChans) {
	botScenario := makeBotScenario(botIdx, scenario)
	data, err := encodeScenario(botScenario)

	if err == nil {
		bot := scenario.Bots[botIdx]
		err = sendToBot(bot, data, chans.Out)
	}

	if err != nil {
		chans.Errs <- err
	}
}

func makeBotScenario(botIdx uint, scenario RequestScenario) RequestScenario {
	scenario.Bots = nil //dont send bot list to bots or we will have infinite recursion

	botReqs := make([]RequestTemplate, len(scenario.Requests))
	for i, req := range scenario.Requests {
		req.StartIdx = botIdx * req.Count

		botReqs[i] = req
	}
	scenario.Requests = botReqs
	return scenario
}

func encodeScenario(scenario RequestScenario) ([]byte, error) {
	data, err := json.Marshal(scenario)
	if err != nil {
		return nil, fmt.Errorf("failed to encode '%#v' to json: %v", scenario, err)
	}
	return data, nil
}

func sendToBot(bot BotInfo, data []byte, out chan ResponseInfo) error {
	resp, err := execBotRequest(bot, data)
	if err != nil {
		return botError("failed to exec bot request", err, bot, data)
	}

	defer resp.Body.Close()
	reader := bufio.NewReader(resp.Body)

	if resp.StatusCode != 200 {
		respBody, _ := ioutil.ReadAll(reader)
		return botError("request failed",
			fmt.Errorf("unexpected response status %v, body: %v", resp.StatusCode, string(respBody)),
			bot, data)
	} else {
		if err = readBotResponse(reader, out); err != nil {
			return botError("failed to read response body", err, bot, data)
		}
	}

	return nil
}

func execBotRequest(bot BotInfo, data []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", bot.Url, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Add("API-KEY", bot.ApiKey)
	req.Header.Add("Content-Type", "application/json")

	resp, err := transport.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("failed to exec request: %v", err)
	}

	return resp, nil
}

func readBotResponse(reader *bufio.Reader, out chan ResponseInfo) error {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		var res ResponseInfo

		if err := json.Unmarshal([]byte(line), &res); err != nil {
			return fmt.Errorf("error parsing response line '%v': %v", line, err)
		}
		out <- res
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}

func botError(msg string, err error, bot BotInfo, data []byte) error {
	return fmt.Errorf("%v:%v\nbot:%#v\nrequest body:\n%v\n", msg, err, bot, string(data))
}
