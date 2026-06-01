// Package results defines the shared run and scenario result types and a
// parallel scenario executor.
package results

import (
	"time"

	"github.com/IBM/simrun/internal/matchers"
	"github.com/IBM/simrun/internal/runner"
)

type ScenarioRunResult struct {
	Name                    string                           `json:"name"`
	Success                 bool                             `json:"isSuccess"`
	ErrorMessage            string                           `json:"errorMessage"`
	DurationSeconds         float64                          `json:"durationSeconds"`
	MatchingDurationSeconds float64                          `json:"matchingDurationSeconds"`
	TimeExecuted            time.Time                        `json:"timeExecuted"`
	ExecutorName            string                           `json:"executorName"`
	ExecutorType            string                           `json:"executorType"`
	ExecutionId             string                           `json:"executionId"`
	SimulationID            string                           `json:"simulationId,omitempty"`
	Assertions              []matchers.AlertGeneratedMatcher `json:"matchers,omitempty"`
	FailedAssertions        []matchers.AlertGeneratedMatcher `json:"-"`
	Indicators              *runner.Indicators               `json:"indicators,omitempty"`
	Metadata                *runner.Metadata                 `json:"metadata,omitempty"`
	CollectedLogPath        string                           `json:"collectedLogPath,omitempty"`
	CollectedDocCount       int                              `json:"collectedDocCount,omitempty"`
	DiscoveredAlerts        []runner.DiscoveredAlert         `json:"discoveredAlerts,omitempty"`
	ExploreMode             bool                             `json:"exploreMode,omitempty"`
}

type SimrunRunResult struct {
	RunId            string              `json:"runId"`
	StartTime        time.Time           `json:"startTime"`
	EndTime          time.Time           `json:"endTime"`
	TotalScenarios   int                 `json:"totalScenarios"`
	SuccessScenarios int                 `json:"successScenarios"`
	FailedScenarios  int                 `json:"failedScenarios"`
	Scenarios        []ScenarioRunResult `json:"scenarios"`
}
