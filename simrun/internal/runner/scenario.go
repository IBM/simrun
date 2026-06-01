package runner

import (
	"time"

	"github.com/IBM/simrun/simrun/internal/collectors"
	"github.com/IBM/simrun/simrun/internal/detonators"
	"github.com/IBM/simrun/simrun/internal/injectors"
	"github.com/IBM/simrun/simrun/internal/matchers"
)

type Scenario struct {
	Name           string
	RunID          string
	EnvVars        map[string]string // run-specific env vars (nil = use process env)
	Detonator      detonators.Detonator
	Injector       injectors.Injector
	Collector      collectors.Collector
	Timeout        time.Duration
	Assertions     []matchers.AlertGeneratedMatcher
	Indicators     *Indicators
	Metadata       *Metadata
	StatusCallback func(scenarioName, phase string)
	ExploreMode    bool // when true, discover all matching alerts instead of asserting specific rules
	CleanupAlerts  bool // when true in explore mode, close discovered alerts after run

	// Populated by runner after assertion matching completes
	FailedAssertions []matchers.AlertGeneratedMatcher

	// Populated by runner after explore mode completes
	DiscoveredAlerts []DiscoveredAlert

	// Populated by runner after collection completes
	CollectedLogPath  string
	CollectedDocCount int
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
