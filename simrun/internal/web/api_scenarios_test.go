package web_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/IBM/simrun/simrun/internal/db"
	"github.com/IBM/simrun/simrun/internal/testutil/testserver"
	"github.com/IBM/simrun/simrun/internal/web"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type scenarioListResponse struct {
	Scenarios []db.SavedScenario `json:"scenarios"`
	Total     int                `json:"total"`
	Page      int                `json:"page"`
	PerPage   int                `json:"perPage"`
}

const sampleYAML = `scenarios:
  - name: lint sample
    detonate:
      awsCliDetonator:
        script: "true"
    expectations:
      - timeout: 1m
        datadogSecuritySignal:
          name: "Test signal"
`

func TestHandleLint_ValidYAML(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/scenarios/lint", web.LintRequest{YAML: sampleYAML})
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got web.LintResponse
	testserver.DecodeJSON(t, resp, &got)
	assert.True(t, got.Valid)
	assert.Empty(t, got.Error)
	require.Len(t, got.Scenarios, 1)
	assert.Equal(t, "lint sample", got.Scenarios[0].Name)
	assert.Equal(t, "detonator", got.Scenarios[0].ExecutorType)
}

func TestHandleLint_InvalidYAML(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/scenarios/lint", web.LintRequest{YAML: "scenarios: [oops"})
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got web.LintResponse
	testserver.DecodeJSON(t, resp, &got)
	assert.False(t, got.Valid)
	assert.NotEmpty(t, got.Error)
}

func TestScenarioCRUD(t *testing.T) {
	ts := testserver.New(t)

	// Create
	resp := ts.Post(t, "/api/scenarios", web.SaveScenarioRequest{
		Name: "my scenario",
		YAML: sampleYAML,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var saved db.SavedScenario
	testserver.DecodeJSON(t, resp, &saved)
	assert.Equal(t, "my scenario", saved.Name)
	assert.Equal(t, web.ScenarioTypeStandard, saved.Type, "should default to standard")
	id := saved.ID

	// List
	resp = ts.Get(t, "/api/scenarios")
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var list scenarioListResponse
	testserver.DecodeJSON(t, resp, &list)
	assert.Len(t, list.Scenarios, 1)
	assert.Equal(t, 1, list.Total)
	assert.Equal(t, 1, list.Page)
	assert.Equal(t, 50, list.PerPage)

	// Get
	resp = ts.Get(t, "/api/scenarios/"+id.String())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got db.SavedScenario
	testserver.DecodeJSON(t, resp, &got)
	assert.Equal(t, "my scenario", got.Name)

	// Update
	resp = ts.Put(t, "/api/scenarios/"+id.String(), web.SaveScenarioRequest{
		Name: "renamed",
		YAML: sampleYAML,
	})
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	resp = ts.Get(t, "/api/scenarios/"+id.String())
	require.Equal(t, http.StatusOK, resp.StatusCode)
	testserver.DecodeJSON(t, resp, &got)
	assert.Equal(t, "renamed", got.Name)

	// Delete
	resp = ts.Delete(t, "/api/scenarios/"+id.String())
	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	resp.Body.Close()

	resp = ts.Get(t, "/api/scenarios/"+id.String())
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	resp.Body.Close()
}

func TestHandleSaveScenario_RejectsInvalidType(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/scenarios", web.SaveScenarioRequest{
		Name: "x",
		Type: "garbage",
		YAML: sampleYAML,
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, testserver.ReadBody(t, resp), "type must be")
}

func TestHandleGetScenario_BadID(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/scenarios/not-a-uuid")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandleGetScenario_NotFound(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/scenarios/"+uuid.New().String())
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleRun_StartsRun(t *testing.T) {
	ts := testserver.New(t)

	// Save a scenario first.
	resp := ts.Post(t, "/api/scenarios", web.SaveScenarioRequest{
		Name: "to run",
		YAML: sampleYAML,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var saved db.SavedScenario
	testserver.DecodeJSON(t, resp, &saved)

	// Run it.
	resp = ts.Post(t, "/api/scenarios/run", web.RunRequest{
		ScenarioID: saved.ID.String(),
	})
	defer resp.Body.Close()
	require.Equal(t, http.StatusAccepted, resp.StatusCode)

	var runResp web.RunResponse
	testserver.DecodeJSON(t, resp, &runResp)
	runID, err := uuid.Parse(runResp.RunID)
	require.NoError(t, err, "expected RunID to be a UUID, got %q", runResp.RunID)

	// A run row should have been created in the store.
	require.Eventually(t, func() bool {
		runs := ts.Stores.Run.All()
		for _, r := range runs {
			if r.ID == runID {
				return true
			}
		}
		return false
	}, eventuallyTimeout, eventuallyTick, "expected run %s to appear in store", runID)
}

func TestHandleRun_BadScenarioID(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/scenarios/run", web.RunRequest{ScenarioID: "not-a-uuid"})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandleRun_MissingScenario(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Post(t, "/api/scenarios/run", web.RunRequest{ScenarioID: uuid.New().String()})
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Contains(t, testserver.ReadBody(t, resp), "scenario not found")
}

func TestHandleListScenarios_Empty(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/scenarios")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got scenarioListResponse
	testserver.DecodeJSON(t, resp, &got)
	assert.Empty(t, got.Scenarios)
	assert.Equal(t, 0, got.Total)
	assert.Equal(t, 1, got.Page)
	assert.Equal(t, 50, got.PerPage)
}

func TestHandleListScenarios_Pagination(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	// Save 5 scenarios; each Save uses time.Now() so the last saved is newest.
	for i := range 5 {
		_, err := ts.Stores.Scenario.Save(ctx, "scen-"+string(rune('a'+i)), web.ScenarioTypeStandard, sampleYAML, "u@x")
		require.NoError(t, err)
	}

	// Page 1, per_page=2 — newest two.
	resp := ts.Get(t, "/api/scenarios?page=1&per_page=2")
	defer resp.Body.Close()
	var got scenarioListResponse
	testserver.DecodeJSON(t, resp, &got)
	require.Len(t, got.Scenarios, 2)
	assert.Equal(t, 5, got.Total)
	assert.Equal(t, 1, got.Page)
	assert.Equal(t, 2, got.PerPage)
	assert.Equal(t, "scen-e", got.Scenarios[0].Name)
	assert.Equal(t, "scen-d", got.Scenarios[1].Name)

	// Page 3, per_page=2 — only "scen-a" left.
	resp2 := ts.Get(t, "/api/scenarios?page=3&per_page=2")
	defer resp2.Body.Close()
	var got2 scenarioListResponse
	testserver.DecodeJSON(t, resp2, &got2)
	require.Len(t, got2.Scenarios, 1)
	assert.Equal(t, 5, got2.Total)
	assert.Equal(t, "scen-a", got2.Scenarios[0].Name)

	// Page beyond range — empty slice, total still reported.
	resp3 := ts.Get(t, "/api/scenarios?page=10&per_page=2")
	defer resp3.Body.Close()
	var got3 scenarioListResponse
	testserver.DecodeJSON(t, resp3, &got3)
	assert.Empty(t, got3.Scenarios)
	assert.Equal(t, 5, got3.Total)
}

func TestHandleListScenarios_PerPageClamped(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/scenarios?per_page=500")
	defer resp.Body.Close()
	var got scenarioListResponse
	testserver.DecodeJSON(t, resp, &got)
	assert.Equal(t, 100, got.PerPage)
}

func TestHandleListScenarios_Filters(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	// Mix of names and types, all updated recently.
	mustSave := func(name, typ string) {
		_, err := ts.Stores.Scenario.Save(ctx, name, typ, sampleYAML, "u@x")
		require.NoError(t, err)
	}
	mustSave("login bruteforce", web.ScenarioTypeStandard)
	mustSave("exfil ransom", web.ScenarioTypeExplore)
	mustSave("audit collect", web.ScenarioTypeCollect)

	// Stash an "old" scenario by editing updated_at in the fake.
	old, err := ts.Stores.Scenario.Save(ctx, "old standard", web.ScenarioTypeStandard, sampleYAML, "u@x")
	require.NoError(t, err)
	ts.Stores.Scenario.SetUpdatedAt(old.ID, time.Now().Add(-72*time.Hour))

	t.Run("name ILIKE", func(t *testing.T) {
		resp := ts.Get(t, "/api/scenarios?name=brute")
		defer resp.Body.Close()
		var got scenarioListResponse
		testserver.DecodeJSON(t, resp, &got)
		require.Len(t, got.Scenarios, 1)
		assert.Equal(t, "login bruteforce", got.Scenarios[0].Name)
	})

	t.Run("multi type", func(t *testing.T) {
		resp := ts.Get(t, "/api/scenarios?type=standard&type=explore")
		defer resp.Body.Close()
		var got scenarioListResponse
		testserver.DecodeJSON(t, resp, &got)
		assert.Equal(t, 3, got.Total) // login bruteforce + exfil ransom + old standard
	})

	t.Run("since window", func(t *testing.T) {
		resp := ts.Get(t, "/api/scenarios?since=24h")
		defer resp.Body.Close()
		var got scenarioListResponse
		testserver.DecodeJSON(t, resp, &got)
		assert.Equal(t, 3, got.Total) // excludes old standard
	})

	t.Run("combined filters", func(t *testing.T) {
		resp := ts.Get(t, "/api/scenarios?name=ransom&type=explore&since=24h")
		defer resp.Body.Close()
		var got scenarioListResponse
		testserver.DecodeJSON(t, resp, &got)
		require.Len(t, got.Scenarios, 1)
		assert.Equal(t, "exfil ransom", got.Scenarios[0].Name)
	})
}

func TestHandleListScenarios_RejectsBadFilters(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/scenarios?type=bogus")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	resp2 := ts.Get(t, "/api/scenarios?since=not-a-duration")
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp2.StatusCode)

	resp3 := ts.Get(t, "/api/scenarios?since=0s")
	defer resp3.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp3.StatusCode)
}
