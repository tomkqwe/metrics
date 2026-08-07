package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"

	"github.com/tomkqwe/metrics/internal/postgreserr"
	"github.com/tomkqwe/metrics/internal/retry"

	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
)

func OpenPostgres(dsn string) (*sql.DB, error) {
	if dsn == "" {
		return nil, nil
	}

	return sql.Open("postgres", dsn)
}

func RunMigrations(dsn string) error {
	if dsn == "" {
		return nil
	}
	m, err := migrate.New("file://./migrations", dsn)
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
