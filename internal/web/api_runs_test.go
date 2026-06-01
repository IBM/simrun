package web_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/testutil/testserver"
	"github.com/IBM/simrun/internal/web"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runListResponse struct {
	Runs    []db.Run `json:"runs"`
	Total   int      `json:"total"`
	Page    int      `json:"page"`
	PerPage int      `json:"perPage"`
}

func TestHandleListRuns_Empty(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/runs")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got runListResponse
	testserver.DecodeJSON(t, resp, &got)
	assert.Empty(t, got.Runs)
	assert.Equal(t, 0, got.Total)
	assert.Equal(t, 1, got.Page)
	assert.Equal(t, 50, got.PerPage)
}

func TestHandleListRuns_PopulatedFromStore(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	id := uuid.New()
	require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
		ID:        id,
		Status:    "running",
		StartTime: time.Now(),
		CreatedAt: time.Now(),
	}))

	resp := ts.Get(t, "/api/runs")
	defer resp.Body.Close()

	var got runListResponse
	testserver.DecodeJSON(t, resp, &got)
	require.Len(t, got.Runs, 1)
	assert.Equal(t, id, got.Runs[0].ID)
	assert.Equal(t, 1, got.Total)
}

func TestHandleListRuns_Pagination(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	// Create 5 runs with increasing CreatedAt so the newest is index 4.
	base := time.Now()
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = uuid.New()
		require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
			ID:        ids[i],
			Status:    "completed",
			StartTime: base.Add(time.Duration(i) * time.Second),
			CreatedAt: base.Add(time.Duration(i) * time.Second),
		}))
	}

	// Page 1, per_page=2 — should return the two newest (ids[4], ids[3]).
	resp := ts.Get(t, "/api/runs?page=1&per_page=2")
	defer resp.Body.Close()

	var p1 runListResponse
	testserver.DecodeJSON(t, resp, &p1)
	require.Len(t, p1.Runs, 2)
	assert.Equal(t, ids[4], p1.Runs[0].ID)
	assert.Equal(t, ids[3], p1.Runs[1].ID)
	assert.Equal(t, 5, p1.Total)
	assert.Equal(t, 2, p1.PerPage)

	// Page 3, per_page=2 — should return only ids[0] (the last page with one item).
	resp2 := ts.Get(t, "/api/runs?page=3&per_page=2")
	defer resp2.Body.Close()

	var p3 runListResponse
	testserver.DecodeJSON(t, resp2, &p3)
	require.Len(t, p3.Runs, 1)
	assert.Equal(t, ids[0], p3.Runs[0].ID)
	assert.Equal(t, 5, p3.Total)
	assert.Equal(t, 3, p3.Page)

	// Page past end — empty rows but total still reported.
	resp3 := ts.Get(t, "/api/runs?page=10&per_page=2")
	defer resp3.Body.Close()

	var pPast runListResponse
	testserver.DecodeJSON(t, resp3, &pPast)
	assert.Empty(t, pPast.Runs)
	assert.Equal(t, 5, pPast.Total)
}

func TestHandleListRuns_PerPageClamped(t *testing.T) {
	ts := testserver.New(t)

	// per_page=500 should clamp to 100.
	resp := ts.Get(t, "/api/runs?per_page=500")
	defer resp.Body.Close()

	var got runListResponse
	testserver.DecodeJSON(t, resp, &got)
	assert.Equal(t, 100, got.PerPage)
}

func TestHandleListRuns_Filters(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	mk := func(name, typ string, age time.Duration) uuid.UUID {
		id := uuid.New()
		n, tp := name, typ
		now := time.Now().Add(-age)
		require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
			ID:           id,
			Status:       "completed",
			StartTime:    now,
			CreatedAt:    now,
			ScenarioName: &n,
			ScenarioType: &tp,
		}))
		return id
	}

	oldStd := mk("data exfil", "standard", 8*24*time.Hour)     // 8 days old
	recentStd := mk("ransomware-sim", "standard", 2*time.Hour) // 2h old
	recentExp := mk("ransomware-sim", "explore", 2*time.Hour)  // 2h old
	recentCol := mk("logs-collect", "collect", 2*time.Hour)    // 2h old

	cases := []struct {
		query string
		want  []uuid.UUID
	}{
		{"name=ransom", []uuid.UUID{recentStd, recentExp}},
		{"type=collect", []uuid.UUID{recentCol}},
		{"type=standard&type=explore", []uuid.UUID{oldStd, recentStd, recentExp}},
		{"since=24h", []uuid.UUID{recentStd, recentExp, recentCol}},
		{"since=24h&name=ransom&type=explore", []uuid.UUID{recentExp}},
	}

	// "8 days old" sanity check: with no filters it shows up.
	{
		resp := ts.Get(t, "/api/runs")
		defer resp.Body.Close()
		var got runListResponse
		testserver.DecodeJSON(t, resp, &got)
		require.Equal(t, 4, got.Total)
		// Verify oldStd is in the unfiltered result.
		found := false
		for _, r := range got.Runs {
			if r.ID == oldStd {
				found = true
			}
		}
		assert.True(t, found, "oldStd should appear in unfiltered list")
	}

	for _, tc := range cases {
		t.Run(tc.query, func(t *testing.T) {
			resp := ts.Get(t, "/api/runs?"+tc.query)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)

			var got runListResponse
			testserver.DecodeJSON(t, resp, &got)
			ids := make([]uuid.UUID, len(got.Runs))
			for i, r := range got.Runs {
				ids[i] = r.ID
			}
			assert.ElementsMatch(t, tc.want, ids)
			assert.Equal(t, len(tc.want), got.Total)
		})
	}
}

// TestHandleListRuns_FiltersExcludeUnlinkedRuns pins the intentional behavior:
// runs that aren't backed by a saved_scenarios row (no ScenarioName/Type)
// cannot match a name or type filter, since those columns are NULL.
func TestHandleListRuns_FiltersExcludeUnlinkedRuns(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	// One run with no linked scenario (ad-hoc).
	adhocID := uuid.New()
	require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
		ID:        adhocID,
		Status:    "completed",
		StartTime: time.Now(),
		CreatedAt: time.Now(),
	}))

	// One run with a saved scenario.
	name, typ := "linked-scenario", "standard"
	linkedID := uuid.New()
	require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
		ID:           linkedID,
		Status:       "completed",
		StartTime:    time.Now(),
		CreatedAt:    time.Now(),
		ScenarioName: &name,
		ScenarioType: &typ,
	}))

	// Unfiltered: both visible.
	resp := ts.Get(t, "/api/runs")
	defer resp.Body.Close()
	var all runListResponse
	testserver.DecodeJSON(t, resp, &all)
	assert.Equal(t, 2, all.Total)

	// Filter by name: only the linked run matches; ad-hoc is excluded.
	resp2 := ts.Get(t, "/api/runs?name=linked")
	defer resp2.Body.Close()
	var byName runListResponse
	testserver.DecodeJSON(t, resp2, &byName)
	require.Len(t, byName.Runs, 1)
	assert.Equal(t, linkedID, byName.Runs[0].ID)
	assert.Equal(t, 1, byName.Total)

	// Filter by type: same — ad-hoc excluded.
	resp3 := ts.Get(t, "/api/runs?type=standard")
	defer resp3.Body.Close()
	var byType runListResponse
	testserver.DecodeJSON(t, resp3, &byType)
	require.Len(t, byType.Runs, 1)
	assert.Equal(t, linkedID, byType.Runs[0].ID)
	assert.Equal(t, 1, byType.Total)
}

func TestHandleListRuns_FilterValidation(t *testing.T) {
	ts := testserver.New(t)

	// Invalid type rejected.
	resp := ts.Get(t, "/api/runs?type=bogus")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	// Invalid since duration rejected.
	resp2 := ts.Get(t, "/api/runs?since=not-a-duration")
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp2.StatusCode)

	// Non-positive since rejected.
	resp3 := ts.Get(t, "/api/runs?since=0s")
	defer resp3.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp3.StatusCode)

	// Invalid scenario_id rejected.
	resp4 := ts.Get(t, "/api/runs?scenario_id=not-a-uuid")
	defer resp4.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp4.StatusCode)
}

func TestHandleListRuns_FilterByScenarioID(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	scenarioA := uuid.New()
	scenarioB := uuid.New()

	want := uuid.New()
	require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
		ID:         want,
		Status:     "completed",
		StartTime:  time.Now(),
		CreatedAt:  time.Now(),
		ScenarioID: &scenarioA,
	}))
	// Other scenario — must be filtered out.
	require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
		ID:         uuid.New(),
		Status:     "completed",
		StartTime:  time.Now(),
		CreatedAt:  time.Now(),
		ScenarioID: &scenarioB,
	}))
	// Ad-hoc run with no scenario — must also be filtered out.
	require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
		ID:        uuid.New(),
		Status:    "completed",
		StartTime: time.Now(),
		CreatedAt: time.Now(),
	}))

	resp := ts.Get(t, "/api/runs?scenario_id="+scenarioA.String())
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got runListResponse
	testserver.DecodeJSON(t, resp, &got)
	require.Len(t, got.Runs, 1)
	assert.Equal(t, want, got.Runs[0].ID)
	assert.Equal(t, 1, got.Total)
}

func TestHandleGetRun(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	id := uuid.New()
	require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
		ID:        id,
		Status:    "completed",
		StartTime: time.Now(),
		CreatedAt: time.Now(),
		Total:     1,
	}))

	resp := ts.Get(t, "/api/runs/"+id.String())
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got map[string]any
	testserver.DecodeJSON(t, resp, &got)
	assert.NotNil(t, got["run"])
	assert.NotNil(t, got["scenarios"])
}

func TestHandleGetRun_NotFound(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/runs/"+uuid.New().String())
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestHandleGetRun_BadID(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/runs/not-a-uuid")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestHandleDeleteRun(t *testing.T) {
	ts := testserver.New(t)
	ctx := t.Context()

	id := uuid.New()
	require.NoError(t, ts.Stores.Run.Create(ctx, &db.Run{
		ID:        id,
		Status:    "completed",
		StartTime: time.Now(),
		CreatedAt: time.Now(),
	}))

	resp := ts.Delete(t, "/api/runs/"+id.String())
	defer resp.Body.Close()
	require.Equal(t, http.StatusNoContent, resp.StatusCode)

	got, err := ts.Stores.Run.Get(ctx, id)
	assert.Error(t, err, "expected ErrNoRows after delete")
	assert.Nil(t, got)
}

func TestHandleGetRunLogs_NoLogs(t *testing.T) {
	ts := testserver.New(t)

	resp := ts.Get(t, "/api/runs/"+uuid.New().String()+"/logs")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	// Empty file/missing → empty list (handler is permissive).
	assert.Contains(t, []string{"null\n", "[]\n"}, testserver.ReadBody(t, resp))
}

func TestHandleGetRunLogs_WithEntries(t *testing.T) {
	ts := testserver.New(t)
	runID := uuid.New().String()

	w, err := web.NewRunLogWriter(ts.DataDir, runID)
	require.NoError(t, err)
	now := time.Now().UTC().Format(time.RFC3339)
	w.Write(web.RunLogEntry{Timestamp: now, Level: "info", Message: "scenario started"})
	w.Write(web.RunLogEntry{Timestamp: now, Level: "info", Message: "scenario completed"})
	require.NoError(t, w.Close())

	resp := ts.Get(t, "/api/runs/"+runID+"/logs")
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var entries []web.RunLogEntry
	testserver.DecodeJSON(t, resp, &entries)
	require.Len(t, entries, 2)
	assert.Equal(t, "scenario started", entries[0].Message)
	assert.Equal(t, "scenario completed", entries[1].Message)
}
