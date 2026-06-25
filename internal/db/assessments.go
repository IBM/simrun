package db

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AssessmentStore manages saved assessment (definition) YAML persistence.
type AssessmentStore interface {
	Save(ctx context.Context, name, assessmentType, yaml, createdBy string) (*Assessment, error)
	Get(ctx context.Context, id uuid.UUID) (*Assessment, error)
	// GetByName returns the assessment with the given unique name.
	GetByName(ctx context.Context, name string) (*Assessment, error)
	// List returns a filtered, paginated slice of assessments for the UI.
	List(ctx context.Context, filters ListAssessmentsFilters, limit, offset int) (AssessmentPage, error)
	// ListAll returns every assessment in updated_at DESC order. For internal
	// callers (e.g. coverage maps) that need the full set in one shot.
	ListAll(ctx context.Context) ([]Assessment, error)
	Update(ctx context.Context, id uuid.UUID, name, assessmentType, yaml, updatedBy string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// Assessment represents a saved assessment definition (the saved YAML).
type Assessment struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	YAML      string    `json:"yaml"`
	CreatedBy string    `json:"createdBy"`
	UpdatedBy string    `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// AssessmentPage is a paginated slice of assessments with the total row count.
type AssessmentPage struct {
	Assessments []Assessment `json:"assessments"`
	Total       int          `json:"total"`
}

// ListAssessmentsFilters narrows the result set for AssessmentStore.List. Zero
// values mean "no constraint on this dimension".
type ListAssessmentsFilters struct {
	// Name is an ILIKE %name% match against assessments.name.
	Name string
	// Types restricts assessments.type to the listed values.
	Types []string
	// Since restricts assessments to updated_at >= Since.
	Since *time.Time
}

type assessmentStore struct {
	pool *pgxpool.Pool
}

// NewAssessmentStore creates a new AssessmentStore backed by PostgreSQL.
func NewAssessmentStore(pool *pgxpool.Pool) AssessmentStore {
	return &assessmentStore{pool: pool}
}

func (s *assessmentStore) Save(ctx context.Context, name, assessmentType, yaml, createdBy string) (*Assessment, error) {
	var sc Assessment
	err := s.pool.QueryRow(ctx,
		`INSERT INTO assessments (name, type, yaml, created_by, updated_by) VALUES ($1, $2, $3, $4, $4)
		 RETURNING id, name, type, yaml, created_by, updated_by, created_at, updated_at`,
		name, assessmentType, yaml, createdBy,
	).Scan(&sc.ID, &sc.Name, &sc.Type, &sc.YAML, &sc.CreatedBy, &sc.UpdatedBy, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (s *assessmentStore) Get(ctx context.Context, id uuid.UUID) (*Assessment, error) {
	var sc Assessment
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, type, yaml, created_by, updated_by, created_at, updated_at FROM assessments WHERE id = $1`, id,
	).Scan(&sc.ID, &sc.Name, &sc.Type, &sc.YAML, &sc.CreatedBy, &sc.UpdatedBy, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (s *assessmentStore) GetByName(ctx context.Context, name string) (*Assessment, error) {
	var sc Assessment
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, type, yaml, created_by, updated_by, created_at, updated_at FROM assessments WHERE name = $1`, name,
	).Scan(&sc.ID, &sc.Name, &sc.Type, &sc.YAML, &sc.CreatedBy, &sc.UpdatedBy, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (s *assessmentStore) List(ctx context.Context, filters ListAssessmentsFilters, limit, offset int) (AssessmentPage, error) {
	where, args := buildAssessmentsWhere(filters)
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, type, yaml, created_by, updated_by, created_at, updated_at,
				COUNT(*) OVER() AS total_count
		 FROM assessments
		 `+where+`
		 ORDER BY updated_at DESC
		 LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, limit, offset)...,
	)
	if err != nil {
		return AssessmentPage{}, err
	}
	defer rows.Close()

	page := AssessmentPage{Assessments: []Assessment{}}
	for rows.Next() {
		var sc Assessment
		var total int
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Type, &sc.YAML, &sc.CreatedBy, &sc.UpdatedBy, &sc.CreatedAt, &sc.UpdatedAt, &total); err != nil {
			return AssessmentPage{}, err
		}
		page.Assessments = append(page.Assessments, sc)
		page.Total = total
	}
	if err := rows.Err(); err != nil {
		return AssessmentPage{}, err
	}
	if len(page.Assessments) == 0 {
		// COUNT(*) OVER() collapses to no rows when LIMIT/OFFSET yields nothing.
		// Re-run the same filter as a plain COUNT so the UI can show "of N".
		countSQL := `SELECT COUNT(*) FROM assessments ` + where
		if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&page.Total); err != nil {
			return AssessmentPage{}, err
		}
	}
	return page, nil
}

func (s *assessmentStore) ListAll(ctx context.Context) ([]Assessment, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, type, yaml, created_by, updated_by, created_at, updated_at
		 FROM assessments ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assessments := []Assessment{}
	for rows.Next() {
		var sc Assessment
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Type, &sc.YAML, &sc.CreatedBy, &sc.UpdatedBy, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		assessments = append(assessments, sc)
	}
	return assessments, rows.Err()
}

// buildAssessmentsWhere returns a WHERE clause (or "") and its positional args
// for assessments. Placeholders are $1..$N in argument order.
func buildAssessmentsWhere(f ListAssessmentsFilters) (string, []any) {
	var clauses []string
	var args []any
	if f.Name != "" {
		args = append(args, "%"+f.Name+"%")
		clauses = append(clauses, "name ILIKE $"+strconv.Itoa(len(args)))
	}
	if len(f.Types) > 0 {
		placeholders := make([]string, len(f.Types))
		for i, t := range f.Types {
			args = append(args, t)
			placeholders[i] = "$" + strconv.Itoa(len(args))
		}
		clauses = append(clauses, "type IN ("+strings.Join(placeholders, ",")+")")
	}
	if f.Since != nil {
		args = append(args, *f.Since)
		clauses = append(clauses, "updated_at >= $"+strconv.Itoa(len(args)))
	}
	if len(clauses) == 0 {
		return "", args
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (s *assessmentStore) Update(ctx context.Context, id uuid.UUID, name, assessmentType, yaml, updatedBy string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE assessments SET name = $2, type = $3, yaml = $4, updated_by = $5, updated_at = NOW() WHERE id = $1`,
		id, name, assessmentType, yaml, updatedBy,
	)
	return err
}

func (s *assessmentStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM assessments WHERE id = $1`, id,
	)
	return err
}
