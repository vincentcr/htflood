package req

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"text/template"
	"time"

	"github.com/imdario/mergo"
)

type predicate func() bool
type requestGenerator func(newVars Variables) (RequestInfo, error)

type localScenarioGenerator struct {
	scen        RequestScenario
	gen         requestGenerator
	pool        *requestExecutorPool
	canContinue predicate
	vars        Variables
	prevResps   []ResponseInfo
	idx         int
}

func newLocalScenarioGenerator(scen RequestScenario) (*localScenarioGenerator, error) {
	g := &localScenarioGenerator{scen: scen, idx: -1, vars: scen.Init, pool: newRequestExecutorPool()}
	g.nextTemplate()
	return g, nil
}

func (g *localScenarioGenerator) hasNext() bool {
	return g.idx < len(g.scen.Requests)
}

func (g *localScenarioGenerator) next() ([]ResponseInfo, error) {
	concurrency := g.scen.Requests[g.idx].Concurrency
	reqs := make([]RequestInfo, concurrency)

	g.pool.setcap(concurrency)

	//generate requests
	for i := 0; i < concurrency; i++ {
		var vars Variables
		if g.prevResps != nil {
			vars = g.prevResps[i].Variables
		} else {
			vars = g.vars
		}
		req, err := g.gen(vars)
		if err != nil {
			return nil, err
		}
		reqs[i] = req
	}

	//execute requests
	resps, err := g.pool.execRequests(reqs)
	if err != nil {
		return nil, err
	}
	g.prevResps = resps

	//if we have reached the end of this template,
	//move to the next one
	if !g.canContinue() {
		if err = g.nextTemplate(); err != nil {
			return nil, err
		}
	}

	return resps, nil
}

func (g *localScenarioGenerator) nextTemplate() error {
	g.idx += 1
	if g.hasNext() {
		tmpl := g.scen.Requests[g.idx]
		gen, err := newRequestGenerator(g.scen.WorkerIdx, tmpl)
		if err != nil {
			return err
		}
		g.gen = gen
		g.canContinue = newLimitChecker(tmpl)
	}
	return nil
}

func newRequestGenerator(workerIdx int, tmpl RequestTemplate) (requestGenerator, error) {
	if err := mergeTemplateWithDefaults(&tmpl); err != nil {
		return nil, err
	}

	tmplJsonBytes, err := json.Marshal(tmpl)
	if err != nil {
		return nil, fmt.Errorf("unable to jsonify %#v: %v", tmpl, err)
	}
	tmplJson := string(tmplJsonBytes)

	var idx int = 0
	if tmpl.Count > 0 {
		totalCount := tmpl.Count * tmpl.Concurrency
		idx = workerIdx * totalCount
	}

	vars := Variables{}

	next := func(newVars Variables) (RequestInfo, error) {

		updateVariables(vars, newVars)

		vars["idx"] = idx
		req, err := renderTemplate(tmplJson, vars)
		if err != nil {
			return RequestInfo{}, err
		}
		req.Idx = idx
		idx += 1
		return req, err
	}

	return next, nil
}

func mergeTemplateWithDefaults(tmpl *RequestTemplate) error {
	if err := mergo.Merge(tmpl, requestTemplateDefaults); err != nil {
		return fmt.Errorf("Failed to merge template '%#v' with defaults: %v", tmpl, err)
	}
	return nil
}

func updateVariables(dst Variables, srcs ...Variables) {
	for idx := range srcs {
		ridx := len(srcs) - 1 - idx
		src := srcs[ridx]
		for key, val := range src {
			dst[key] = val
		}
	}
}

func renderTemplate(tmplText string, vars Variables) (RequestInfo, error) {
	reqInfo := RequestInfo{}

	tmpl, err := template.New("_").Parse(tmplText)
	if err != nil {
		return reqInfo, fmt.Errorf("template parse error: '%v' => %v", tmplText, err)
	}

	buf := bytes.Buffer{}
	err = tmpl.Execute(&buf, vars)
	if err != nil {
		return reqInfo, fmt.Errorf("template render error. text: '%v'; vars '%#v' => %v", tmplText, vars, err)
	}

	err = json.Unmarshal(buf.Bytes(), &reqInfo)
	if err != nil {
		return reqInfo, fmt.Errorf("unable to parse '%v' into request object: %v", buf.String(), err)
	}

	return reqInfo, err
}

func newLimitChecker(tmpl RequestTemplate) predicate {
	s := time.Now()
	c := 0

	maxCount := tmpl.Count
	maxDuration := tmpl.MaxDuration
	randomize := tmpl.Randomize
	var targetAvgElapsed time.Duration
	if tmpl.MaxReqSec > 0 {
		targetAvgElapsed = time.Millisecond * time.Duration(math.Floor(1000*tmpl.MaxReqSec))
		log.Println("tmpl.MaxReqSec", tmpl.MaxReqSec, targetAvgElapsed, math.Floor(1000*tmpl.MaxReqSec), time.Millisecond*time.Duration(math.Floor(1000.0*tmpl.MaxReqSec+0.5)))
	}

	return func() bool {
		c += 1
		e := time.Since(s)

		if maxCount > 0 && c >= maxCount {
			// log.Println("maxCount limit", c, maxCount)
			return false
		}

		if maxDuration > 0 && e >= maxDuration {
			log.Println("maxDuration limit", e, maxDuration)
			return false
		}

		if randomize {
			d := time.Millisecond * time.Duration(50+rand.Int31n(500))
			log.Println("randomize!", d)
			time.Sleep(d)
		}

		if targetAvgElapsed > 0 {
			avg := e / time.Duration(c)
			delta := targetAvgElapsed - avg
			log.Println("avg -->", avg, "target", targetAvgElapsed, "delta", delta, "tot", e, "count", c)
			if delta > 0 {
				// log.Println("throttle for", targetAvgElapsed-avg)
				time.Sleep(delta)
			}
		}
		return true
	}
}
