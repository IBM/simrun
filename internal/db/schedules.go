package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ScheduleStore manages schedule persistence.
type ScheduleStore interface {
	Create(ctx context.Context, assessmentID uuid.UUID, cronExpr string, enabled bool, parallelism int, createdBy string) (*Schedule, error)
	Get(ctx context.Context, id uuid.UUID) (*Schedule, error)
	GetByAssessmentID(ctx context.Context, assessmentID uuid.UUID) (*Schedule, error)
	List(ctx context.Context) ([]Schedule, error)
	ListEnabled(ctx context.Context) ([]Schedule, error)
	Update(ctx context.Context, id uuid.UUID, cronExpr string, enabled bool, parallelism int, updatedBy string) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateLastRun(ctx context.Context, id uuid.UUID, lastRunAt time.Time) error
}

// Schedule represents a cron schedule for an assessment.
type Schedule struct {
	ID             uuid.UUID  `json:"id"`
	AssessmentID   uuid.UUID  `json:"assessmentId"`
	CronExpression string     `json:"cronExpression"`
	Enabled        bool       `json:"enabled"`
	Parallelism    int        `json:"parallelism"`
	LastRunAt      *time.Time `json:"lastRunAt,omitempty"`
	CreatedBy      string     `json:"createdBy"`
	UpdatedBy      string     `json:"updatedBy"`
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

type scheduleStore struct {
	pool *pgxpool.Pool
}

// NewScheduleStore creates a new ScheduleStore backed by PostgreSQL.
func NewScheduleStore(pool *pgxpool.Pool) ScheduleStore {
	return &scheduleStore{pool: pool}
}

func (s *scheduleStore) Create(ctx context.Context, assessmentID uuid.UUID, cronExpr string, enabled bool, parallelism int, createdBy string) (*Schedule, error) {
	var sch Schedule
	err := s.pool.QueryRow(ctx,
		`INSERT INTO schedules (assessment_id, cron_expression, enabled, parallelism, created_by, updated_by)
		 VALUES ($1, $2, $3, $4, $5, $5)
		 RETURNING id, assessment_id, cron_expression, enabled, parallelism, last_run_at, created_by, updated_by, created_at, updated_at`,
		assessmentID, cronExpr, enabled, parallelism, createdBy,
	).Scan(&sch.ID, &sch.AssessmentID, &sch.CronExpression, &sch.Enabled, &sch.Parallelism, &sch.LastRunAt, &sch.CreatedBy, &sch.UpdatedBy, &sch.CreatedAt, &sch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sch, nil
}

func (s *scheduleStore) Get(ctx context.Context, id uuid.UUID) (*Schedule, error) {
	var sch Schedule
	err := s.pool.QueryRow(ctx,
		`SELECT id, assessment_id, cron_expression, enabled, parallelism, last_run_at, created_by, updated_by, created_at, updated_at
		 FROM schedules WHERE id = $1`, id,
	).Scan(&sch.ID, &sch.AssessmentID, &sch.CronExpression, &sch.Enabled, &sch.Parallelism, &sch.LastRunAt, &sch.CreatedBy, &sch.UpdatedBy, &sch.CreatedAt, &sch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sch, nil
}

func (s *scheduleStore) GetByAssessmentID(ctx context.Context, assessmentID uuid.UUID) (*Schedule, error) {
	var sch Schedule
	err := s.pool.QueryRow(ctx,
		`SELECT id, assessment_id, cron_expression, enabled, parallelism, last_run_at, created_by, updated_by, created_at, updated_at
		 FROM schedules WHERE assessment_id = $1`, assessmentID,
	).Scan(&sch.ID, &sch.AssessmentID, &sch.CronExpression, &sch.Enabled, &sch.Parallelism, &sch.LastRunAt, &sch.CreatedBy, &sch.UpdatedBy, &sch.CreatedAt, &sch.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sch, nil
}

func (s *scheduleStore) List(ctx context.Context) ([]Schedule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, assessment_id, cron_expression, enabled, parallelism, last_run_at, created_by, updated_by, created_at, updated_at
		 FROM schedules ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules := []Schedule{}
	for rows.Next() {
		var sch Schedule
		if err := rows.Scan(&sch.ID, &sch.AssessmentID, &sch.CronExpression, &sch.Enabled, &sch.Parallelism, &sch.LastRunAt, &sch.CreatedBy, &sch.UpdatedBy, &sch.CreatedAt, &sch.UpdatedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, sch)
	}
	return schedules, rows.Err()
}

func (s *scheduleStore) ListEnabled(ctx context.Context) ([]Schedule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, assessment_id, cron_expression, enabled, parallelism, last_run_at, created_by, updated_by, created_at, updated_at
		 FROM schedules WHERE enabled = true ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	schedules := []Schedule{}
	for rows.Next() {
		var sch Schedule
		if err := rows.Scan(&sch.ID, &sch.AssessmentID, &sch.CronExpression, &sch.Enabled, &sch.Parallelism, &sch.LastRunAt, &sch.CreatedBy, &sch.UpdatedBy, &sch.CreatedAt, &sch.UpdatedAt); err != nil {
			return nil, err
		}
		schedules = append(schedules, sch)
	}
	return schedules, rows.Err()
}

func (s *scheduleStore) Update(ctx context.Context, id uuid.UUID, cronExpr string, enabled bool, parallelism int, updatedBy string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE schedules SET cron_expression = $2, enabled = $3, parallelism = $4, updated_by = $5, updated_at = NOW()
		 WHERE id = $1`,
		id, cronExpr, enabled, parallelism, updatedBy,
	)
	return err
}

func (s *scheduleStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM schedules WHERE id = $1`, id,
	)
	return err
}

func (s *scheduleStore) UpdateLastRun(ctx context.Context, id uuid.UUID, lastRunAt time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE schedules SET last_run_at = $2 WHERE id = $1`,
		id, lastRunAt,
	)
	return err
}
