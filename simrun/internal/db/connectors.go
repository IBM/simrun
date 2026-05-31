package db

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectorStore manages connector persistence.
type ConnectorStore interface {
	Save(ctx context.Context, name, connectorType, description string, secretGroupID *uuid.UUID, config json.RawMessage, isDefault bool, createdBy string) (*Connector, error)
	Get(ctx context.Context, id uuid.UUID) (*Connector, error)
	GetByName(ctx context.Context, name string) (*Connector, error)
	GetDefault(ctx context.Context, connectorType string) (*Connector, error)
	List(ctx context.Context) ([]Connector, error)
	Update(ctx context.Context, id uuid.UUID, name, description string, secretGroupID *uuid.UUID, config json.RawMessage, enabled bool, isDefault bool, updatedBy string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// Connector represents an external system connector (alert source or cloud target).
type Connector struct {
	ID            uuid.UUID       `json:"id"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	Description   string          `json:"description"`
	SecretGroupID *uuid.UUID      `json:"secretGroupId,omitempty"`
	Config        json.RawMessage `json:"config"`
	Enabled       bool            `json:"enabled"`
	IsDefault     bool            `json:"isDefault"`
	CreatedBy     string          `json:"createdBy"`
	UpdatedBy     string          `json:"updatedBy"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

type connectorStore struct {
	pool *pgxpool.Pool
}

// NewConnectorStore creates a new ConnectorStore backed by PostgreSQL.
func NewConnectorStore(pool *pgxpool.Pool) ConnectorStore {
	return &connectorStore{pool: pool}
}

func (s *connectorStore) Save(ctx context.Context, name, connectorType, description string, secretGroupID *uuid.UUID, config json.RawMessage, isDefault bool, createdBy string) (*Connector, error) {
	var c Connector
	err := s.pool.QueryRow(ctx,
		`INSERT INTO connectors (name, type, description, secret_group_id, config, is_default, created_by, updated_by)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
		 RETURNING id, name, type, description, secret_group_id, config, enabled, is_default, created_by, updated_by, created_at, updated_at`,
		name, connectorType, description, secretGroupID, config, isDefault, createdBy,
	).Scan(&c.ID, &c.Name, &c.Type, &c.Description, &c.SecretGroupID, &c.Config, &c.Enabled, &c.IsDefault, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *connectorStore) Get(ctx context.Context, id uuid.UUID) (*Connector, error) {
	var c Connector
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, type, description, secret_group_id, config, enabled, is_default, created_by, updated_by, created_at, updated_at
		 FROM connectors WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Type, &c.Description, &c.SecretGroupID, &c.Config, &c.Enabled, &c.IsDefault, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *connectorStore) GetByName(ctx context.Context, name string) (*Connector, error) {
	var c Connector
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, type, description, secret_group_id, config, enabled, is_default, created_by, updated_by, created_at, updated_at
		 FROM connectors WHERE name = $1`, name,
	).Scan(&c.ID, &c.Name, &c.Type, &c.Description, &c.SecretGroupID, &c.Config, &c.Enabled, &c.IsDefault, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *connectorStore) GetDefault(ctx context.Context, connectorType string) (*Connector, error) {
	var c Connector
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, type, description, secret_group_id, config, enabled, is_default, created_by, updated_by, created_at, updated_at
		 FROM connectors WHERE type = $1 AND is_default = true AND enabled = true`, connectorType,
	).Scan(&c.ID, &c.Name, &c.Type, &c.Description, &c.SecretGroupID, &c.Config, &c.Enabled, &c.IsDefault, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *connectorStore) List(ctx context.Context) ([]Connector, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, type, description, secret_group_id, config, enabled, is_default, created_by, updated_by, created_at, updated_at
		 FROM connectors ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	connectors := []Connector{}
	for rows.Next() {
		var c Connector
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Description, &c.SecretGroupID, &c.Config, &c.Enabled, &c.IsDefault, &c.CreatedBy, &c.UpdatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		connectors = append(connectors, c)
	}
	return connectors, rows.Err()
}

func (s *connectorStore) Update(ctx context.Context, id uuid.UUID, name, description string, secretGroupID *uuid.UUID, config json.RawMessage, enabled bool, isDefault bool, updatedBy string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE connectors SET name = $2, description = $3, secret_group_id = $4, config = $5, enabled = $6, is_default = $7, updated_by = $8, updated_at = NOW()
		 WHERE id = $1`,
		id, name, description, secretGroupID, config, enabled, isDefault, updatedBy,
	)
	return err
}

func (s *connectorStore) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM connectors WHERE id = $1`, id,
	)
	return err
}
