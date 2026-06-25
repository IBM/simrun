package web_test

import (
	"net/http"
	"testing"

	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/testutil/testserver"
	"github.com/IBM/simrun/internal/web"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func saveScenario(t *testing.T, ts *testserver.TS) db.Assessment {
	t.Helper()
	resp := ts.Post(t, "/api/assessments", web.SaveAssessmentRequest{
		Name: "scheduled scenario",
		YAML: sampleYAML,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var saved db.Assessment
	testserver.DecodeJSON(t, resp, &saved)
	return saved
}

func TestScheduleCRUD(t *testing.T) {
	ts := testserver.New(t)
	scenario := saveScenario(t, ts)

	// Create
	resp := ts.Post(t, "/api/schedules", web.CreateScheduleRequest{
		AssessmentID:   scenario.ID.String(),
		CronExpression: "0 * * * *",
		Enabled:        true,
		Parallelism:    5,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var sched db.Schedule
	testserver.DecodeJSON(t, resp, &sched)
	assert.Equal(t, scenario.ID, sched.AssessmentID)
	assert.Equal(t, "0 * * * *", sched.CronExpression)
	assert.Equal(t, 5, sched.Parallelism)

	// List
	resp = ts.Get(t, "/api/schedules")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list []db.Schedule
	testserver.DecodeJSON(t, resp, &list)
	assert.Len(t, list, 1)

	// Get
	resp = ts.Get(t, "/api/schedules/"+sched.ID.String())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// GetByScenario
	resp = ts.Get(t, "/api/assessments/"+scenario.ID.String()+"/schedule")
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Update
	resp = ts.Put(t, "/api/schedules/"+sched.ID.String(), web.UpdateScheduleRequest{
		CronExpression: "*/30 * * * *",
		Enabled:        false,
		Parallelism:    8,
	})
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	updated, err := ts.Stores.Schedule.Get(t.Context(), sched.ID)
	require.NoError(t, err)
	assert.Equal(t, "*/30 * * * *", updated.CronExpression)
	assert.False(t, updated.Enabled)

	// Delete
	resp = ts.Delete(t, "/api/schedules/"+sched.ID.String())
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	_, err = ts.Stores.Schedule.Get(t.Context(), sched.ID)
	assert.Error(t, err)
}

func TestHandleCreateSchedule_RejectsInvalidCron(t *testing.T) {
	ts := testserver.New(t)
	scenario := saveScenario(t, ts)

	resp := ts.Post(t, "/api/schedules", web.CreateScheduleRequest{
		AssessmentID:   scenario.ID.String(),
		CronExpression: "garbage",
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, testserver.ReadBody(t, resp), "invalid cron expression")
}

func TestHandleCreateSchedule_MissingScenario(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/schedules", web.CreateScheduleRequest{
		AssessmentID:   uuid.New().String(),
		CronExpression: "0 * * * *",
	})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleGetConfig_Empty(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/config")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got map[string]any
	testserver.DecodeJSON(t, resp, &got)
	assert.Empty(t, got)
}

func TestHandleUpdateConfig(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Put(t, "/api/config", web.UpdateConfigRequest{
		Key:   "parallelism",
		Value: []byte(`12`),
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	cfg, err := ts.Stores.Config.GetAppConfig(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 12, cfg.Parallelism)
}
