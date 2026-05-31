package web

import (
	"testing"
	"time"

	"github.com/IBM/simrun/simrun/internal/connectors/elastic"
	"github.com/IBM/simrun/simrun/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestBuildRuleNameToScenariosMap(t *testing.T) {
	scenario1ID := uuid.New()
	scenario2ID := uuid.New()

	scenarios := []db.SavedScenario{
		{
			ID:   scenario1ID,
			Name: "AWS IAM Brute Force Test",
			YAML: `scenarios:
  - name: iam brute force
    detonate:
      simrunDetonator:
        pack: aws
        simulation: iam-brute-force
    expectations:
      - elasticSecurityAlert:
          name: "AWS IAM Brute Force"
`,
		},
		{
			ID:   scenario2ID,
			Name: "Multi Rule Test",
			YAML: `scenarios:
  - name: multi rule
    detonate:
      awsCliDetonator:
        script: "true"
    expectations:
      - elasticSecurityAlert:
          name: "Rule A"
      - elasticSecurityAlert:
          name: "Rule B"
`,
		},
	}

	result := buildRuleNameToScenariosMap(scenarios)

	assert.Len(t, result, 3)

	// Check "AWS IAM Brute Force"
	assert.Len(t, result["AWS IAM Brute Force"], 1)
	assert.Equal(t, "aws", result["AWS IAM Brute Force"][0].PackName)
	assert.Equal(t, "iam-brute-force", result["AWS IAM Brute Force"][0].SimulationID)

	// Check "Rule A"
	assert.Len(t, result["Rule A"], 1)
	assert.Empty(t, result["Rule A"][0].PackName)

	// Check "Rule B"
	assert.Len(t, result["Rule B"], 1)
	assert.Empty(t, result["Rule B"][0].PackName)
}

func TestBuildRuleNameToScenariosMap_InvalidYAML(t *testing.T) {
	scenarios := []db.SavedScenario{
		{
			ID:   uuid.New(),
			Name: "Bad Scenario",
			YAML: `this is not valid: yaml: [[[`,
		},
	}

	result := buildRuleNameToScenariosMap(scenarios)

	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestBuildCoverageResponse(t *testing.T) {
	rules := []elastic.RuleSummary{
		{
			RuleID:    "rule-1",
			Name:      "Covered Rule",
			Severity:  "high",
			RiskScore: 73,
			Tags:      []string{"AWS"},
		},
		{
			RuleID:    "rule-2",
			Name:      "Uncovered Rule",
			Severity:  "low",
			RiskScore: 21,
			Tags:      []string{"Linux"},
		},
	}

	scenarioMap := map[string][]CoverageScenario{
		"Covered Rule": {
			{
				ScenarioID:   uuid.New().String(),
				ScenarioName: "Test Scenario",
				SimulationID: "sim-1",
				PackName:     "aws",
			},
		},
	}

	runID := uuid.New()
	now := time.Now()
	assertionResults := []db.LatestAssertionResult{
		{
			AlertName: "Covered Rule",
			Passed:    true,
			RunID:     runID,
			CreatedAt: now,
		},
	}

	resp := buildCoverageResponse(rules, scenarioMap, assertionResults)

	// Check summary
	assert.Equal(t, 2, resp.Summary.TotalRules)
	assert.Equal(t, 1, resp.Summary.CoveredRules)
	assert.InDelta(t, 50.0, resp.Summary.CoveragePercent, 0.1)

	// Check covered rule
	assert.True(t, resp.Rules[0].Covered)
	assert.Len(t, resp.Rules[0].Scenarios, 1)
	assert.NotNil(t, resp.Rules[0].LastResult)
	assert.True(t, resp.Rules[0].LastResult.Passed)

	// Check uncovered rule
	assert.False(t, resp.Rules[1].Covered)
	assert.Empty(t, resp.Rules[1].Scenarios)
	assert.Nil(t, resp.Rules[1].LastResult)
}

func TestBuildCoverageResponse_NoRules(t *testing.T) {
	resp := buildCoverageResponse(nil, nil, nil)

	assert.Equal(t, 0, resp.Summary.TotalRules)
	assert.Equal(t, 0.0, resp.Summary.CoveragePercent)
	assert.NotNil(t, resp.Rules)
	assert.Empty(t, resp.Rules)
}
