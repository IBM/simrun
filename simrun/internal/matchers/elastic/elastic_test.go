package elastic

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestElasticAlertMatchesExecution(t *testing.T) {
	// Test the alertMatchesExecution method
	assertion := &ElasticSecurityAlertGeneratedAssertion{
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

func TestBuildElasticAlertQuery(t *testing.T) {
	// Test the query building method
	assertion := &ElasticSecurityAlertGeneratedAssertion{
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

func TestElasticAlertGeneratedAssertionString(t *testing.T) {
	assertion := &ElasticSecurityAlertGeneratedAssertion{
		AlertFilter: ElasticSecurityAlertFilter{
			RuleName: "My Test Rule",
		},
	}

	result := assertion.String()
	expected := "Elastic Security alert 'My Test Rule'"
	assert.Equal(t, expected, result)
}
