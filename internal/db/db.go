// Package db is the PostgreSQL persistence layer (pgx), running embedded
// migrations on startup.
package db

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps a PostgreSQL connection pool.
type DB struct {
	Pool *pgxpool.Pool
}

// New creates a new DB connection pool and runs migrations.
func New(ctx context.Context, databaseURL string) (*DB, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	d := &DB{Pool: pool}

	if err := d.Migrate(databaseURL); err != nil {
		pool.Close()
		return nil, err
	}

	return d, nil
}

// Close closes the connection pool.
func (d *DB) Close() {
	d.Pool.Close()
}
