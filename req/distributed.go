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

type distributedScenarioGenerator struct {
	scen  RequestScenario
	done  bool
	outCh chan ResponseInfo
	errCh chan error
}

func newDistributedScenarioGenerator(scen RequestScenario) (*distributedScenarioGenerator, error) {
	g := distributedScenarioGenerator{
		scen:  scen,
		outCh: make(chan ResponseInfo, 16),
		errCh: make(chan error),
	}
	g.start()
	return &g, nil
}

func (g *distributedScenarioGenerator) hasNext() bool {
	return g.done
}

func (g *distributedScenarioGenerator) next() ([]ResponseInfo, error) {
	select {
	case err := <-g.errCh:
		return nil, err
	case resp := <-g.outCh:
		return []ResponseInfo{resp}, nil
	}
}

func (g *distributedScenarioGenerator) start() {
	loadSelfSignedCertificate()

	go func() {
		var wg sync.WaitGroup

		for idx, _ := range g.scen.Bots {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				execScenarioFromBot(idx, g.scen, g.outCh, g.errCh)
			}(int(idx))
		}

		wg.Wait()
		g.done = true
	}()
}

func execScenarioFromBot(botIdx int, scen RequestScenario, outCh chan ResponseInfo, errCh chan error) {
	botScenario := makeBotScenario(botIdx, scen)
	data, err := encodeScenario(botScenario)

	if err == nil {
		bot := scen.Bots[botIdx]
		err = sendToBot(bot, data, outCh)
	}

	if err != nil {
		errCh <- err
	}
}

func makeBotScenario(botIdx int, scen RequestScenario) RequestScenario {
	scen.Bots = nil //dont send bot list to bots or we will have infinite recursion

	botReqs := make([]RequestTemplate, len(scen.Requests))
	for i, req := range scen.Requests {
		botReqs[i] = req
	}
	scen.WorkerIdx = botIdx
	scen.Requests = botReqs
	return scen
}

func encodeScenario(scen RequestScenario) ([]byte, error) {
	data, err := json.Marshal(scen)
	if err != nil {
		return nil, fmt.Errorf("failed to encode '%#v' to json: %v", scen, err)
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
