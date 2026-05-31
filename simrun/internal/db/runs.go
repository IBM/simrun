package db

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// RunStore manages run and scenario result persistence.
type RunStore interface {
	Create(ctx context.Context, run *Run) error
	Get(ctx context.Context, id uuid.UUID) (*Run, error)
	List(ctx context.Context, filters ListRunsFilters, limit, offset int) (RunPage, error)
	Update(ctx context.Context, id uuid.UUID, status string, total, succeeded, failed int, endTime *time.Time) error
	Delete(ctx context.Context, id uuid.UUID) error
	AddScenarioResult(ctx context.Context, runID uuid.UUID, result *ScenarioResult) error
	GetScenarioResults(ctx context.Context, runID uuid.UUID) ([]ScenarioResult, error)
	GetScenarioResult(ctx context.Context, id uuid.UUID) (*ScenarioResult, error)

	// Run lifecycle
	CompleteRun(ctx context.Context, id uuid.UUID, endTime *time.Time) error

	// Scenario status tracking
	CreateScenarioStatus(ctx context.Context, runID uuid.UUID, name string) (uuid.UUID, error)
	UpdateScenarioPhase(ctx context.Context, id uuid.UUID, phase string) error
	CompleteScenarioResult(ctx context.Context, id uuid.UUID, result *ScenarioResult) error
	IncrementRunCounters(ctx context.Context, id uuid.UUID, successDelta, failDelta int) error

	// GetLatestAssertionResults returns the most recent pass/fail for each alert name.
	GetLatestAssertionResults(ctx context.Context) ([]LatestAssertionResult, error)
}

// Run represents a single simrun execution.
type Run struct {
	ID           uuid.UUID  `json:"id"`
	Status       string     `json:"status"`
	StartTime    time.Time  `json:"startTime"`
	EndTime      *time.Time `json:"endTime,omitempty"`
	Total        int        `json:"total"`
	Succeeded    int        `json:"succeeded"`
	Failed       int        `json:"failed"`
	ScenarioID   *uuid.UUID `json:"scenarioId,omitempty"`
	ScenarioName *string    `json:"scenarioName,omitempty"`
	ScenarioType *string    `json:"scenarioType,omitempty"`
	ScheduleID   *uuid.UUID `json:"scheduleId,omitempty"`
	ScheduleName *string    `json:"scheduleName,omitempty"`
	CreatedBy    string     `json:"createdBy"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// ScenarioResult represents the result of a single scenario execution.
type ScenarioResult struct {
	ID                uuid.UUID       `json:"id"`
	RunID             uuid.UUID       `json:"runId"`
	Name              string          `json:"name"`
	Status            string          `json:"status"`
	Phase             *string         `json:"phase,omitempty"`
	IsSuccess         *bool           `json:"isSuccess"`
	ErrorMessage      string          `json:"errorMessage,omitempty"`
	DurationSecs      float64         `json:"durationSecs"`
	MatchingDurSecs   float64         `json:"matchingDurSecs"`
	TimeExecuted      *time.Time      `json:"timeExecuted,omitempty"`
	ExecutorName      string          `json:"executorName"`
	ExecutorType      string          `json:"executorType"`
	ExecutionID       string          `json:"executionId"`
	SimulationID      string          `json:"simulationId,omitempty"`
	Assertions        json.RawMessage `json:"assertions,omitempty"`
	Indicators        json.RawMessage `json:"indicators,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	CollectedLogPath  *string         `json:"collectedLogPath,omitempty"`
	CollectedDocCount int             `json:"collectedDocCount,omitempty"`
	DiscoveredAlerts  json.RawMessage `json:"discoveredAlerts,omitempty"`
	CreatedAt         time.Time       `json:"createdAt"`
}

// RunPage is a paginated slice of runs together with the total row count.
type RunPage struct {
	Runs  []Run `json:"runs"`
	Total int   `json:"total"`
}

// ListRunsFilters narrows the result set for RunStore.List. Zero values mean
// "no constraint on this dimension".
//
// Note: filters that reference saved_scenarios columns (Name, Types) silently
// exclude ad-hoc runs whose scenario_id is NULL, because NULL never matches an
// equality/LIKE predicate.
type ListRunsFilters struct {
	// Name is an ILIKE %name% match against saved_scenarios.name.
	Name string
	// Types restricts saved_scenarios.type to the listed values.
	Types []string
	// Since restricts runs to created_at >= Since.
	Since *time.Time
	// ScenarioID restricts runs to the given saved_scenarios.id.
	ScenarioID *uuid.UUID
}

// LatestAssertionResult holds the most recent pass/fail for a given alert name.
type LatestAssertionResult struct {
	AlertName string    `json:"alertName"`
	Passed    bool      `json:"passed"`
	RunID     uuid.UUID `json:"runId"`
	CreatedAt time.Time `json:"createdAt"`
}

type runStore struct {
	pool *pgxpool.Pool
}

// NewRunStore creates a new RunStore backed by PostgreSQL.
func NewRunStore(pool *pgxpool.Pool) RunStore {
	return &runStore{pool: pool}
}

func (s *runStore) Create(ctx context.Context, run *Run) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO runs (id, status, start_time, total, succeeded, failed, scenario_id, schedule_id, schedule_name, created_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		run.ID, run.Status, run.StartTime, run.Total, run.Succeeded, run.Failed, run.ScenarioID, run.ScheduleID, run.ScheduleName, run.CreatedBy,
	)
	return err
}

func (s *runStore) Get(ctx context.Context, id uuid.UUID) (*Run, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT r.id, r.status, r.start_time, r.end_time, r.total, r.succeeded, r.failed,
				r.scenario_id, ss.name, ss.type,
				r.schedule_id, r.schedule_name, r.created_by, r.created_at
		 FROM runs r
		 LEFT JOIN saved_scenarios ss ON r.scenario_id = ss.id
		 WHERE r.id = $1`, id,
	)
	var run Run
	err := row.Scan(&run.ID, &run.Status, &run.StartTime, &run.EndTime, &run.Total, &run.Succeeded, &run.Failed,
		&run.ScenarioID, &run.ScenarioName, &run.ScenarioType,
		&run.ScheduleID, &run.ScheduleName, &run.CreatedBy, &run.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &run, nil
}

func (s *runStore) List(ctx context.Context, filters ListRunsFilters, limit, offset int) (RunPage, error) {
	where, args := buildRunsWhere(filters)
	rows, err := s.pool.Query(ctx,
		`SELECT r.id, r.status, r.start_time, r.end_time, r.total, r.succeeded, r.failed,
				r.scenario_id, ss.name, ss.type,
				r.schedule_id, r.schedule_name, r.created_by, r.created_at,
				COUNT(*) OVER() AS total_count
		 FROM runs r
		 LEFT JOIN saved_scenarios ss ON r.scenario_id = ss.id
		 `+where+`
		 ORDER BY r.created_at DESC
		 LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, limit, offset)...,
	)
	if err != nil {
		return RunPage{}, err
	}
	defer rows.Close()

	page := RunPage{Runs: []Run{}}
	for rows.Next() {
		var run Run
		var total int
		if err := rows.Scan(&run.ID, &run.Status, &run.StartTime, &run.EndTime, &run.Total, &run.Succeeded, &run.Failed,
			&run.ScenarioID, &run.ScenarioName, &run.ScenarioType,
			&run.ScheduleID, &run.ScheduleName, &run.CreatedBy, &run.CreatedAt, &total); err != nil {
			return RunPage{}, err
		}
		page.Runs = append(page.Runs, run)
		page.Total = total
	}
	if err := rows.Err(); err != nil {
		return RunPage{}, err
	}
	if len(page.Runs) == 0 {
		// COUNT(*) OVER() collapses to no rows when LIMIT/OFFSET yields nothing.
		// Re-run the same filter as a plain COUNT so the UI can show "of N".
		countSQL := `SELECT COUNT(*) FROM runs r LEFT JOIN saved_scenarios ss ON r.scenario_id = ss.id ` + where
		if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&page.Total); err != nil {
			return RunPage{}, err
		}
	}
	return page, nil
}

// buildRunsWhere returns a WHERE clause (or "") and its positional args for the
// runs+saved_scenarios join. Placeholders are $1..$N in argument order.
func buildRunsWhere(f ListRunsFilters) (string, []any) {
	var clauses []string
	var args []any
	if f.Name != "" {
		args = append(args, "%"+f.Name+"%")
		clauses = append(clauses, "ss.name ILIKE $"+strconv.Itoa(len(args)))
	}
	if len(f.Types) > 0 {
		placeholders := make([]string, len(f.Types))
		for i, t := range f.Types {
			args = append(args, t)
			placeholders[i] = "$" + strconv.Itoa(len(args))
		}
		clauses = append(clauses, "ss.type IN ("+strings.Join(placeholders, ",")+")")
	}
	if f.Since != nil {
		args = append(args, *f.Since)
		clauses = append(clauses, "r.created_at >= $"+strconv.Itoa(len(args)))
	}
	if f.ScenarioID != nil {
		args = append(args, *f.ScenarioID)
		clauses = append(clauses, "r.scenario_id = $"+strconv.Itoa(len(args)))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (s *runStore) Update(ctx context.Context, id uuid.UUID, status string, total, succeeded, failed int, endTime *time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE runs SET status = $2, total = $3, succeeded = $4, failed = $5, end_time = $6
		 WHERE id = $1`,
		id, status, total, succeeded, failed, endTime,
	)
	return err
}

func (s *runStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM runs WHERE id = $1`, id)
	return err
}

func (s *runStore) AddScenarioResult(ctx context.Context, runID uuid.UUID, result *ScenarioResult) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO scenario_results (run_id, name, status, is_success, error_message, duration_secs, matching_dur_secs, time_executed, executor_name, executor_type, execution_id, simulation_id, assertions, indicators, metadata, collected_log_path, collected_doc_count, discovered_alerts)
		 VALUES ($1, $2, 'completed', $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		runID, result.Name, result.IsSuccess, result.ErrorMessage, result.DurationSecs,
		result.MatchingDurSecs, result.TimeExecuted, result.ExecutorName, result.ExecutorType,
		result.ExecutionID, result.SimulationID, result.Assertions, result.Indicators, result.Metadata,
		result.CollectedLogPath, result.CollectedDocCount, result.DiscoveredAlerts,
	)
	return err
}

func (s *runStore) GetScenarioResults(ctx context.Context, runID uuid.UUID) ([]ScenarioResult, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, run_id, name, status, phase, is_success,
			COALESCE(error_message, ''), COALESCE(duration_secs, 0), COALESCE(matching_dur_secs, 0),
			time_executed,
			COALESCE(executor_name, ''), COALESCE(executor_type, ''), COALESCE(execution_id, ''), COALESCE(simulation_id, ''),
			assertions, indicators, metadata, collected_log_path, COALESCE(collected_doc_count, 0), discovered_alerts, created_at
		 FROM scenario_results WHERE run_id = $1 ORDER BY created_at`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	results := []ScenarioResult{}
	for rows.Next() {
		var r ScenarioResult
		if err := rows.Scan(&r.ID, &r.RunID, &r.Name, &r.Status, &r.Phase, &r.IsSuccess, &r.ErrorMessage,
			&r.DurationSecs, &r.MatchingDurSecs, &r.TimeExecuted, &r.ExecutorName,
			&r.ExecutorType, &r.ExecutionID, &r.SimulationID, &r.Assertions, &r.Indicators, &r.Metadata,
			&r.CollectedLogPath, &r.CollectedDocCount, &r.DiscoveredAlerts, &r.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (s *runStore) GetScenarioResult(ctx context.Context, id uuid.UUID) (*ScenarioResult, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, run_id, name, status, phase, is_success,
			COALESCE(error_message, ''), COALESCE(duration_secs, 0), COALESCE(matching_dur_secs, 0),
			time_executed,
			COALESCE(executor_name, ''), COALESCE(executor_type, ''), COALESCE(execution_id, ''), COALESCE(simulation_id, ''),
			assertions, indicators, metadata, collected_log_path, COALESCE(collected_doc_count, 0), discovered_alerts, created_at
		 FROM scenario_results WHERE id = $1`, id,
	)
	var r ScenarioResult
	err := row.Scan(&r.ID, &r.RunID, &r.Name, &r.Status, &r.Phase, &r.IsSuccess, &r.ErrorMessage,
		&r.DurationSecs, &r.MatchingDurSecs, &r.TimeExecuted, &r.ExecutorName,
		&r.ExecutorType, &r.ExecutionID, &r.SimulationID, &r.Assertions, &r.Indicators, &r.Metadata,
		&r.CollectedLogPath, &r.CollectedDocCount, &r.DiscoveredAlerts, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *runStore) CreateScenarioStatus(ctx context.Context, runID uuid.UUID, name string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.pool.Exec(ctx,
		`INSERT INTO scenario_results (id, run_id, name, status) VALUES ($1, $2, $3, 'pending')`,
		id, runID, name,
	)
	return id, err
}

func (s *runStore) UpdateScenarioPhase(ctx context.Context, id uuid.UUID, phase string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE scenario_results SET status = 'running', phase = $2 WHERE id = $1`,
		id, phase,
	)
	return err
}

func (s *runStore) CompleteScenarioResult(ctx context.Context, id uuid.UUID, result *ScenarioResult) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE scenario_results SET
			status = 'completed', phase = NULL,
			is_success = $2, error_message = $3, duration_secs = $4, matching_dur_secs = $5,
			time_executed = $6, executor_name = $7, executor_type = $8, execution_id = $9,
			simulation_id = $10, assertions = $11, indicators = $12, metadata = $13,
			collected_log_path = $14, collected_doc_count = $15, discovered_alerts = $16
		 WHERE id = $1`,
		id, result.IsSuccess, result.ErrorMessage, result.DurationSecs, result.MatchingDurSecs,
		result.TimeExecuted, result.ExecutorName, result.ExecutorType, result.ExecutionID,
		result.SimulationID, result.Assertions, result.Indicators, result.Metadata,
		result.CollectedLogPath, result.CollectedDocCount, result.DiscoveredAlerts,
	)
	return err
}

func (s *runStore) IncrementRunCounters(ctx context.Context, id uuid.UUID, successDelta, failDelta int) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE runs SET succeeded = succeeded + $2, failed = failed + $3 WHERE id = $1`,
		id, successDelta, failDelta,
	)
	return err
}

func (s *runStore) CompleteRun(ctx context.Context, id uuid.UUID, endTime *time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE runs SET status = 'completed', end_time = $2 WHERE id = $1`,
		id, endTime,
	)
	return err
}

func (s *runStore) GetLatestAssertionResults(ctx context.Context) ([]LatestAssertionResult, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT ON (a.value->>'alertName')
			a.value->>'alertName' AS alert_name,
			(a.value->>'passed')::boolean AS passed,
			sr.run_id,
			sr.created_at
		FROM scenario_results sr,
			jsonb_array_elements(sr.assertions) AS a(value)
		WHERE sr.status = 'completed'
			AND sr.assertions IS NOT NULL
			AND a.value->>'matcherType' = 'Elastic Security alert'
		ORDER BY a.value->>'alertName', sr.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []LatestAssertionResult
	for rows.Next() {
		var r LatestAssertionResult
		if err := rows.Scan(&r.AlertName, &r.Passed, &r.RunID, &r.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

