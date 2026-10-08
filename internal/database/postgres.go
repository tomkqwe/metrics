// Package database opens PostgreSQL connection pools and applies schema migrations.
package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/tomkqwe/metrics/internal/postgreserr"
	"github.com/tomkqwe/metrics/internal/retry"
	"github.com/tomkqwe/metrics/migrations"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/lib/pq"
)

// OpenPostgres creates a PostgreSQL connection pool without checking connectivity.
// An empty DSN returns nil, nil to disable PostgreSQL storage.
func OpenPostgres(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, nil
	}

	return sql.Open("postgres", dsn)
}

// RunMigrations applies embedded SQL migrations, retrying connection exceptions.
// An empty DSN or an already-current schema is a no-op.
func RunMigrations(dsn string) error {
	if dsn == "" {
		return nil
	}
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, dsn)
	if err != nil {
		return fmt.Errorf("failed to create migration: %w", err)
	}
	defer func() {
		sourceErr, databaseErr := m.Close()
		_ = sourceErr
		_ = databaseErr
	}()

	err = retry.Do(context.Background(), m.Up, postgreserr.IsConnectionException)
	if err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("failed to migrate: %w", err)
	}
	return nil
}
