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
