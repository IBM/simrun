package web

import (
	"encoding/json"
	"time"

	"github.com/IBM/simrun/internal/connectors/elastic"
	"github.com/IBM/simrun/internal/db"
	"sigs.k8s.io/yaml"
)

// Coverage response types

// CoverageResponse is the top-level response for the coverage endpoint.
type CoverageResponse struct {
	Summary CoverageSummary     `json:"summary"`
	Rules   []RuleCoverageEntry `json:"rules"`
}

// CoverageSummary contains aggregate coverage statistics.
type CoverageSummary struct {
	TotalRules      int     `json:"totalRules"`
	CoveredRules    int     `json:"coveredRules"`
	CoveragePercent float64 `json:"coveragePercent"`
}

// RuleCoverageEntry represents a single Elastic rule and its coverage status.
type RuleCoverageEntry struct {
	RuleID     string              `json:"ruleId"`
	Name       string              `json:"name"`
	Severity   string              `json:"severity"`
	RiskScore  int                 `json:"riskScore"`
	Tags       []string            `json:"tags"`
	Covered    bool                `json:"covered"`
	Scenarios  []CoverageScenario  `json:"scenarios"`
	LastResult *CoverageLastResult `json:"lastResult,omitempty"`
}

// CoverageScenario links a scenario to a rule it covers.
type CoverageScenario struct {
	ScenarioID   string `json:"scenarioId"`
	ScenarioName string `json:"scenarioName"`
	SimulationID string `json:"simulationId,omitempty"`
	PackName     string `json:"packName,omitempty"`
}

// CoverageLastResult holds the most recent test result for a rule.
type CoverageLastResult struct {
	Passed    bool      `json:"passed"`
	RunID     string    `json:"runId"`
	Timestamp time.Time `json:"timestamp"`
}

// Lightweight YAML parsing structs — just enough to extract expectations and
// detonator info without importing the full parser or instantiating executors.

type scenarioYAML struct {
	Scenarios []scenarioYAMLEntry `json:"scenarios"`
}

type scenarioYAMLEntry struct {
	Name         string                    `json:"name"`
	Detonate     *scenarioYAMLDetonate     `json:"detonate,omitempty"`
	Expectations []scenarioYAMLExpectation `json:"expectations"`
}

type scenarioYAMLDetonate struct {
	SimrunDetonator *scenarioYAMLSimrunDet `json:"simrunDetonator,omitempty"`
}

type scenarioYAMLSimrunDet struct {
	Pack       string `json:"pack"`
	Simulation string `json:"simulation"`
}

type scenarioYAMLExpectation struct {
	ElasticSecurityAlert *scenarioYAMLElasticAlert `json:"elasticSecurityAlert,omitempty"`
}

type scenarioYAMLElasticAlert struct {
	Name string `json:"name"`
}

// buildRuleNameToScenariosMap parses saved scenarios' YAML and returns a map
// from Elastic Security rule name to the scenarios that cover that rule.
func buildRuleNameToScenariosMap(scenarios []db.SavedScenario) map[string][]CoverageScenario {
	result := make(map[string][]CoverageScenario)

	for _, saved := range scenarios {
		jsonBytes, err := yaml.YAMLToJSON([]byte(saved.YAML))
		if err != nil {
			continue // skip invalid YAML
		}

		var parsed scenarioYAML
		if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
			continue // skip unparseable scenarios
		}

		for _, entry := range parsed.Scenarios {
			var packName, simulationID string
			if entry.Detonate != nil && entry.Detonate.SimrunDetonator != nil {
				packName = entry.Detonate.SimrunDetonator.Pack
				simulationID = entry.Detonate.SimrunDetonator.Simulation
			}

			for _, exp := range entry.Expectations {
				if exp.ElasticSecurityAlert == nil || exp.ElasticSecurityAlert.Name == "" {
					continue
				}
				ruleName := exp.ElasticSecurityAlert.Name
				result[ruleName] = append(result[ruleName], CoverageScenario{
					ScenarioID:   saved.ID.String(),
					ScenarioName: saved.Name,
					SimulationID: simulationID,
					PackName:     packName,
				})
			}
		}
	}

	return result
}

// buildCoverageResponse joins Elastic rules with scenario coverage data and
// the latest assertion results to produce the full coverage response.
func buildCoverageResponse(rules []elastic.RuleSummary, scenarioMap map[string][]CoverageScenario, assertionResults []db.LatestAssertionResult) CoverageResponse {
	// Build assertion lookup by alert name.
	assertionByName := make(map[string]db.LatestAssertionResult, len(assertionResults))
	for _, ar := range assertionResults {
		assertionByName[ar.AlertName] = ar
	}

	entries := make([]RuleCoverageEntry, 0, len(rules))
	coveredCount := 0

	for _, rule := range rules {
		scenarios := scenarioMap[rule.Name]
		covered := len(scenarios) > 0

		entry := RuleCoverageEntry{
			RuleID:    rule.RuleID,
			Name:      rule.Name,
			Severity:  rule.Severity,
			RiskScore: rule.RiskScore,
			Tags:      rule.Tags,
			Covered:   covered,
			Scenarios: scenarios,
		}

		// Attach the most recent assertion result if available.
		if ar, ok := assertionByName[rule.Name]; ok {
			entry.LastResult = &CoverageLastResult{
				Passed:    ar.Passed,
				RunID:     ar.RunID.String(),
				Timestamp: ar.CreatedAt,
			}
		}

		if entry.Scenarios == nil {
			entry.Scenarios = []CoverageScenario{}
		}
		if entry.Tags == nil {
			entry.Tags = []string{}
		}

		if covered {
			coveredCount++
		}

		entries = append(entries, entry)
	}

	var coveragePercent float64
	if len(rules) > 0 {
		coveragePercent = float64(coveredCount) / float64(len(rules)) * 100
	}

	return CoverageResponse{
		Summary: CoverageSummary{
			TotalRules:      len(rules),
			CoveredRules:    coveredCount,
			CoveragePercent: coveragePercent,
		},
		Rules: entries,
	}
}
