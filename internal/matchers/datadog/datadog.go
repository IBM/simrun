// Package datadog matches expected Datadog security signals.
package datadog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
	"github.com/aws/smithy-go/ptr"
	"github.com/sirupsen/logrus"
)

const QueryAllOpenSignals = `@workflow.triage.state:open`

func (m *DatadogAlertGeneratedAssertionBuilder) HasExpectedAlert(indicators []string, logger *logrus.Entry) (bool, error) {
	return m.DatadogAlertGeneratedAssertion.HasExpectedAlert(indicators, logger)
}

func (m *DatadogAlertGeneratedAssertionBuilder) Cleanup(indicators []string, logger *logrus.Entry) error {
	return m.DatadogAlertGeneratedAssertion.Cleanup(indicators, logger)
}

const QueryOpenSignalsByAlertNameAndSeverity = `@workflow.triage.state:open @workflow.rule.name:"%s" %s`
const QuerySeverity = `status:%s`

type DatadogSecuritySignalsAPI interface {
	SearchSignals(query string) ([]datadogV2.SecurityMonitoringSignal, error)
	CloseSignal(id string) error
}

type DatadogSecuritySignalsAPIImpl struct {
	securityMonitoringAPI *datadogV2.SecurityMonitoringApi
	ctx                   context.Context
}

func (m *DatadogSecuritySignalsAPIImpl) SearchSignals(query string) ([]datadogV2.SecurityMonitoringSignal, error) {
	maxSignals := 1000
	params := datadogV2.NewSearchSecurityMonitoringSignalsOptionalParameters().WithBody(datadogV2.SecurityMonitoringSignalListRequest{
		Filter: &datadogV2.SecurityMonitoringSignalListRequestFilter{
			From:  datadog.PtrTime(time.Now().Add(-1 * time.Hour)), // Signals no older than 1 hour
			Query: datadog.PtrString(query),
		},
		Page: &datadogV2.SecurityMonitoringSignalListRequestPage{Limit: ptr.Int32(int32(maxSignals))},
		Sort: datadogV2.SECURITYMONITORINGSIGNALSSORT_TIMESTAMP_DESCENDING.Ptr(),
	})

	signals, _, err := m.securityMonitoringAPI.SearchSecurityMonitoringSignals(m.ctx, *params)

	if len(signals.Data) >= maxSignals {
		return nil, errors.New("unsupported: more than 1000 open signals") // todo: paginate response
	}
	return signals.Data, err
}

func (m *DatadogSecuritySignalsAPIImpl) CloseSignal(id string) error {
	payload, _ := json.Marshal(map[string]interface{}{
		"state":          "archived",
		"archiveReason":  "testing_or_maintenance",
		"archiveComment": "End to end detection testing",
	})
	path := fmt.Sprintf("api/v1/security_analytics/signals/%s/state", id)
	ddSite := (m.ctx.Value(datadog.ContextServerVariables).(map[string]string))["site"]
	req, err := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("https://api.%s/%s", ddSite, path),
		bytes.NewBuffer(payload),
	)

	if err != nil {
		return err
	}
	keys := m.ctx.Value(datadog.ContextAPIKeys).(map[string]datadog.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("DD-API-KEY", keys["apiKeyAuth"].Key)
	req.Header.Set("DD-APPLICATION-KEY", keys["appKeyAuth"].Key)

	client := &http.Client{}
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	if response.StatusCode != 200 {
		return errors.New("unable to archive signal, got status code " + strconv.Itoa(response.StatusCode))
	}
	return nil
}

func (m *DatadogAlertGeneratedAssertion) HasExpectedAlert(indicators []string, logger *logrus.Entry) (bool, error) {
	logger = m.prepareLogger(logger)

	query := m.buildDatadogSignalQuery()
	logger.Info("Querying Datadog for security signals")

	signals, err := m.searchSignalsWithQuery(query)
	if err != nil {
		return false, err
	}

	matchingSignal := m.findMatchingSignal(signals, indicators, logger)
	return matchingSignal != nil, nil
}

func (m *DatadogAlertGeneratedAssertion) String() string {
	return fmt.Sprintf("Datadog security signal '%s'", m.AlertFilter.RuleName)
}

func (m *DatadogAlertGeneratedAssertion) MatcherName() string {
	return "Datadog security signal"
}

func (m *DatadogAlertGeneratedAssertion) AlertName() string {
	return m.AlertFilter.RuleName
}

func (m *DatadogAlertGeneratedAssertion) Cleanup(indicators []string, logger *logrus.Entry) error {
	logger = m.prepareLogger(logger)

	signals, err := m.searchSignalsWithQuery(QueryAllOpenSignals)
	if err != nil {
		return err
	}

	for i := range signals {
		if m.signalMatchesExecution(signals[i], indicators, logger) {
			if err := m.SignalsAPI.CloseSignal(*signals[i].Id); err != nil {
				return errors.New("unable to archive signal " + *signals[i].Id + ": " + err.Error())
			}
		}
	}

	return nil
}

// TODO: Would probably make more sense to retrieve all open signal and iterate instead of doing 2 pass
func (m *DatadogAlertGeneratedAssertion) buildDatadogSignalQuery() string {
	severityQuery := ""
	if m.AlertFilter.Severity != "" {
		severityQuery = fmt.Sprintf(QuerySeverity, m.AlertFilter.Severity) + " "
	}
	return fmt.Sprintf(
		QueryOpenSignalsByAlertNameAndSeverity,
		m.AlertFilter.RuleName,
		severityQuery,
	)
}

func (m *DatadogAlertGeneratedAssertion) prepareLogger(logger *logrus.Entry) *logrus.Entry {
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}
	return logger.WithFields(logrus.Fields{
		"matcher":   "DatadogSecuritySignal",
		"rule_name": m.AlertFilter.RuleName,
	})
}

func (m *DatadogAlertGeneratedAssertion) searchSignalsWithQuery(query string) ([]datadogV2.SecurityMonitoringSignal, error) {
	signals, err := m.SignalsAPI.SearchSignals(query)
	if err != nil {
		return nil, errors.New("unable to search for Datadog security signal: " + err.Error())
	}
	return signals, nil
}

func (m *DatadogAlertGeneratedAssertion) findMatchingSignal(signals []datadogV2.SecurityMonitoringSignal, indicators []string, logger *logrus.Entry) *datadogV2.SecurityMonitoringSignal {
	logger.WithField("signal_count", len(signals)).Info("Received signals from Datadog")

	if len(signals) == 0 {
		return nil
	}

	for i := range signals {
		if m.signalMatchesExecution(signals[i], indicators, logger) {
			return &signals[i]
		}
	}

	return nil
}

func (m *DatadogAlertGeneratedAssertion) signalMatchesExecution(signal datadogV2.SecurityMonitoringSignal, indicators []string, logger *logrus.Entry) bool {
	buf, _ := json.Marshal(signal.Attributes.Custom)
	rawSignal := string(buf)

	for _, indicator := range indicators {
		if strings.Contains(rawSignal, indicator) {
			logger.WithField("indicator", indicator).Debug("Found matching signal based on provided indicators")
			return true
		}
	}
	return false
}
