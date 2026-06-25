package web

import (
	"encoding/json"

	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/matchers"
	"github.com/IBM/simrun/internal/runner"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// expectationDTO is the per-matcher status row persisted under
// `scenario_results.expectations`. Field names are wire-format.
type expectationDTO struct {
	MatcherType string `json:"matcherType"`
	AlertName   string `json:"alertName"`
	Passed      bool   `json:"passed"`
}

// partialExpectationDTO is the mid-run counterpart of expectationDTO: a pending
// (not-yet-matched) expectation omits `passed` so the frontend renders it muted
// rather than as a failure. The terminal write uses expectationDTO (passed always
// present) instead.
type partialExpectationDTO struct {
	MatcherType string `json:"matcherType"`
	AlertName   string `json:"alertName"`
	Passed      *bool  `json:"passed,omitempty"`
}

// buildPartialExpectationsJSON marshals the runner's mid-run expectation state
// into the same wire shape as buildScenarioResultRow, preserving pending vs passed.
func buildPartialExpectationsJSON(results []runner.ExpectationResult) ([]byte, error) {
	dtos := make([]partialExpectationDTO, 0, len(results))
	for _, r := range results {
		dtos = append(dtos, partialExpectationDTO{
			MatcherType: r.MatcherType,
			AlertName:   r.AlertName,
			Passed:      r.Passed,
		})
	}
	return json.Marshal(dtos)
}

// buildScenarioResultRow projects an in-memory scenario result into the
// `scenario_results` row shape: marshals expectations/indicators/metadata/
// discovered-alerts and copies scalar fields. Marshaling errors are logged
// and the offending JSON field is left nil so the row still persists.
func buildScenarioResultRow(runID uuid.UUID, result *runner.ScenarioResult) *db.ScenarioResult {
	// Build a set of unmet expectations for quick lookup.
	// If the scenario failed but UnmetExpectations is nil (e.g., error during
	// matching), fall back to marking all expectations as failed.
	failedSet := make(map[matchers.AlertGeneratedMatcher]struct{}, len(result.UnmetExpectations))
	for _, fa := range result.UnmetExpectations {
		failedSet[fa] = struct{}{}
	}
	hasPerMatcherResults := result.Success || result.UnmetExpectations != nil
	var expectationDTOs []expectationDTO
	for _, a := range result.Matchers {
		passed := result.Success
		if hasPerMatcherResults {
			_, failed := failedSet[a]
			passed = !failed
		}
		expectationDTOs = append(expectationDTOs, expectationDTO{
			MatcherType: a.MatcherName(),
			AlertName:   a.AlertName(),
			Passed:      passed,
		})
	}
	expectationsJSON, err := json.Marshal(expectationDTOs)
	if err != nil {
		log.WithError(err).Warn("Failed to marshal expectations")
		expectationsJSON = nil
	}
	indicatorsJSON, err := json.Marshal(result.Indicators)
	if err != nil {
		log.WithError(err).Warn("Failed to marshal indicators")
		indicatorsJSON = nil
	}
	metadataJSON, err := json.Marshal(result.Metadata)
	if err != nil {
		log.WithError(err).Warn("Failed to marshal metadata")
		metadataJSON = nil
	}

	var collectedLogPath *string
	if result.CollectedLogPath != "" {
		collectedLogPath = &result.CollectedLogPath
	}

	var discoveredAlertsJSON json.RawMessage
	if result.ExploreMode {
		discoveredAlertsJSON, err = json.Marshal(result.DiscoveredAlerts)
		if err != nil {
			log.WithError(err).Warn("Failed to marshal discovered alerts")
			discoveredAlertsJSON = nil
		}
	}

	isSuccess := result.Success
	// A scenario errored (vs. cleanly missing an expectation) when it failed
	// without producing per-expectation results: warmup/detonation failures and
	// matching-infrastructure errors leave UnmetExpectations nil.
	errored := !result.Success && len(result.UnmetExpectations) == 0
	return &db.ScenarioResult{
		RunID:             runID,
		Name:              result.Name,
		IsSuccess:         &isSuccess,
		Errored:           errored,
		ErrorMessage:      result.ErrorMessage,
		DurationSecs:      result.DurationSeconds,
		MatchingDurSecs:   result.MatchingDurationSeconds,
		TimeExecuted:      &result.TimeExecuted,
		ExecutorName:      result.ExecutorName,
		ExecutorType:      result.ExecutorType,
		ExecutionID:       result.ExecutionId,
		SimulationID:      result.SimulationID,
		Expectations:      expectationsJSON,
		Indicators:        indicatorsJSON,
		Metadata:          metadataJSON,
		CollectedLogPath:  collectedLogPath,
		CollectedDocCount: result.CollectedDocCount,
		DiscoveredAlerts:  discoveredAlertsJSON,
	}
}
