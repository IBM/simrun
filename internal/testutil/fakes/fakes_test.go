package fakes

import (
	"context"
	"testing"
	"time"

	"github.com/IBM/simrun/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ListExpired is the assessment-retention sweeper's deletion query: it must
// return only runs created before the cutoff and must never return a run that
// is still running, even if old. (The production SQL is Postgres-bound and
// verified manually per the change's task 7.3; this pins the shared contract.)
func TestRunStore_ListExpired(t *testing.T) {
	s := New().Run
	ctx := context.Background()
	cutoff := time.Now().Add(-7 * 24 * time.Hour)

	old := mustCreateRun(t, ctx, s, "completed", cutoff.Add(-time.Hour))
	recent := mustCreateRun(t, ctx, s, "completed", cutoff.Add(time.Hour))
	oldRunning := mustCreateRun(t, ctx, s, "running", cutoff.Add(-time.Hour))

	ids, err := s.ListExpired(ctx, cutoff)
	require.NoError(t, err)

	assert.Contains(t, ids, old, "old completed run should be expired")
	assert.NotContains(t, ids, recent, "run newer than cutoff must be kept")
	assert.NotContains(t, ids, oldRunning, "running run must be skipped even when old")
}

// UpdateScenarioIdentity and UpdateScenarioAssertions are the mid-run partial
// writes behind the live scenario detail view: each must touch only its own
// columns and leave the lifecycle status/phase untouched. (Production SQL is
// Postgres-bound and verified manually per task 5.3; this pins the contract the
// web wiring relies on.)
func TestRunStore_PartialScenarioUpdates(t *testing.T) {
	s := New().Run
	ctx := context.Background()
	runID := mustCreateRun(t, ctx, s, "running", time.Now())

	id, err := s.CreateScenarioStatus(ctx, runID, "scn")
	require.NoError(t, err)
	require.NoError(t, s.UpdateScenarioPhase(ctx, id, "matching"))

	// Identity write: only identity columns change; status/phase preserved.
	require.NoError(t, s.UpdateScenarioIdentity(ctx, id, "elastic-detonator", "detonator", "exec-9", "sim-9"))
	got, err := s.GetScenarioResult(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "running", got.Status, "identity write must not change status")
	require.NotNil(t, got.Phase)
	assert.Equal(t, "matching", *got.Phase, "identity write must not change phase")
	assert.Equal(t, "elastic-detonator", got.ExecutorName)
	assert.Equal(t, "detonator", got.ExecutorType)
	assert.Equal(t, "exec-9", got.ExecutionID)
	assert.Equal(t, "sim-9", got.SimulationID)
	assert.Nil(t, got.Assertions, "identity write must not populate assertions")

	// Assertions write: only the assertions column changes; everything else stays.
	partial := []byte(`[{"matcherType":"Elastic","alertName":"a","passed":true},{"matcherType":"Elastic","alertName":"b"}]`)
	require.NoError(t, s.UpdateScenarioAssertions(ctx, id, partial))
	got, err = s.GetScenarioResult(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, "running", got.Status, "assertions write must not change status")
	require.NotNil(t, got.Phase)
	assert.Equal(t, "matching", *got.Phase, "assertions write must not change phase")
	assert.Equal(t, "exec-9", got.ExecutionID, "assertions write must not touch identity")
	assert.JSONEq(t, string(partial), string(got.Assertions))
}

func mustCreateRun(t *testing.T, ctx context.Context, s *RunStore, status string, createdAt time.Time) uuid.UUID {
	t.Helper()
	id := uuid.New()
	require.NoError(t, s.Create(ctx, &db.Run{
		ID:        id,
		Status:    status,
		StartTime: createdAt,
		CreatedAt: createdAt,
	}))
	return id
}
