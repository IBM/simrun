package elastic

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestElasticAlertMatchesExecution(t *testing.T) {
	// Test the alertMatchesExecution method
	assertion := &ElasticSecurityAlertMatcher{
		AlertFilter: ElasticSecurityAlertFilter{
			RuleName: "Test Rule",
		},
	}
	testLogger := logrus.WithFields(logrus.Fields{"matcher": "test"})

	alert := ElasticSecurityDetectionAlert{
		ID:    "test-alert-1",
		Index: ".alerts-security.alerts-default",
		Source: map[string]interface{}{
			"kibana.alert.rule.name": "Test Alert",
			"custom_field":           "my-detonation-uuid",
		},
	}

	// Should match when UUID is present
	matches := assertion.alertMatchesExecution(alert, []string{"my-detonation-uuid"}, testLogger)
	assert.True(t, matches)

	// Should not match when UUID is not present
	matches = assertion.alertMatchesExecution(alert, []string{"different-uuid"}, testLogger)
	assert.False(t, matches)
}

// TestAlertMatchesIndicatorsCaseInsensitive guards against providers (e.g. Azure)
// emitting fields in a different case than the indicator. Matching must succeed
// regardless of casing on either side; this test fails on the old case-sensitive
// strings.Contains implementation.
func TestAlertMatchesIndicatorsCaseInsensitive(t *testing.T) {
	alert := ElasticSecurityDetectionAlert{
		ID: "test-alert-1",
		Source: map[string]interface{}{
			"azure.resource_id": "/SUBSCRIPTIONS/ABC-123/RESOURCEGROUPS/RG",
		},
	}

	// Indicator is lower-case, alert value is upper-case.
	assert.True(t, alertMatchesIndicators(alert, []string{"abc-123"}))
	// Indicator is upper-case, alert value is upper-case but differently cased.
	assert.True(t, alertMatchesIndicators(alert, []string{"/subscriptions/abc-123/resourcegroups/rg"}))
	// Genuinely absent value still does not match.
	assert.False(t, alertMatchesIndicators(alert, []string{"def-456"}))
}

func TestBuildElasticAlertQuery(t *testing.T) {
	// Test the query building method
	assertion := &ElasticSecurityAlertMatcher{
		AlertFilter: ElasticSecurityAlertFilter{
			RuleName: "Test Rule",
			Severity: "high",
		},
	}

	query := assertion.buildElasticAlertQuery()
	assert.Contains(t, query, "Test Rule")
	assert.Contains(t, query, "high")
	assert.Contains(t, query, "kibana.alert.rule.name")
	assert.Contains(t, query, "kibana.alert.severity")
}

func TestElasticSecurityAlertMatcherString(t *testing.T) {
	assertion := &ElasticSecurityAlertMatcher{
		AlertFilter: ElasticSecurityAlertFilter{
			RuleName: "My Test Rule",
		},
	}

	result := assertion.String()
	expected := "Elastic Security alert 'My Test Rule'"
	assert.Equal(t, expected, result)
}
