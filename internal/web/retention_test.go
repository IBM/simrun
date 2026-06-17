package web_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IBM/simrun/internal/db"
	"github.com/IBM/simrun/internal/testutil/fakes"
	"github.com/IBM/simrun/internal/web"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// writeLog writes a run-log JSONL file and back-dates its mtime by ageDays.
func writeLog(t *testing.T, dataDir, runID string, ageDays int) string {
	t.Helper()
	w, err := web.NewRunLogWriter(dataDir, runID)
	require.NoError(t, err)
	w.Write(web.RunLogEntry{Level: "info", Message: "entry"})
	require.NoError(t, w.Close())
	path := filepath.Join(dataDir, "run-logs", runID+".jsonl")
	when := time.Now().AddDate(0, 0, -ageDays)
	require.NoError(t, os.Chtimes(path, when, when))
	return path
}

// The log sweeper expires verbose logs by mtime but must never touch the runs
// row — the assessment summary outlives its logs.
func TestSweepRunLogs_DeletesAgedKeepsRecent(t *testing.T) {
	dataDir := t.TempDir()
	aged := writeLog(t, dataDir, uuid.NewString(), 10)
	recent := writeLog(t, dataDir, uuid.NewString(), 1)

	web.SweepRunLogs(dataDir, true, 7)

	assert.NoFileExists(t, aged, "log older than 7 days should be swept")
	assert.FileExists(t, recent, "log newer than 7 days should be kept")
}

func TestSweepRunLogs_DisabledIsNoOp(t *testing.T) {
	dataDir := t.TempDir()
	aged := writeLog(t, dataDir, uuid.NewString(), 30)

	web.SweepRunLogs(dataDir, false, 7)

	assert.FileExists(t, aged, "disabled sweeper must delete nothing")
}

// makeRun creates a run plus a JSONL log and a collected .ndjson artifact,
// returning the run ID and both file paths.
func makeRun(t *testing.T, ctx context.Context, store *fakes.RunStore, dataDir, status string, ageDays int) (uuid.UUID, string, string) {
	t.Helper()
	id := uuid.New()
	created := time.Now().AddDate(0, 0, -ageDays)
	require.NoError(t, store.Create(ctx, &db.Run{
		ID:        id,
		Status:    status,
		StartTime: created,
		CreatedAt: created,
	}))

	jsonl := writeLog(t, dataDir, id.String(), ageDays)

	ndjson := filepath.Join(dataDir, "collected-"+id.String()+".ndjson")
	require.NoError(t, os.WriteFile(ndjson, []byte(`{"x":1}`+"\n"), 0644))
	require.NoError(t, store.AddScenarioResult(ctx, id, &db.ScenarioResult{
		Name:             "s1",
		CollectedLogPath: &ndjson,
	}))
	return id, jsonl, ndjson
}

// The assessment sweeper purges aged completed runs entirely (row + results +
// JSONL + collected .ndjson), keeps recent runs, and never deletes a run still
// running even if it is old.
func TestSweepAssessments_PurgesAgedSkipsRunningAndRecent(t *testing.T) {
	dataDir := t.TempDir()
	store := fakes.New().Run
	ctx := context.Background()

	oldID, oldJSONL, oldNDJSON := makeRun(t, ctx, store, dataDir, "completed", 40)
	recentID, recentJSONL, _ := makeRun(t, ctx, store, dataDir, "completed", 1)
	runningID, runningJSONL, _ := makeRun(t, ctx, store, dataDir, "running", 40)

	web.SweepAssessments(ctx, store, dataDir, true, 30)

	// Aged completed run: everything gone.
	_, err := store.Get(ctx, oldID)
	assert.Error(t, err, "aged run row should be deleted")
	results, err := store.GetScenarioResults(ctx, oldID)
	require.NoError(t, err)
	assert.Empty(t, results, "aged run results should be cascade-deleted")
	assert.NoFileExists(t, oldJSONL, "aged run JSONL should be removed")
	assert.NoFileExists(t, oldNDJSON, "aged run collected .ndjson should be removed")

	// Recent run: untouched.
	_, err = store.Get(ctx, recentID)
	assert.NoError(t, err, "recent run should be kept")
	assert.FileExists(t, recentJSONL, "recent run JSONL should be kept")

	// Old running run: untouched.
	_, err = store.Get(ctx, runningID)
	assert.NoError(t, err, "running run should be kept even when old")
	assert.FileExists(t, runningJSONL, "running run JSONL should be kept")
}

func TestSweepAssessments_DisabledIsNoOp(t *testing.T) {
	dataDir := t.TempDir()
	store := fakes.New().Run
	ctx := context.Background()

	id, jsonl, ndjson := makeRun(t, ctx, store, dataDir, "completed", 40)

	web.SweepAssessments(ctx, store, dataDir, false, 30)

	_, err := store.Get(ctx, id)
	assert.NoError(t, err, "disabled sweeper must keep the run")
	assert.FileExists(t, jsonl)
	assert.FileExists(t, ndjson)
}
