package datadog

import (
	"context"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/IBM/simrun/internal/envutil"
	log "github.com/sirupsen/logrus"
)

type DatadogAlertFilter struct {
	RuleName string `yaml:"rule-name"`
	Severity string
	// There might be other attributes in the future
}

type DatadogAlertMatcher struct {
	SignalsAPI  DatadogSecuritySignalsAPI
	AlertFilter *DatadogAlertFilter
}

// builder
type DatadogAlertMatcherBuilder struct {
	DatadogAlertMatcher
}

// getEnvWithFallback returns the value of the primary env var, or falls back to legacy
// and logs a deprecation warning if the legacy var is used.
// Uses explicit envVars if provided, falling back to os.Getenv.
func getEnvWithFallback(envVars map[string]string, primary, legacy string) string {
	if value := envutil.Lookup(envVars, primary); value != "" {
		return value
	}
	if value := envutil.Lookup(envVars, legacy); value != "" {
		log.Warnf("Environment variable %s is deprecated, please use %s instead", legacy, primary)
		return value
	}
	return ""
}

func getDDSite(envVars map[string]string) string {
	if site := getEnvWithFallback(envVars, "SR_DATADOG_SITE", "DD_SITE"); site != "" {
		return site
	}
	return "datadoghq.com"
}

// DatadogSecuritySignal creates a new Datadog security signal matcher.
// envVars provides run-specific env vars; pass nil to read from process env (CLI path).
func DatadogSecuritySignal(name string, envVars map[string]string) *DatadogAlertMatcherBuilder {
	builder := &DatadogAlertMatcherBuilder{}
	ddApiKey := getEnvWithFallback(envVars, "SR_DATADOG_API_KEY", "DD_API_KEY")
	ddAppKey := getEnvWithFallback(envVars, "SR_DATADOG_APP_KEY", "DD_APP_KEY")
	ctx := context.WithValue(context.Background(), datadog.ContextAPIKeys, map[string]datadog.APIKey{
		"apiKeyAuth": {Key: ddApiKey},
		"appKeyAuth": {Key: ddAppKey},
	})
	ctx = context.WithValue(ctx, datadog.ContextServerVariables, map[string]string{
		"site": getDDSite(envVars),
	})
	cfg := datadog.NewConfiguration()
	cfg.SetUnstableOperationEnabled("SearchSecurityMonitoringSignals", true)

	builder.SignalsAPI = &DatadogSecuritySignalsAPIImpl{
		securityMonitoringAPI: datadogV2.NewSecurityMonitoringApi(datadog.NewAPIClient(cfg)),
		ctx:                   ctx,
	}
	builder.AlertFilter = &DatadogAlertFilter{RuleName: name}
	return builder
}

func (m *DatadogAlertMatcherBuilder) WithSeverity(severity string) *DatadogAlertMatcherBuilder {
	m.AlertFilter.Severity = severity
	return m
}
