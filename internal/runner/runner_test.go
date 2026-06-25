package runner

import (
	"encoding/json"
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
		HasNoMatcher        bool
		ExpectError         bool
	}{
		{Name: "Alert exists from the beginning", AlertExistsSequence: []bool{true}},
		{Name: "Alert doesn't exist then exists", AlertExistsSequence: []bool{false, true}},
		{Name: "Alert never exists", AlertExistsSequence: []bool{false}, ExpectError: true},
		{Name: "No matcher", HasNoMatcher: true},
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

			var matcherList []matchers.AlertGeneratedMatcher
			if !testCase.HasNoMatcher {
				matcherList = []matchers.AlertGeneratedMatcher{mockMatcher}
			}

			r := Runner{Interval: 1 * time.Millisecond}
			result := r.Run(&Scenario{
				Name:      "test-scenario",
				Detonator: mockDetonator,
				Matchers:  matcherList,
				Timeout:   50 * time.Millisecond,
			})
			if testCase.ExpectError {
				assert.False(t, result.Success)
				assert.NotEmpty(t, result.ErrorMessage)
			} else {
				assert.True(t, result.Success)
				assert.Empty(t, result.ErrorMessage)
			}
			mockDetonator.AssertNumberOfCalls(t, "Detonate", 1)

			if !testCase.HasNoMatcher {
				mockMatcher.AssertCalled(t, "Cleanup", []string{"my-uid"}, mock.AnythingOfType("*logrus.Entry"))
			}
		})
	}
}

// TestRunnerFailedDetonationRetainsExecutionID verifies that a scenario whose
// detonation errors still surfaces the execution_id (so a partial detonation
// stays correlatable) and is reported as a failure.
func TestRunnerFailedDetonationRetainsExecutionID(t *testing.T) {
	mockFailingDetonator := &detonatorMocks.MockDetonator{}
	mockFailingDetonator.On("Detonate").Return(map[string]string{"execution_id": "failed-uid"}, assert.AnError)
	mockFailingDetonator.On("String").Return("mock-failing-detonator")
	mockFailingDetonator.On("SimulationId").Return("failing-simulation")
	mockFailingDetonator.On("PackName").Return("")
	mockFailingDetonator.On("SetStatusCallback", mock.AnythingOfType("func(string)")).Return()

	mockMatcher := &matcherMocks.MockAlertGeneratedMatcher{}
	mockMatcher.On("String").Return("sample")
	mockMatcher.On("MatcherName").Return("sample")
	mockMatcher.On("AlertName").Return("sample alert")

	r := Runner{Interval: 0}
	result := r.Run(&Scenario{
		Name:      "test-scenario-error",
		Detonator: mockFailingDetonator,
		Matchers:  []matchers.AlertGeneratedMatcher{mockMatcher},
		Timeout:   5 * time.Second,
	})

	assert.False(t, result.Success, "a scenario whose detonation errors must fail")
	assert.Equal(t, "failed-uid", result.ExecutionId,
		"failed scenario should retain the execution_id from the detonation output")
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

	r := NewRunner()
	r.Interval = 0
	result := r.Run(&Scenario{
		Name:      "test-scenario-with-uuid",
		Detonator: mockDetonator,
		Matchers:  []matchers.AlertGeneratedMatcher{mockMatcher},
		Timeout:   5 * time.Second,
	})
	assert.True(t, result.Success)

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

	r := Runner{Interval: 1 * time.Millisecond}
	result := r.Run(&Scenario{
		Name:        "explore-no-alerts",
		Detonator:   mockDetonator,
		ExploreMode: true,
		Timeout:     30 * time.Millisecond,
		EnvVars: map[string]string{
			"SR_KIBANA_URL":      ts.URL,
			"SR_ELASTIC_API_KEY": "test",
		},
	})

	assert.False(t, result.Success, "explore mode with zero discovered alerts should fail")
	assert.Contains(t, result.ErrorMessage, "no matching alerts discovered")
	assert.Empty(t, result.DiscoveredAlerts)
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

	r := Runner{Interval: 1 * time.Millisecond}
	result := r.Run(&Scenario{
		Name:        "explore-with-alerts",
		Detonator:   mockDetonator,
		ExploreMode: true,
		Timeout:     30 * time.Millisecond,
		EnvVars: map[string]string{
			"SR_KIBANA_URL":      ts.URL,
			"SR_ELASTIC_API_KEY": "test",
		},
	})

	assert.True(t, result.Success)
	assert.GreaterOrEqual(t, len(result.DiscoveredAlerts), 1)
	assert.Equal(t, "alert-1", result.DiscoveredAlerts[0].AlertID)
}

// TestRunnerFiresIdentityAndExpectationCallbacks verifies the mid-run hooks that
// power the live scenario detail view: identity is emitted exactly once after
// detonation, and the expectations callback fires once per newly-matched
// expectation, carrying passed=true for matches and nil (pending) for the rest.
func TestRunnerFiresIdentityAndExpectationCallbacks(t *testing.T) {
	mockDetonator := &detonatorMocks.MockDetonator{}
	mockDetonator.On("Detonate").Return(map[string]string{"execution_id": "exec-123"}, nil)
	mockDetonator.On("String").Return("mock-detonator")
	mockDetonator.On("SimulationId").Return("sim-456")
	mockDetonator.On("PackName").Return("")
	mockDetonator.On("SetStatusCallback", mock.AnythingOfType("func(string)")).Return()

	// matcherA matches on the first poll; matcherB only on the second, so the
	// expectations callback must fire twice with distinct partial states.
	matcherA := &matcherMocks.MockAlertGeneratedMatcher{}
	matcherA.On("HasExpectedAlert", []string{"exec-123"}, mock.AnythingOfType("*logrus.Entry")).Return(true, nil)
	matcherA.On("MatcherName").Return("Elastic")
	matcherA.On("AlertName").Return("alert-a")
	matcherA.On("String").Return("alert-a")
	matcherA.On("Cleanup", []string{"exec-123"}, mock.AnythingOfType("*logrus.Entry")).Return(nil)

	matcherB := &matcherMocks.MockAlertGeneratedMatcher{}
	matcherB.On("HasExpectedAlert", []string{"exec-123"}, mock.AnythingOfType("*logrus.Entry")).Return(false, nil).Once()
	matcherB.On("HasExpectedAlert", []string{"exec-123"}, mock.AnythingOfType("*logrus.Entry")).Return(true, nil)
	matcherB.On("MatcherName").Return("Elastic")
	matcherB.On("AlertName").Return("alert-b")
	matcherB.On("String").Return("alert-b")
	matcherB.On("Cleanup", []string{"exec-123"}, mock.AnythingOfType("*logrus.Entry")).Return(nil)

	var identities []ScenarioIdentity
	var snapshots [][]ExpectationResult

	scenario := &Scenario{
		Name:      "callback-scenario",
		Detonator: mockDetonator,
		Matchers:  []matchers.AlertGeneratedMatcher{matcherA, matcherB},
		Timeout:   5 * time.Second,
		IdentityCallback: func(_ string, id ScenarioIdentity) {
			identities = append(identities, id)
		},
		ExpectationsCallback: func(_ string, results []ExpectationResult) {
			snap := make([]ExpectationResult, len(results))
			copy(snap, results)
			snapshots = append(snapshots, snap)
		},
	}

	r := Runner{Interval: 1 * time.Millisecond}
	result := r.Run(scenario)
	assert.True(t, result.Success)

	// Identity fires exactly once, after detonation, carrying all four fields.
	assert.Len(t, identities, 1)
	assert.Equal(t, ScenarioIdentity{
		ExecutorName: "mock-detonator",
		ExecutorType: "detonator",
		ExecutionID:  "exec-123",
		SimulationID: "sim-456",
	}, identities[0])

	// One callback per newly-matched expectation: A then B → two snapshots.
	assert.Len(t, snapshots, 2)

	first := snapshots[0]
	assertExpectationPassed(t, first, "alert-a", boolPtr(true))
	assertExpectationPassed(t, first, "alert-b", nil) // still pending

	last := snapshots[len(snapshots)-1]
	assertExpectationPassed(t, last, "alert-a", boolPtr(true))
	assertExpectationPassed(t, last, "alert-b", boolPtr(true))
}

func boolPtr(b bool) *bool { return &b }

func assertExpectationPassed(t *testing.T, results []ExpectationResult, alertName string, want *bool) {
	t.Helper()
	for _, r := range results {
		if r.AlertName != alertName {
			continue
		}
		if want == nil {
			assert.Nil(t, r.Passed, "%s should be pending", alertName)
		} else {
			if assert.NotNil(t, r.Passed, "%s should be resolved", alertName) {
				assert.Equal(t, *want, *r.Passed, "%s passed state", alertName)
			}
		}
		return
	}
	t.Fatalf("expectation %q not found in snapshot", alertName)
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

	r := NewRunner()
	r.Interval = 10 * time.Millisecond
	result := r.Run(&Scenario{
		Name:     "test scenario with injector",
		Injector: mockInjector,
		Matchers: []matchers.AlertGeneratedMatcher{mockMatcher},
		Timeout:  1 * time.Second,
	})
	assert.True(t, result.Success, "the runner should succeed when the injection succeeds")
	assert.Equal(t, "my-injection-uid", result.ExecutionId)

	mockInjector.AssertNumberOfCalls(t, "Inject", 1)
}
