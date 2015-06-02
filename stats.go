package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type StatsCommand struct{}

func (cmd StatsCommand) Run(args []string) error {

	acc := Accumulator{}
	acc.Responses = make([]ResponseInfo, 0)

	for res := range readResponses() {
		accumulate(&acc, res)
	}

	stats := finalize(&acc)
	print(stats)

	return nil
}

type Stat struct {
	Average float64
	StdDev  float64
	Q95     float64
	Q5      float64
	Total   float64
}

const Precision = 4

type Stats struct {
	Elapsed     Stat
	Transfer    Stat
	Count       int
	StatusCodes map[string]int
}

type Accumulator struct {
	Stats     Stats
	Responses []ResponseInfo
}

type statAccessor func(res ResponseInfo) float64

func readResponses() chan ResponseInfo {
	out := make(chan ResponseInfo)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)

		for scanner.Scan() {
			line := scanner.Text()
			res, err := parse(line)
			if err != nil {
				fatal(fmt.Errorf("Error parsing line '%v': %v", line, err))
			} else {
				out <- res
			}
		}

		if err := scanner.Err(); err != nil {
			fatal(fmt.Errorf("Error reading stdin: %v", err))
		}

		close(out)
	}()

	return out
}

func parse(line string) (ResponseInfo, error) {
	var res ResponseInfo
	err := json.Unmarshal([]byte(line), &res)
	return res, err
}

func accumulate(acc *Accumulator, res ResponseInfo) {
	acc.Responses = append(acc.Responses, res)
	accumulateStat(&acc.Stats.Elapsed, res.Elapsed)
	accumulateStat(&acc.Stats.Transfer, float64(res.Length))
}

func accumulateStat(stat *Stat, value float64) {
	stat.Total += value
}

func finalize(acc *Accumulator) Stats {
	stats := acc.Stats
	stats.Count = len(acc.Responses)

	if stats.Count == 0 {
		fatal(fmt.Errorf("empty data"))
	}

	finalizeStat(&stats.Elapsed, acc.Responses, func(res ResponseInfo) float64 {
		return res.Elapsed
	})
	finalizeStat(&stats.Transfer, acc.Responses, func(res ResponseInfo) float64 {
		return float64(res.Length)
	})
	stats.StatusCodes = calcStatusCodes(acc)

	return stats
}

func calcStatusCodes(acc *Accumulator) map[string]int {
	statusCodes := map[string]int{}

	for _, res := range acc.Responses {
		statusCode := fmt.Sprintf("%d", res.StatusCode)
		count, ok := statusCodes[statusCode]
		if !ok {
			count = 0
		}
		statusCodes[statusCode] = count + 1
	}
	return statusCodes
}

func finalizeStat(stat *Stat, responses []ResponseInfo, accessor statAccessor) {
	stat.Average = stat.Total / float64(len(responses))
	values := sortedValues(responses, accessor)
	for _, value := range values {
		stat.StdDev = math.Abs(stat.Average - value)
	}
	stat.Total = round(stat.Total, Precision)
	stat.Average = round(stat.Average, Precision)
	stat.StdDev = round(math.Sqrt(stat.StdDev), Precision)
	stat.Q5 = round(percentile(5, values), Precision)
	stat.Q95 = round(percentile(95, values), Precision)
}

func sortedValues(responses []ResponseInfo, accessor statAccessor) []float64 {
	values := make([]float64, 0, len(responses))

	for _, res := range responses {
		value := accessor(res)
		values = append(values, value)
	}

	sort.Float64s(values)
	return values
}

func round(value float64, precision int) float64 {
	var mult = math.Pow10(precision)
	return float64(int(value*mult+0.5)) / mult
}

func percentile(p float64, values []float64) float64 {
	var result float64

	n := len(values)
	rank := (p / 100.0) * float64(n)
	idx := int(rank)
	val := values[idx]
	if idx == n-1 {
		result = val
	} else {
		nextVal := values[idx+1]
		weight := rank - float64(idx)
		result = val + (nextVal-val)*weight
	}

	return result
}

func print(stats Stats) {
	bytes, err := json.Marshal(stats)
	if err != nil {
		fatal(fmt.Errorf("Unable to format stats '%#v' to json: %v", stats, err))
	} else {
		os.Stdout.Write(bytes)
	}
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%v\n", err)
	os.Exit(1)
}
