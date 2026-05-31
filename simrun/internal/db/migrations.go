package db

import (
	"embed"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate runs all pending database migrations.
func (d *DB) Migrate(databaseURL string) error {
	source, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Convert postgres:// or postgresql:// to pgx5:// for the migrate driver
	migrationURL := databaseURL
	if strings.HasPrefix(migrationURL, "postgresql://") {
		migrationURL = "pgx5://" + strings.TrimPrefix(migrationURL, "postgresql://")
	} else if strings.HasPrefix(migrationURL, "postgres://") {
		migrationURL = "pgx5://" + strings.TrimPrefix(migrationURL, "postgres://")
	}

	m, err := migrate.NewWithSourceInstance("iofs", source, migrationURL)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("migration failed: %w", err)
	}

	return nil
}
