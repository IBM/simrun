package results

import (
	"sync"
	"time"

	"github.com/IBM/simrun/internal/runner"
)

// RunScenariosParallel runs scenarios in parallel with the given parallelism.
// The callback is invoked each time a scenario completes.
func RunScenariosParallel(
	scenarios []*runner.Scenario,
	parallelism int,
	callback func(result *runner.ScenarioResult),
) []runner.ScenarioResult {
	numWorkers := parallelism
	if numScenarios := len(scenarios); numScenarios < numWorkers {
		numWorkers = numScenarios
	}

	scenarioChan := make(chan *runner.Scenario, len(scenarios))
	resultsChan := make(chan *runner.ScenarioResult, len(scenarios))

	var wg sync.WaitGroup

	for worker := 0; worker < numWorkers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runSingleScenario(scenarioChan, resultsChan)
		}()
	}

	for _, scenario := range scenarios {
		if scenario.StatusCallback != nil {
			scenario.StatusCallback(scenario.Name, "queued")
		}
		scenarioChan <- scenario
	}
	close(scenarioChan)

	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	var allResults []runner.ScenarioResult
	for result := range resultsChan {
		callback(result)
		allResults = append(allResults, *result)
	}

	return allResults
}

func runSingleScenario(scenarios <-chan *runner.Scenario, results chan<- *runner.ScenarioResult) {
	for scenario := range scenarios {
		testRunner := runner.NewRunner()
		testRunner.Interval = 10 * time.Second

		start := time.Now()
		result := testRunner.Run(scenario)
		end := time.Now()

		// The runner populates everything except the wall-clock timing.
		result.TimeExecuted = start
		result.DurationSeconds = end.Sub(start).Seconds()
		results <- &result
	}
}
