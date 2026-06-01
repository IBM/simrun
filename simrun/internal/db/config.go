package db

import (
	"context"
	"encoding/json"

	"github.com/IBM/simrun/simrun/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ConfigStore manages key-value application configuration.
type ConfigStore interface {
	Get(ctx context.Context, key string) (json.RawMessage, error)
	Set(ctx context.Context, key string, value json.RawMessage) error
	GetAll(ctx context.Context) (map[string]json.RawMessage, error)
	GetAppConfig(ctx context.Context) (config.AppConfig, error)
	UpdateAppConfig(ctx context.Context, c config.AppConfig) error
}

type configStore struct {
	pool *pgxpool.Pool
}

// NewConfigStore creates a new ConfigStore backed by PostgreSQL.
func NewConfigStore(pool *pgxpool.Pool) ConfigStore {
	return &configStore{pool: pool}
}

func (s *configStore) Get(ctx context.Context, key string) (json.RawMessage, error) {
	var value json.RawMessage
	err := s.pool.QueryRow(ctx,
		`SELECT value FROM app_config WHERE key = $1`, key,
	).Scan(&value)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func (s *configStore) Set(ctx context.Context, key string, value json.RawMessage) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO app_config (key, value) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = NOW()`,
		key, value,
	)
	return err
}

func (s *configStore) GetAll(ctx context.Context) (map[string]json.RawMessage, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT key, value FROM app_config ORDER BY key`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]json.RawMessage)
	for rows.Next() {
		var key string
		var value json.RawMessage
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		result[key] = value
	}
	return result, rows.Err()
}

// parseAppConfig converts the raw key-value map from app_config into a typed
// AppConfig, falling back to defaults for missing keys or unparseable values.
func parseAppConfig(all map[string]json.RawMessage) config.AppConfig {
	cfg := config.DefaultAppConfig()

	if v, ok := all["parallelism"]; ok {
		var n int
		if err := json.Unmarshal(v, &n); err == nil && n > 0 {
			cfg.Parallelism = n
		}
	}
	if v, ok := all["terraform_version"]; ok {
		var s string
		if err := json.Unmarshal(v, &s); err == nil {
			cfg.TerraformVersion = s
		}
	}
	if v, ok := all["pack_logs_enabled"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			cfg.PackLogsEnabled = b
		}
	}
	if v, ok := all["ssh_logging_enabled"]; ok {
		var b bool
		if err := json.Unmarshal(v, &b); err == nil {
			cfg.SSHLoggingEnabled = b
		}
	}

	return cfg
}

type appConfigKV struct {
	key string
	val any
}

// appConfigKVs returns the key-value pairs that should be persisted for the
// given AppConfig. Extracted so the marshalling can be unit-tested without
// a database.
func appConfigKVs(c config.AppConfig) []appConfigKV {
	return []appConfigKV{
		{"parallelism", c.Parallelism},
		{"terraform_version", c.TerraformVersion},
		{"pack_logs_enabled", c.PackLogsEnabled},
		{"ssh_logging_enabled", c.SSHLoggingEnabled},
	}
}

func (s *configStore) GetAppConfig(ctx context.Context) (config.AppConfig, error) {
	all, err := s.GetAll(ctx)
	if err != nil {
		return config.DefaultAppConfig(), err
	}
	return parseAppConfig(all), nil
}

func (s *configStore) UpdateAppConfig(ctx context.Context, c config.AppConfig) error {
	for _, p := range appConfigKVs(c) {
		raw, err := json.Marshal(p.val)
		if err != nil {
			return err
		}
		if err := s.Set(ctx, p.key, raw); err != nil {
			return err
		}
	}
	return nil
}
