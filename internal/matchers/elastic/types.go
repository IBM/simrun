package elastic

import "time"

// ElasticAlertFilter holds filtering criteria for elastic alerts
type ElasticSecurityAlertFilter struct {
	RuleName string `yaml:"rule-name"`
	Severity string
}

// ElasticSecurityAlertMatcher implements the AlertGeneratedMatcher interface
type ElasticSecurityAlertMatcher struct {
	AlertsAPI   ElasticSecurityDetectionAlertsAPI
	AlertFilter ElasticSecurityAlertFilter
	envVars     map[string]string // run-specific env vars for credential isolation
	since       time.Time         // only match alerts after this time; zero means now-10h fallback
}

// ElasticSecurityAlert creates a new elastic security alert matcher.
// The API client is lazily initialized when first used (in HasExpectedAlert or Cleanup),
// allowing lint to validate scenario files without requiring API credentials.
// envVars provides run-specific env vars; pass nil to read from process env (CLI path).
func ElasticSecurityAlert(name string, envVars map[string]string) (*ElasticSecurityAlertMatcher, error) {
	return &ElasticSecurityAlertMatcher{
		AlertFilter: ElasticSecurityAlertFilter{RuleName: name},
		envVars:     envVars,
	}, nil
}

// WithSeverity adds severity filtering to the matcher
// Returns self for method chaining
func (m *ElasticSecurityAlertMatcher) WithSeverity(severity string) *ElasticSecurityAlertMatcher {
	// Modify in place - no need to create new objects
	m.AlertFilter.Severity = severity
	return m
}

// SetSince restricts the query to alerts created after the given time.
func (m *ElasticSecurityAlertMatcher) SetSince(t time.Time) {
	m.since = t
}
