package main

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/repository/file_storage"
	"github.com/tomkqwe/metrics/internal/repository/postgres"
)

func TestParseConfigUsesDefaults(t *testing.T) {
	unsetServerEnv(t)

	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.ServerAddress != defaultServerAddress {
		t.Fatalf("ServerAddress = %q, want %q", cfg.ServerAddress, defaultServerAddress)
	}
	if cfg.StoreInterval != defaultStoreIntervalSeconds*time.Second {
		t.Fatalf("StoreInterval = %v, want %v", cfg.StoreInterval, defaultStoreIntervalSeconds*time.Second)
	}
	if cfg.FileStoragePath != "" {
		t.Fatalf("FileStoragePath = %q, want empty", cfg.FileStoragePath)
	}
	if cfg.Restore != defaultRestore {
		t.Fatalf("Restore = %v, want %v", cfg.Restore, defaultRestore)
	}
	if cfg.DatabaseDSN != "" {
		t.Fatalf("DatabaseDSN = %q, want empty", cfg.DatabaseDSN)
	}
}

func TestParseConfigUsesFlags(t *testing.T) {
	unsetServerEnv(t)

	cfg, err := parseConfig([]string{
		"-a", "localhost:9090",
		"-i", "10",
		"-f", "/tmp/custom-metrics.json",
		"-r=false",
		"-d", "postgres://flag-dsn",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:9090" {
		t.Fatalf("ServerAddress = %q, want localhost:9090", cfg.ServerAddress)
	}
	if cfg.StoreInterval != 10*time.Second {
		t.Fatalf("StoreInterval = %v, want 10s", cfg.StoreInterval)
	}
	if cfg.FileStoragePath != "/tmp/custom-metrics.json" {
		t.Fatalf("FileStoragePath = %q, want /tmp/custom-metrics.json", cfg.FileStoragePath)
	}
	if cfg.Restore {
		t.Fatal("Restore = true, want false")
	}
	if cfg.DatabaseDSN != "postgres://flag-dsn" {
		t.Fatalf("DatabaseDSN = %q, want postgres://flag-dsn", cfg.DatabaseDSN)
	}
}

func TestParseConfigEnvOverridesFlags(t *testing.T) {
	unsetServerEnv(t)
	t.Setenv("ADDRESS", "localhost:7070")
	t.Setenv("STORE_INTERVAL", "0")
	t.Setenv("FILE_STORAGE_PATH", "/tmp/env-metrics.json")
	t.Setenv("RESTORE", "false")
	t.Setenv("DATABASE_DSN", "postgres://env-dsn")

	cfg, err := parseConfig([]string{
		"-a", "localhost:9090",
		"-i", "10",
		"-f", "/tmp/flag-metrics.json",
		"-r=true",
		"-d", "postgres://flag-dsn",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.ServerAddress != "localhost:7070" {
		t.Fatalf("ServerAddress = %q, want localhost:7070", cfg.ServerAddress)
	}
	if cfg.StoreInterval != 0 {
		t.Fatalf("StoreInterval = %v, want 0", cfg.StoreInterval)
	}
	if cfg.FileStoragePath != "/tmp/env-metrics.json" {
		t.Fatalf("FileStoragePath = %q, want /tmp/env-metrics.json", cfg.FileStoragePath)
	}
	if cfg.Restore {
		t.Fatal("Restore = true, want false")
	}
	if cfg.DatabaseDSN != "postgres://env-dsn" {
		t.Fatalf("DatabaseDSN = %q, want postgres://env-dsn", cfg.DatabaseDSN)
	}
}

func TestNewServerStorageRestoresMetrics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	fileStorage := file_storage.NewFileStorage(path)
	gaugeValue := 12.5
	counterValue := int64(3)
	if err := fileStorage.Save([]models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &gaugeValue,
		},
		{
			ID:    "PollCount",
			MType: models.MetricTypeCounter,
			Delta: &counterValue,
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	storage, restoredFileStorage, err := newServerStorage(config{
		FileStoragePath: path,
		Restore:         true,
	}, nil)
	if err != nil {
		t.Fatalf("newServerStorage() error = %v", err)
	}
	if restoredFileStorage == nil {
		t.Fatal("file storage = nil, want configured storage")
	}

	if value, ok, err := storage.GetGauge(context.Background(), "Alloc"); err != nil || !ok || value != models.Gauge(12.5) {
		t.Fatalf("GetGauge() = %v, %v, want 12.5, true", value, ok)
	}
	if value, ok, err := storage.GetCounter(context.Background(), "PollCount"); err != nil || !ok || value != models.Counter(3) {
		t.Fatalf("GetCounter() = %v, %v, want 3, true", value, ok)
	}
}

func TestNewServerStorageSkipsRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	fileStorage := file_storage.NewFileStorage(path)
	gaugeValue := 12.5
	if err := fileStorage.Save([]models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &gaugeValue,
		},
	}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	storage, _, err := newServerStorage(config{
		FileStoragePath: path,
		Restore:         false,
	}, nil)
	if err != nil {
		t.Fatalf("newServerStorage() error = %v", err)
	}

	if _, ok, err := storage.GetGauge(context.Background(), "Alloc"); err != nil || ok {
		t.Fatal("GetGauge() ok = true, want false")
	}
}

func TestNewServerStorageUsesMemoryWhenFileStoragePathEmpty(t *testing.T) {
	storage, fileStorage, err := newServerStorage(config{}, nil)
	if err != nil {
		t.Fatalf("newServerStorage() error = %v", err)
	}
	if fileStorage != nil {
		t.Fatal("file storage is configured, want nil")
	}

	if err := storage.UpdateGauge(context.Background(), "Alloc", models.Gauge(12.5)); err != nil {
		t.Fatalf("UpdateGauge() error = %v", err)
	}
	if value, ok, err := storage.GetGauge(context.Background(), "Alloc"); err != nil || !ok || value != models.Gauge(12.5) {
		t.Fatalf("GetGauge() = %v, %v, want 12.5, true", value, ok)
	}
}

func TestNewServerStorageUsesPostgresWhenDatabaseDSNConfigured(t *testing.T) {
	db, err := sql.Open("postgres", "postgres://user:pass@localhost:5432/metrics?sslmode=disable")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})

	storage, fileStorage, err := newServerStorage(config{
		DatabaseDSN:     "postgres://user:pass@localhost:5432/metrics?sslmode=disable",
		FileStoragePath: filepath.Join(t.TempDir(), "metrics.json"),
		Restore:         true,
	}, db)
	if err != nil {
		t.Fatalf("newServerStorage() error = %v", err)
	}
	if fileStorage != nil {
		t.Fatal("file storage is configured, want nil")
	}
	if _, ok := storage.(*postgres.Storage); !ok {
		t.Fatalf("storage type = %T, want *postgres.Storage", storage)
	}
}

func TestNewServerStorageReturnsErrorWhenDatabaseDSNConfiguredWithoutDB(t *testing.T) {
	_, _, err := newServerStorage(config{
		DatabaseDSN: "postgres://user:pass@localhost:5432/metrics?sslmode=disable",
	}, nil)
	if err == nil {
		t.Fatal("newServerStorage() error = nil, want error")
	}
}

func unsetServerEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{"ADDRESS", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE", "DATABASE_DSN"} {
		oldValue, ok := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(key, oldValue)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}
