package runner

import (
	"time"

	"github.com/IBM/simrun/internal/collectors"
	"github.com/IBM/simrun/internal/detonators"
	"github.com/IBM/simrun/internal/injectors"
	"github.com/IBM/simrun/internal/matchers"
)

type Scenario struct {
	Name           string
	RunID          string
	EnvVars        map[string]string // run-specific env vars (nil = use process env)
	Detonator      detonators.Detonator
	Injector       injectors.Injector
	Collector      collectors.Collector
	Timeout        time.Duration
	Matchers       []matchers.AlertGeneratedMatcher
	Indicators     *Indicators
	Metadata       *Metadata
	StatusCallback func(scenarioName, phase string)
	// IdentityCallback fires once after detonation, carrying executor identity.
	IdentityCallback func(scenarioName string, identity ScenarioIdentity)
	// ExpectationsCallback fires when an expectation newly matches, carrying the
	// current pass/pending state of every expectation.
	ExpectationsCallback func(scenarioName string, results []ExpectationResult)
	ExploreMode          bool // when true, discover all matching alerts instead of matching specific rules
	CleanupAlerts        bool // when true in explore mode, close discovered alerts after run
}

// ScenarioResult is the single in-memory outcome of executing one scenario,
// returned by the runner and consumed by the parallel executor and the web
// layer. The runner populates everything except the wall-clock timing
// (TimeExecuted, DurationSeconds), which the executor records around the call.
// The persistence row (db.ScenarioResult) is a separate column-shaped DTO.
type ScenarioResult struct {
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
	Matchers                []matchers.AlertGeneratedMatcher `json:"expectations,omitempty"`
	UnmetExpectations       []matchers.AlertGeneratedMatcher `json:"-"`
	Indicators              *Indicators                      `json:"indicators,omitempty"`
	Metadata                *Metadata                        `json:"metadata,omitempty"`
	CollectedLogPath        string                           `json:"collectedLogPath,omitempty"`
	CollectedDocCount       int                              `json:"collectedDocCount,omitempty"`
	DiscoveredAlerts        []DiscoveredAlert                `json:"discoveredAlerts,omitempty"`
	ExploreMode             bool                             `json:"exploreMode,omitempty"`
}

// RunResult is the aggregate outcome of a whole run (one assessment execution).
type RunResult struct {
	RunId            string           `json:"runId"`
	StartTime        time.Time        `json:"startTime"`
	EndTime          time.Time        `json:"endTime"`
	TotalScenarios   int              `json:"totalScenarios"`
	SuccessScenarios int              `json:"successScenarios"`
	FailedScenarios  int              `json:"failedScenarios"`
	Scenarios        []ScenarioResult `json:"scenarios"`
}

// ScenarioIdentity is the executor identity surfaced mid-run, after detonation.
type ScenarioIdentity struct {
	ExecutorName string
	ExecutorType string
	ExecutionID  string
	SimulationID string
}

// ExpectationResult is the mid-run state of a single expectation. Passed is nil
// while the expectation is still pending (not yet matched).
type ExpectationResult struct {
	MatcherType string
	AlertName   string
	Passed      *bool
}

// DiscoveredAlert represents an alert found during explore mode.
type DiscoveredAlert struct {
	RuleName string `json:"ruleName"`
	AlertID  string `json:"alertId"`
	Severity string `json:"severity,omitempty"`
}

type Metadata struct {
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
}

type Indicators struct {
	TerraformOutput []string `json:"terraformOutput,omitempty" yaml:"terraformOutput,omitempty"`
	Static          []string `json:"static,omitempty" yaml:"static,omitempty"`
}
