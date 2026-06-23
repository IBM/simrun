package runner

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	detonatorMocks "github.com/IBM/simrun/internal/detonators/mocks"
	injectorMocks "github.com/IBM/simrun/internal/injectors/mocks"
	"github.com/IBM/simrun/internal/matchers"
	"github.com/IBM/simrun/internal/matchers/elastic"
	matcherMocks "github.com/IBM/simrun/internal/matchers/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

//TODO nuke interval for tests

func TestRunnerWorks(t *testing.T) {
	testCases := []struct {
		Name                string
		AlertExistsSequence []bool
		HasNoAssertion      bool
		ExpectError         bool
	}{
		{Name: "Alert exists from the beginning", AlertExistsSequence: []bool{true}},
		{Name: "Alert doesn't exist then exists", AlertExistsSequence: []bool{false, true}},
		{Name: "Alert never exists", AlertExistsSequence: []bool{false}, ExpectError: true},
		{Name: "No assertion", HasNoAssertion: true},
	}

	for i := range testCases {
		testCase := testCases[i]

		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()
			mockDetonator := &detonatorMocks.MockDetonator{}
			mockDetonator.On("Detonate").Return(map[string]string{"execution_id": "my-uid"}, nil)
			mockDetonator.On("String").Return("mock-detonator")
			mockDetonator.On("SimulationId").Return("test-simulation")
			mockDetonator.On("PackName").Return("")
			mockDetonator.On("SetStatusCallback", mock.AnythingOfType("func(string)")).Return()

			mockMatcher := &matcherMocks.MockAlertGeneratedMatcher{}
			if len(testCase.AlertExistsSequence) == 1 {
				mockMatcher.On("HasExpectedAlert", []string{"my-uid"}, mock.AnythingOfType("*logrus.Entry")).Return(testCase.AlertExistsSequence[0], nil)
			} else {
				for i := range testCase.AlertExistsSequence {
					mockMatcher.On("HasExpectedAlert", []string{"my-uid"}, mock.AnythingOfType("*logrus.Entry")).Return(testCase.AlertExistsSequence[i], nil).Once()
				}
			}
			mockMatcher.On("String").Return("sample")
			mockMatcher.On("MatcherName").Return("sample")
			mockMatcher.On("AlertName").Return("sample alert")
			mockMatcher.On("Cleanup", []string{"my-uid"}, mock.AnythingOfType("*logrus.Entry")).Return(nil)

			var assertions []matchers.AlertGeneratedMatcher
			assertions = []matchers.AlertGeneratedMatcher{}
			if !testCase.HasNoAssertion {
				assertions = []matchers.AlertGeneratedMatcher{mockMatcher}
			}

			runner := Runner{
				Scenarios: []*Scenario{
					{
						Name:       "test-scenario",
						Detonator:  mockDetonator,
						Assertions: assertions,
						Timeout:    50 * time.Millisecond,
					},
				},
				Interval: 1 * time.Millisecond,
			}
			results, err := runner.Run()
			if testCase.ExpectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Len(t, results, 1)
				assert.True(t, results[0].Success)
			}
			mockDetonator.AssertNumberOfCalls(t, "Detonate", 1)

			if !testCase.HasNoAssertion {
				mockMatcher.AssertCalled(t, "Cleanup", []string{"my-uid"}, mock.AnythingOfType("*logrus.Entry"))
			}

		})
	}

}

func TestRunnerErrorHandling(t *testing.T) {

	mockDetonator := &detonatorMocks.MockDetonator{}
	mockDetonator.On("Detonate").Return(map[string]string{"execution_id": "my-uid"}, nil)
	mockDetonator.On("String").Return("mock-detonator")
	mockDetonator.On("SimulationId").Return("test-simulation")
	mockDetonator.On("PackName").Return("")
	mockDetonator.On("SetStatusCallback", mock.AnythingOfType("func(string)")).Return()

	mockFailingDetonator := &detonatorMocks.MockDetonator{}
	mockFailingDetonator.On("Detonate").Return(map[string]string{"execution_id": "failed-uid"}, errors.New("foo"))
	mockFailingDetonator.On("String").Return("mock-failing-detonator")
	mockFailingDetonator.On("SimulationId").Return("failing-simulation")
	mockFailingDetonator.On("PackName").Return("")
	mockFailingDetonator.On("SetStatusCallback", mock.AnythingOfType("func(string)")).Return()

	mockMatcher := &matcherMocks.MockAlertGeneratedMatcher{}
	mockMatcher.On("String").Return("sample")
	mockMatcher.On("MatcherName").Return("sample")
	mockMatcher.On("AlertName").Return("sample alert")
	mockMatcher.On("Cleanup", []string{"my-uid"}, mock.AnythingOfType("*logrus.Entry")).Return(nil)
	mockMatcher.On("HasExpectedAlert", []string{"my-uid"}, mock.AnythingOfType("*logrus.Entry")).Return(true, nil)

	runner := Runner{
		Scenarios: []*Scenario{
			{
				Name:       "test-scenario1",
				Detonator:  mockDetonator,
				Assertions: []matchers.AlertGeneratedMatcher{mockMatcher},
				Timeout:    5 * time.Second,
			},
			{
				Name:       "test-scenario2-error",
				Detonator:  mockFailingDetonator,
				Assertions: []matchers.AlertGeneratedMatcher{mockMatcher},
				Timeout:    5 * time.Second,
			},
			{
				Name:       "test-scenario3",
				Detonator:  mockDetonator,
				Assertions: []matchers.AlertGeneratedMatcher{mockMatcher},
				Timeout:    5 * time.Second,
			},
		},
		Interval: 0,
	}
	results, err := runner.Run()
	assert.Error(t, err, "the runner should return an error when a scenario returns an error")
	assert.Len(t, results, 3) // Should have results for all scenarios

	// Check that we have both success and failed scenarios
	var successCount, failedCount int
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failedCount++
		}
	}
	assert.Equal(t, 2, successCount)
	assert.Equal(t, 1, failedCount)

	// A failed scenario must still carry its execution_id so the result can be
	// correlated with the partial detonation (e.g. terraform that did apply).
	for _, result := range results {
		if !result.Success {
			assert.Equal(t, "failed-uid", result.ExecutionId,
				"failed scenario should retain the execution_id from the detonation output")
		}
	}

	// All scenarios should have been detonated, even if one returned an error
	mockDetonator.AssertNumberOfCalls(t, "Detonate", 2)
	mockFailingDetonator.AssertNumberOfCalls(t, "Detonate", 1)
}

func TestRunnerWithExecutionUUID(t *testing.T) {
	mockDetonator := &detonatorMocks.MockDetonator{}
	mockDetonator.On("Detonate").Return(map[string]string{
		"execution_id":   "my-uid",
		"execution_uuid": "d41b8fa5-c7b0-5b8e-831c-f68b932e1234",
	}, nil)
	mockDetonator.On("String").Return("mock-detonator")
	mockDetonator.On("SimulationId").Return("test-simulation")
	mockDetonator.On("PackName").Return("")
	mockDetonator.On("SetStatusCallback", mock.AnythingOfType("func(string)")).Return()

	expectedIndicators := []string{"my-uid", "d41b8fa5-c7b0-5b8e-831c-f68b932e1234"}

	mockMatcher := &matcherMocks.MockAlertGeneratedMatcher{}
	mockMatcher.On("HasExpectedAlert", expectedIndicators, mock.AnythingOfType("*logrus.Entry")).Return(true, nil)
	mockMatcher.On("String").Return("sample")
	mockMatcher.On("MatcherName").Return("sample")
	mockMatcher.On("AlertName").Return("sample alert")
	mockMatcher.On("Cleanup", expectedIndicators, mock.AnythingOfType("*logrus.Entry")).Return(nil)

	runner := NewRunner()
	runner.Scenarios = []*Scenario{
		{
			Name:       "test-scenario-with-uuid",
			Detonator:  mockDetonator,
			Assertions: []matchers.AlertGeneratedMatcher{mockMatcher},
			Timeout:    5 * time.Second,
		},
	}
	runner.Interval = 0

	results, err := runner.Run()
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)

	// Verify both nanoid and UUID were passed to matcher
	mockMatcher.AssertCalled(t, "HasExpectedAlert", expectedIndicators, mock.AnythingOfType("*logrus.Entry"))
	mockMatcher.AssertCalled(t, "Cleanup", expectedIndicators, mock.AnythingOfType("*logrus.Entry"))
}

// newFakeElasticServer returns an httptest server that returns the given alerts
// from POST /api/detection_engine/signals/search.
func newFakeElasticServer(t *testing.T, alerts []elastic.ElasticSecurityDetectionAlert) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := elastic.ElasticSecurityDetectionEngineSearchResponse{}
		resp.Hits.Total.Value = len(alerts)
		resp.Hits.Hits = alerts
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}

func TestRunnerExploreModeFailsWhenNoAlertsDiscovered(t *testing.T) {
	ts := newFakeElasticServer(t, nil)
	defer ts.Close()

	mockDetonator := &detonatorMocks.MockDetonator{}
	mockDetonator.On("Detonate").Return(map[string]string{"execution_id": "no-match-uid"}, nil)
	mockDetonator.On("String").Return("mock-detonator")
	mockDetonator.On("SimulationId").Return("test-simulation")
	mockDetonator.On("PackName").Return("")
	mockDetonator.On("SetStatusCallback", mock.AnythingOfType("func(string)")).Return()
	mockDetonator.On("SetEnvVars", mock.AnythingOfType("map[string]string")).Return()

	scenario := &Scenario{
		Name:        "explore-no-alerts",
		Detonator:   mockDetonator,
		ExploreMode: true,
		Timeout:     30 * time.Millisecond,
		EnvVars: map[string]string{
			"SR_KIBANA_URL":      ts.URL,
			"SR_ELASTIC_API_KEY": "test",
		},
	}

	runner := Runner{
		Scenarios: []*Scenario{scenario},
		Interval:  1 * time.Millisecond,
	}

	results, err := runner.Run()
	assert.Error(t, err, "explore mode with zero discovered alerts should fail")
	assert.Len(t, results, 1)
	assert.False(t, results[0].Success)
	assert.Contains(t, results[0].Error, "no matching alerts discovered")
	assert.Empty(t, scenario.DiscoveredAlerts)
}

func TestRunnerExploreModeSucceedsWhenAlertsDiscovered(t *testing.T) {
	matchingAlert := elastic.ElasticSecurityDetectionAlert{
		ID: "alert-1",
		Source: map[string]interface{}{
			"kibana.alert.rule.name": "Test Rule",
			"user.name":              "match-uid",
		},
	}
	ts := newFakeElasticServer(t, []elastic.ElasticSecurityDetectionAlert{matchingAlert})
	defer ts.Close()

	mockDetonator := &detonatorMocks.MockDetonator{}
	mockDetonator.On("Detonate").Return(map[string]string{"execution_id": "match-uid"}, nil)
	mockDetonator.On("String").Return("mock-detonator")
	mockDetonator.On("SimulationId").Return("test-simulation")
	mockDetonator.On("PackName").Return("")
	mockDetonator.On("SetStatusCallback", mock.AnythingOfType("func(string)")).Return()
	mockDetonator.On("SetEnvVars", mock.AnythingOfType("map[string]string")).Return()

	scenario := &Scenario{
		Name:        "explore-with-alerts",
		Detonator:   mockDetonator,
		ExploreMode: true,
		Timeout:     30 * time.Millisecond,
		EnvVars: map[string]string{
			"SR_KIBANA_URL":      ts.URL,
			"SR_ELASTIC_API_KEY": "test",
		},
	}

	runner := Runner{
		Scenarios: []*Scenario{scenario},
		Interval:  1 * time.Millisecond,
	}

	results, err := runner.Run()
	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.GreaterOrEqual(t, len(scenario.DiscoveredAlerts), 1)
	assert.Equal(t, "alert-1", scenario.DiscoveredAlerts[0].AlertID)
}

func TestRunnerWithInjector(t *testing.T) {
	// Mock injector
	mockInjector := &injectorMocks.MockInjector{}
	mockInjector.On("Inject").Return(map[string]string{"execution_id": "my-injection-uid"}, nil)
	mockInjector.On("String").Return("MockInjector")

	// Mock matcher
	mockMatcher := &matcherMocks.MockAlertGeneratedMatcher{}
	mockMatcher.On("HasExpectedAlert", []string{"my-injection-uid"}, mock.AnythingOfType("*logrus.Entry")).Return(true, nil)
	mockMatcher.On("String").Return("sample")
	mockMatcher.On("MatcherName").Return("sample")
	mockMatcher.On("AlertName").Return("sample alert")
	mockMatcher.On("Cleanup", []string{"my-injection-uid"}, mock.AnythingOfType("*logrus.Entry")).Return(nil)

	scenario := Scenario{
		Name:       "test scenario with injector",
		Injector:   mockInjector,
		Assertions: []matchers.AlertGeneratedMatcher{mockMatcher},
		Timeout:    1 * time.Second,
	}

	runner := NewRunner()
	runner.Scenarios = []*Scenario{&scenario}
	runner.Interval = 10 * time.Millisecond

	results, err := runner.Run()
	assert.NoError(t, err, "the runner should not return an error when the injection succeeds")
	assert.Len(t, results, 1)
	assert.True(t, results[0].Success)
	assert.Equal(t, "my-injection-uid", results[0].ExecutionId)

	mockInjector.AssertNumberOfCalls(t, "Inject", 1)
}
