package web

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/IBM/simrun/internal/matchers"
	"github.com/IBM/simrun/internal/runner"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubMatcher is the smallest comparable struct that satisfies the
// AlertGeneratedMatcher interface. The map-key check in buildScenarioResultRow
// (failedSet) depends on the matcher being usable as a map key — only the
// MatcherName/AlertName methods are exercised here.
type stubMatcher struct {
	matcher string
	alert   string
}

func (m stubMatcher) HasExpectedAlert([]string, *logrus.Entry) (bool, error) { return false, nil }
func (m stubMatcher) String() string                                         { return m.matcher + "/" + m.alert }
func (m stubMatcher) Cleanup([]string, *logrus.Entry) error                  { return nil }
func (m stubMatcher) MatcherName() string                                    { return m.matcher }
func (m stubMatcher) AlertName() string                                      { return m.alert }

func TestBuildScenarioResultRow_SuccessWithAssertions(t *testing.T) {
	// A successful run with two matchers: both should be marked passed=true.
	runID := uuid.New()
	a1 := stubMatcher{matcher: "elastic", alert: "Suspicious AWS API Call"}
	a2 := stubMatcher{matcher: "datadog", alert: "Privilege Escalation"}
	res := &runner.ScenarioResult{
		Name:                    "scenario-a",
		Success:                 true,
		DurationSeconds:         1.5,
		MatchingDurationSeconds: 0.5,
		TimeExecuted:            time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		ExecutorName:            "simrun",
		ExecutorType:            "detonator",
		ExecutionId:             "exec-1",
		Matchers:                []matchers.AlertGeneratedMatcher{a1, a2},
	}

	row := buildScenarioResultRow(runID, res)
	require.NotNil(t, row)
	assert.Equal(t, runID, row.RunID)
	assert.Equal(t, "scenario-a", row.Name)
	require.NotNil(t, row.IsSuccess)
	assert.True(t, *row.IsSuccess)

	var got []expectationDTO
	require.NoError(t, json.Unmarshal(row.Expectations, &got))
	require.Len(t, got, 2)
	for _, d := range got {
		assert.True(t, d.Passed, "all assertions pass on a successful run: %+v", d)
	}
	assert.Equal(t, "elastic", got[0].MatcherType)
	assert.Equal(t, "Suspicious AWS API Call", got[0].AlertName)
}

func TestBuildScenarioResultRow_FailureWithNilFailedAssertions(t *testing.T) {
	// Scenario failed before assertion check — FailedAssertions is nil. The
	// fallback branch marks every assertion as failed (not "passed because
	// success=true" — that would be misleading).
	runID := uuid.New()
	a := stubMatcher{matcher: "elastic", alert: "Whatever"}
	res := &runner.ScenarioResult{
		Name:              "scenario-b",
		Success:           false,
		ErrorMessage:      "detonate timeout",
		Matchers:          []matchers.AlertGeneratedMatcher{a},
		UnmetExpectations: nil,
	}

	row := buildScenarioResultRow(runID, res)
	require.NotNil(t, row)
	require.NotNil(t, row.IsSuccess)
	assert.False(t, *row.IsSuccess)
	assert.Equal(t, "detonate timeout", row.ErrorMessage)

	var got []expectationDTO
	require.NoError(t, json.Unmarshal(row.Expectations, &got))
	require.Len(t, got, 1)
	assert.False(t, got[0].Passed, "fallback branch marks all assertions as failed when FailedAssertions is nil")
}

func TestBuildScenarioResultRow_ExploreModeIncludesDiscoveredAlerts(t *testing.T) {
	// Explore mode emits DiscoveredAlerts; the column is empty in non-explore runs.
	runID := uuid.New()
	res := &runner.ScenarioResult{
		Name:        "scenario-c",
		Success:     true,
		ExploreMode: true,
		DiscoveredAlerts: []runner.DiscoveredAlert{
			{RuleName: "my-rule", AlertID: "alert-1"},
		},
	}

	row := buildScenarioResultRow(runID, res)
	require.NotEmpty(t, row.DiscoveredAlerts, "explore-mode runs persist discovered alerts")
	var discovered []runner.DiscoveredAlert
	require.NoError(t, json.Unmarshal(row.DiscoveredAlerts, &discovered))
	require.Len(t, discovered, 1)
	assert.Equal(t, "my-rule", discovered[0].RuleName)
	assert.Equal(t, "alert-1", discovered[0].AlertID)

	// Sanity: a non-explore run with the same alerts leaves the column empty.
	res.ExploreMode = false
	row = buildScenarioResultRow(runID, res)
	assert.Empty(t, row.DiscoveredAlerts, "non-explore runs do not persist discovered alerts")
}
