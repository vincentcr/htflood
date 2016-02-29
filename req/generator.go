import (
	"log"
	"time"
)

func requestGenerator(scen RequestScenario, chans execChans) chan RequestInfo {
	vars := scenario.Init
	if vars == nil {
		vars = Variables{}
	}

	go func() {
		for _, tmpl := range scenario.Requests {
			requests, err := generateRequestBatches(tmpl, *vars)
			if err != nil {
				return err
			}

			if err := execRequestPlan(tmpl, &vars, chans.Out); err != nil {
				chans.Errs <- err
			}
		}

		chans.Done <- true
	}()
}

func baseGenerator() {
    for _, tmpl := range scenario.Requests {
      requests, err := generateRequestBatches(tmpl, *vars)
      if err != nil {
        return err
      }

      if err := execRequestPlan(tmpl, &vars, chans.Out); err != nil {
        chans.Errs <- err
      }
    }

    chans.Done <- true

}

func parallel(concurrency int, generator chan RequestInfo) chan RequestInfo {
	out := make(chan RequestInfo)
	go func() {
    for req := range generator {
      out <- req
    }

		close(out)
	}()
	return out
}

func deadline(deadline time.Duration, generator chan RequestInfo) chan RequestInfo {
	out := make(chan RequestInfo)

	start := time.Now()

	go func() {
		for req := range generator {
			out <- req
			elapsed := time.Since(start)
			if elapsed >= deadline {
				log.Println("[reached deadline]")
				break
			}
		}
		close(out)
	}()
	return out

}

// throttles request generator to maxRate
func throttle(maxReqSec float32, generator chan RequestInfo) chan RequestInfo {
	out := make(chan RequestInfo)

	// we'll try to maintain an average of 1s / maxReqSec
	target := time.Second / time.Duration(maxReqSec)
	tot := time.Duration(0)
	avg := time.Duration(0)
	count := 0

	go func() {
		last := time.Now()
		for req := range generator {
			out <- req
			elapsed := time.Since(last)
			count += 1
			tot += elapsed
			avg = tot / time.Duration(count)
			if avg < target {
				time.Sleep(target - avg)
			}
			last = time.Now()
		}
		close(out)
	}()
	return out
}
