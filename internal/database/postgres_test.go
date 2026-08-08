package database

import "testing"

func TestRunMigrationsSkipsEmptyDSN(t *testing.T) {
	if err := RunMigrations(""); err != nil {
		t.Fatalf("RunMigrations() error = %v, want nil", err)
	}
}
