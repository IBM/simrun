package web

import (
	"encoding/json"

	"github.com/IBM/simrun/simrun/internal/db"
	"github.com/IBM/simrun/simrun/internal/matchers"
	"github.com/IBM/simrun/simrun/internal/results"
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
