package db

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ScenarioStore manages saved scenario YAML persistence.
type ScenarioStore interface {
	Save(ctx context.Context, name, scenarioType, yaml, createdBy string) (*SavedScenario, error)
	Get(ctx context.Context, id uuid.UUID) (*SavedScenario, error)
	// List returns a filtered, paginated slice of scenarios for the UI.
	List(ctx context.Context, filters ListScenariosFilters, limit, offset int) (ScenarioPage, error)
	// ListAll returns every scenario in updated_at DESC order. For internal
	// callers (e.g. coverage maps) that need the full set in one shot.
	ListAll(ctx context.Context) ([]SavedScenario, error)
	Update(ctx context.Context, id uuid.UUID, name, scenarioType, yaml, updatedBy string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// SavedScenario represents a saved scenario configuration.
type SavedScenario struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	YAML      string    `json:"yaml"`
	CreatedBy string    `json:"createdBy"`
	UpdatedBy string    `json:"updatedBy"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// ScenarioPage is a paginated slice of saved scenarios with the total row count.
type ScenarioPage struct {
	Scenarios []SavedScenario `json:"scenarios"`
	Total     int             `json:"total"`
}

// ListScenariosFilters narrows the result set for ScenarioStore.List. Zero
// values mean "no constraint on this dimension".
type ListScenariosFilters struct {
	// Name is an ILIKE %name% match against saved_scenarios.name.
	Name string
	// Types restricts saved_scenarios.type to the listed values.
	Types []string
	// Since restricts scenarios to updated_at >= Since.
	Since *time.Time
}

type scenarioStore struct {
	pool *pgxpool.Pool
}

// NewScenarioStore creates a new ScenarioStore backed by PostgreSQL.
func NewScenarioStore(pool *pgxpool.Pool) ScenarioStore {
	return &scenarioStore{pool: pool}
}

func (s *scenarioStore) Save(ctx context.Context, name, scenarioType, yaml, createdBy string) (*SavedScenario, error) {
	var sc SavedScenario
	err := s.pool.QueryRow(ctx,
		`INSERT INTO saved_scenarios (name, type, yaml, created_by, updated_by) VALUES ($1, $2, $3, $4, $4)
		 RETURNING id, name, type, yaml, created_by, updated_by, created_at, updated_at`,
		name, scenarioType, yaml, createdBy,
	).Scan(&sc.ID, &sc.Name, &sc.Type, &sc.YAML, &sc.CreatedBy, &sc.UpdatedBy, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (s *scenarioStore) Get(ctx context.Context, id uuid.UUID) (*SavedScenario, error) {
	var sc SavedScenario
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, type, yaml, created_by, updated_by, created_at, updated_at FROM saved_scenarios WHERE id = $1`, id,
	).Scan(&sc.ID, &sc.Name, &sc.Type, &sc.YAML, &sc.CreatedBy, &sc.UpdatedBy, &sc.CreatedAt, &sc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}

func (s *scenarioStore) List(ctx context.Context, filters ListScenariosFilters, limit, offset int) (ScenarioPage, error) {
	where, args := buildScenariosWhere(filters)
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, type, yaml, created_by, updated_by, created_at, updated_at,
				COUNT(*) OVER() AS total_count
		 FROM saved_scenarios
		 `+where+`
		 ORDER BY updated_at DESC
		 LIMIT $`+strconv.Itoa(len(args)+1)+` OFFSET $`+strconv.Itoa(len(args)+2),
		append(args, limit, offset)...,
	)
	if err != nil {
		return ScenarioPage{}, err
	}
	defer rows.Close()

	page := ScenarioPage{Scenarios: []SavedScenario{}}
	for rows.Next() {
		var sc SavedScenario
		var total int
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Type, &sc.YAML, &sc.CreatedBy, &sc.UpdatedBy, &sc.CreatedAt, &sc.UpdatedAt, &total); err != nil {
			return ScenarioPage{}, err
		}
		page.Scenarios = append(page.Scenarios, sc)
		page.Total = total
	}
	if err := rows.Err(); err != nil {
		return ScenarioPage{}, err
	}
	if len(page.Scenarios) == 0 {
		// COUNT(*) OVER() collapses to no rows when LIMIT/OFFSET yields nothing.
		// Re-run the same filter as a plain COUNT so the UI can show "of N".
		countSQL := `SELECT COUNT(*) FROM saved_scenarios ` + where
		if err := s.pool.QueryRow(ctx, countSQL, args...).Scan(&page.Total); err != nil {
			return ScenarioPage{}, err
		}
	}
	return page, nil
}

func (s *scenarioStore) ListAll(ctx context.Context) ([]SavedScenario, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, type, yaml, created_by, updated_by, created_at, updated_at
		 FROM saved_scenarios ORDER BY updated_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	scenarios := []SavedScenario{}
	for rows.Next() {
		var sc SavedScenario
		if err := rows.Scan(&sc.ID, &sc.Name, &sc.Type, &sc.YAML, &sc.CreatedBy, &sc.UpdatedBy, &sc.CreatedAt, &sc.UpdatedAt); err != nil {
			return nil, err
		}
		scenarios = append(scenarios, sc)
	}
	return scenarios, rows.Err()
}

// buildScenariosWhere returns a WHERE clause (or "") and its positional args
// for saved_scenarios. Placeholders are $1..$N in argument order.
func buildScenariosWhere(f ListScenariosFilters) (string, []any) {
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

func (s *scenarioStore) Update(ctx context.Context, id uuid.UUID, name, scenarioType, yaml, updatedBy string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE saved_scenarios SET name = $2, type = $3, yaml = $4, updated_by = $5, updated_at = NOW() WHERE id = $1`,
		id, name, scenarioType, yaml, updatedBy,
	)
	return err
}

func (s *scenarioStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM saved_scenarios WHERE id = $1`, id,
	)
	return err
}
