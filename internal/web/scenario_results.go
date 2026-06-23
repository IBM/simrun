package web

import (
	"encoding/json"

	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/matchers"
	"github.com/IBM/simrun/internal/results"
	"github.com/IBM/simrun/internal/runner"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// assertionDTO is the per-matcher status row persisted under
// `scenario_results.assertions`. Field names are wire-format.
type assertionDTO struct {
	MatcherType string `json:"matcherType"`
	AlertName   string `json:"alertName"`
	Passed      bool   `json:"passed"`
}

// partialAssertionDTO is the mid-run counterpart of assertionDTO: a pending
// (not-yet-matched) assertion omits `passed` so the frontend renders it muted
// rather than as a failure. The terminal write uses assertionDTO (passed always
// present) instead.
type partialAssertionDTO struct {
	MatcherType string `json:"matcherType"`
	AlertName   string `json:"alertName"`
	Passed      *bool  `json:"passed,omitempty"`
}

// buildPartialAssertionsJSON marshals the runner's mid-run assertion state into
// the same wire shape as buildScenarioResultRow, preserving pending vs passed.
func buildPartialAssertionsJSON(results []runner.AssertionResult) ([]byte, error) {
	dtos := make([]partialAssertionDTO, 0, len(results))
	for _, r := range results {
		dtos = append(dtos, partialAssertionDTO{
			MatcherType: r.MatcherType,
			AlertName:   r.AlertName,
			Passed:      r.Passed,
		})
	}
	return json.Marshal(dtos)
}

// buildScenarioResultRow projects an in-memory scenario result into the
// `scenario_results` row shape: marshals assertions/indicators/metadata/
// discovered-alerts and copies scalar fields. Marshaling errors are logged
// and the offending JSON field is left nil so the row still persists.
func buildScenarioResultRow(runID uuid.UUID, result *results.ScenarioRunResult) *db.ScenarioResult {
	// Build a set of failed assertions for quick lookup.
	// If the scenario failed but FailedAssertions is nil (e.g., error during
	// assertion check), fall back to marking all assertions as failed.
	failedSet := make(map[matchers.AlertGeneratedMatcher]struct{}, len(result.FailedAssertions))
	for _, fa := range result.FailedAssertions {
		failedSet[fa] = struct{}{}
	}
	hasPerMatcherResults := result.Success || result.FailedAssertions != nil
	var assertionDTOs []assertionDTO
	for _, a := range result.Assertions {
		passed := result.Success
		if hasPerMatcherResults {
			_, failed := failedSet[a]
			passed = !failed
		}
		assertionDTOs = append(assertionDTOs, assertionDTO{
			MatcherType: a.MatcherName(),
			AlertName:   a.AlertName(),
			Passed:      passed,
		})
	}
	assertionsJSON, err := json.Marshal(assertionDTOs)
	if err != nil {
		log.WithError(err).Warn("Failed to marshal assertions")
		assertionsJSON = nil
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
	return &db.ScenarioResult{
		RunID:             runID,
		Name:              result.Name,
		IsSuccess:         &isSuccess,
		ErrorMessage:      result.ErrorMessage,
		DurationSecs:      result.DurationSeconds,
		MatchingDurSecs:   result.MatchingDurationSeconds,
		TimeExecuted:      &result.TimeExecuted,
		ExecutorName:      result.ExecutorName,
		ExecutorType:      result.ExecutorType,
		ExecutionID:       result.ExecutionId,
		SimulationID:      result.SimulationID,
		Assertions:        assertionsJSON,
		Indicators:        indicatorsJSON,
		Metadata:          metadataJSON,
		CollectedLogPath:  collectedLogPath,
		CollectedDocCount: result.CollectedDocCount,
		DiscoveredAlerts:  discoveredAlertsJSON,
	}
}
