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
	callback func(result *ScenarioRunResult),
) []ScenarioRunResult {
	numWorkers := parallelism
	if numScenarios := len(scenarios); numScenarios < numWorkers {
		numWorkers = numScenarios
	}

	scenarioChan := make(chan *runner.Scenario, len(scenarios))
	resultsChan := make(chan *ScenarioRunResult, len(scenarios))

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

	var allResults []ScenarioRunResult
	for result := range resultsChan {
		callback(result)
		allResults = append(allResults, *result)
	}

	return allResults
}

func runSingleScenario(scenarios <-chan *runner.Scenario, results chan<- *ScenarioRunResult) {
	for scenario := range scenarios {
		testRunner := runner.NewRunner()
		testRunner.Scenarios = append(testRunner.Scenarios, scenario)
		testRunner.Interval = 10 * time.Second

		start := time.Now()
		scenarioResults, runError := testRunner.Run()
		end := time.Now()

		var executorType, executorName, simulationID string
		if scenario.Detonator != nil {
			executorType = "detonator"
			executorName = scenario.Detonator.String()
			simulationID = scenario.Detonator.SimulationId()
		} else if scenario.Injector != nil {
			executorType = "injector"
			executorName = scenario.Injector.String()
		} else {
			executorType = "unknown"
			executorName = "unknown"
		}

		if len(scenarioResults) > 0 {
			results <- &ScenarioRunResult{
				Name:                    scenario.Name,
				ErrorMessage:            scenarioResults[0].Error,
				Success:                 scenarioResults[0].Success,
				DurationSeconds:         end.Sub(start).Seconds(),
				MatchingDurationSeconds: scenarioResults[0].MatchingDurationSeconds,
				TimeExecuted:            start,
				ExecutorName:            executorName,
				ExecutorType:            executorType,
				ExecutionId:             scenarioResults[0].ExecutionId,
				SimulationID:            simulationID,
				Assertions:              scenario.Assertions,
				FailedAssertions:        scenario.FailedAssertions,
				Indicators:              scenario.Indicators,
				Metadata:                scenario.Metadata,
				CollectedLogPath:        scenario.CollectedLogPath,
				CollectedDocCount:       scenario.CollectedDocCount,
				DiscoveredAlerts:        scenario.DiscoveredAlerts,
				ExploreMode:             scenario.ExploreMode,
			}
		} else {
			errorMessage := "Scenario failed to execute"
			if runError != nil {
				errorMessage = runError.Error()
			}
			results <- &ScenarioRunResult{
				Name:                    scenario.Name,
				ErrorMessage:            errorMessage,
				Success:                 false,
				DurationSeconds:         end.Sub(start).Seconds(),
				MatchingDurationSeconds: 0,
				TimeExecuted:            start,
				ExecutorName:            executorName,
				ExecutorType:            executorType,
				ExecutionId:             "",
				SimulationID:            simulationID,
				Assertions:              scenario.Assertions,
				Indicators:              scenario.Indicators,
				Metadata:                scenario.Metadata,
			}
		}
	}
}
