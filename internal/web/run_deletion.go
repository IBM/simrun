package web

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IBM/simrun/internal/db"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

// deleteRunWithArtifacts removes a run's database row (cascading to its
// scenario_results via FK) and best-effort removes all of its on-disk
// artifacts: the per-run JSONL log file, every collected .ndjson file
// referenced by scenario_results.collected_log_path, and each scenario
// result's Terraform working directory at <dataDir>/terraform/<execution_id>/.
//
// Collected paths and execution IDs are read before the row is deleted, since
// the cascade removes the scenario_results that hold them. On-disk removal is
// best-effort: a missing or unremovable file logs a warning but does not fail
// the delete, so a leftover artifact never blocks reclaiming the database row.
//
// Shared by manual delete (HandleDeleteRun) and the assessment-retention
// sweeper so both reclaim the large collected .ndjson artifacts identically.
func deleteRunWithArtifacts(ctx context.Context, runStore db.RunStore, dataDir string, runID uuid.UUID) error {
	// Collect the .ndjson paths and execution IDs before deleting, while the
	// rows still exist.
	var collected []string
	var executionIDs []string
	results, err := runStore.GetScenarioResults(ctx, runID)
	if err != nil {
		// A failed lookup must not strand the row: log and continue to delete.
		logrus.WithError(err).WithField("run_id", runID).
			Warn("failed to load scenario results for artifact cleanup; deleting run anyway")
	} else {
		for _, res := range results {
			if res.CollectedLogPath != nil && *res.CollectedLogPath != "" {
				collected = append(collected, *res.CollectedLogPath)
			}
			executionIDs = append(executionIDs, res.ExecutionID)
		}
	}

	if err := runStore.Delete(ctx, runID); err != nil {
		return err
	}

	// Best-effort: remove the JSONL log file.
	DeleteRunLog(dataDir, runID.String())

	// Best-effort: remove each collected .ndjson artifact.
	for _, p := range collected {
		clean := filepath.Clean(p)
		if filepath.Ext(clean) != ".ndjson" {
			// Guard against removing anything that isn't a collected log file,
			// mirroring the download handler's path check.
			logrus.WithField("path", p).WithField("run_id", runID).
				Warn("skipping removal of non-.ndjson collected log path")
			continue
		}
		if err := os.Remove(clean); err != nil && !os.IsNotExist(err) {
			logrus.WithError(err).WithField("path", clean).WithField("run_id", runID).
				Warn("failed to remove collected log file")
		}
	}

	// Best-effort: remove each scenario result's Terraform working directory at
	// <dataDir>/terraform/<execution_id>/. Skip blank/whitespace or path-bearing
	// IDs so cleanup can never escape or wipe the terraform/ base directory.
	for _, id := range executionIDs {
		if strings.TrimSpace(id) == "" ||
			strings.ContainsRune(id, '/') || strings.ContainsRune(id, filepath.Separator) {
			continue
		}
		dir := filepath.Join(dataDir, "terraform", id)
		if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
			logrus.WithError(err).WithField("path", dir).WithField("run_id", runID).
				Warn("failed to remove Terraform working directory")
		}
	}

	return nil
}

// SweepAssessments deletes whole runs (row + scenario_results + JSONL log +
// collected .ndjson artifacts) whose created_at is older than days. It is a
// no-op when enabled is false. Runs still in the "running" status are excluded
// by ListExpired, so an actively-writing run is never purged. A per-run delete
// failure is logged and the sweep continues with the remaining runs.
func SweepAssessments(ctx context.Context, runStore db.RunStore, dataDir string, enabled bool, days int) {
	if !enabled {
		return
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	ids, err := runStore.ListExpired(ctx, cutoff)
	if err != nil {
		logrus.WithError(err).Warn("assessment sweep: failed to list expired runs")
		return
	}

	for _, id := range ids {
		if err := deleteRunWithArtifacts(ctx, runStore, dataDir, id); err != nil {
			logrus.WithError(err).WithField("run_id", id).Warn("assessment sweep: failed to delete expired run")
		}
	}
}
