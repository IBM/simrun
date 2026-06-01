package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PackStore manages pack persistence.
type PackStore interface {
	Upsert(ctx context.Context, pack *Pack, installedBy string) error
	Get(ctx context.Context, name string) (*Pack, error)
	List(ctx context.Context) ([]Pack, error)
	Delete(ctx context.Context, name string) error
	UpdateParameters(ctx context.Context, name string, parameters map[string]any) error
}

// Pack represents an installed simulation pack.
type Pack struct {
	ID          uuid.UUID      `json:"id"`
	Name        string         `json:"name"`
	Type        string         `json:"type"`
	Source      string         `json:"source"`
	Version     string         `json:"version,omitempty"`
	Status      string         `json:"status"`
	Parameters  map[string]any `json:"parameters,omitempty"`
	InstalledBy string         `json:"installedBy"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type packStore struct {
	pool *pgxpool.Pool
}

// NewPackStore creates a new PackStore backed by PostgreSQL.
func NewPackStore(pool *pgxpool.Pool) PackStore {
	return &packStore{pool: pool}
}

func (s *packStore) Upsert(ctx context.Context, pack *Pack, installedBy string) error {
	params := pack.Parameters
	if params == nil {
		params = map[string]any{}
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO packs (name, type, source, version, status, parameters, installed_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (name) DO UPDATE SET type = $2, source = $3, version = $4, status = $5, parameters = $6, installed_by = $7, updated_at = NOW()`,
		pack.Name, pack.Type, pack.Source, pack.Version, pack.Status, params, installedBy,
	)
	return err
}

func (s *packStore) Get(ctx context.Context, name string) (*Pack, error) {
	var p Pack
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, type, source, version, status, parameters, installed_by, created_at, updated_at FROM packs WHERE name = $1`, name,
	).Scan(&p.ID, &p.Name, &p.Type, &p.Source, &p.Version, &p.Status, &p.Parameters, &p.InstalledBy, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *packStore) List(ctx context.Context) ([]Pack, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, type, source, version, status, parameters, installed_by, created_at, updated_at FROM packs ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	packs := []Pack{}
	for rows.Next() {
		var p Pack
		if err := rows.Scan(&p.ID, &p.Name, &p.Type, &p.Source, &p.Version, &p.Status, &p.Parameters, &p.InstalledBy, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		packs = append(packs, p)
	}
	return packs, rows.Err()
}

func (s *packStore) Delete(ctx context.Context, name string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM packs WHERE name = $1`, name,
	)
	return err
}

func (s *packStore) UpdateParameters(ctx context.Context, name string, parameters map[string]any) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE packs SET parameters = $1, updated_at = NOW() WHERE name = $2`,
		parameters, name,
	)
	return err
}
