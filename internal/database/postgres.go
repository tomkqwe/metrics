package database

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"

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

	if err = m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			return nil
		}
		return fmt.Errorf("failed to migrate: %w", err)
	}
	return nil
}
