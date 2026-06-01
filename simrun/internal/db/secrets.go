package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SecretStore manages secret group persistence.
type SecretStore interface {
	Save(ctx context.Context, name, description string, entries json.RawMessage, createdBy string) (*SecretGroup, error)
	Get(ctx context.Context, id uuid.UUID) (*SecretGroup, error)
	List(ctx context.Context) ([]SecretGroup, error)
	Update(ctx context.Context, id uuid.UUID, name, description string, entries json.RawMessage, updatedBy string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// SecretGroup represents a named group of encrypted key-value secrets.
type SecretGroup struct {
	ID          uuid.UUID       `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Entries     json.RawMessage `json:"entries"`
	CreatedBy   string          `json:"createdBy"`
	UpdatedBy   string          `json:"updatedBy"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type secretStore struct {
	pool *pgxpool.Pool
}

// NewSecretStore creates a new SecretStore backed by PostgreSQL.
func NewSecretStore(pool *pgxpool.Pool) SecretStore {
	return &secretStore{pool: pool}
}

func (s *secretStore) Save(ctx context.Context, name, description string, entries json.RawMessage, createdBy string) (*SecretGroup, error) {
	var sg SecretGroup
	err := s.pool.QueryRow(ctx,
		`INSERT INTO secret_groups (name, description, entries, created_by, updated_by) VALUES ($1, $2, $3, $4, $4)
		 RETURNING id, name, description, entries, created_by, updated_by, created_at, updated_at`,
		name, description, entries, createdBy,
	).Scan(&sg.ID, &sg.Name, &sg.Description, &sg.Entries, &sg.CreatedBy, &sg.UpdatedBy, &sg.CreatedAt, &sg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sg, nil
}

func (s *secretStore) Get(ctx context.Context, id uuid.UUID) (*SecretGroup, error) {
	var sg SecretGroup
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, description, entries, created_by, updated_by, created_at, updated_at FROM secret_groups WHERE id = $1`, id,
	).Scan(&sg.ID, &sg.Name, &sg.Description, &sg.Entries, &sg.CreatedBy, &sg.UpdatedBy, &sg.CreatedAt, &sg.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sg, nil
}

func (s *secretStore) List(ctx context.Context) ([]SecretGroup, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, description, entries, created_by, updated_by, created_at, updated_at FROM secret_groups ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []SecretGroup{}
	for rows.Next() {
		var sg SecretGroup
		if err := rows.Scan(&sg.ID, &sg.Name, &sg.Description, &sg.Entries, &sg.CreatedBy, &sg.UpdatedBy, &sg.CreatedAt, &sg.UpdatedAt); err != nil {
			return nil, err
		}
		groups = append(groups, sg)
	}
	return groups, rows.Err()
}

func (s *secretStore) Update(ctx context.Context, id uuid.UUID, name, description string, entries json.RawMessage, updatedBy string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE secret_groups SET name = $2, description = $3, entries = $4, updated_by = $5, updated_at = NOW() WHERE id = $1`,
		id, name, description, entries, updatedBy,
	)
	return err
}

func (s *secretStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM secret_groups WHERE id = $1`, id,
	)
	return err
}
