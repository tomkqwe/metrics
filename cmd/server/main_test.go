package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/repository"
)

func TestParseConfigUsesDefaults(t *testing.T) {
	unsetServerEnv(t)

	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.serverAddress != defaultServerAddress {
		t.Fatalf("serverAddress = %q, want %q", cfg.serverAddress, defaultServerAddress)
	}
	if cfg.storeInterval != defaultStoreIntervalSeconds*time.Second {
		t.Fatalf("storeInterval = %v, want %v", cfg.storeInterval, defaultStoreIntervalSeconds*time.Second)
	}
	if cfg.fileStoragePath != defaultFileStoragePath {
		t.Fatalf("fileStoragePath = %q, want %q", cfg.fileStoragePath, defaultFileStoragePath)
	}
	if cfg.restore != defaultRestore {
		t.Fatalf("restore = %v, want %v", cfg.restore, defaultRestore)
	}
}

func TestParseConfigUsesFlags(t *testing.T) {
	unsetServerEnv(t)

	cfg, err := parseConfig([]string{
		"-a", "localhost:9090",
		"-i", "10",
		"-f", "/tmp/custom-metrics.json",
		"-r=false",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.serverAddress != "localhost:9090" {
		t.Fatalf("serverAddress = %q, want localhost:9090", cfg.serverAddress)
	}
	if cfg.storeInterval != 10*time.Second {
		t.Fatalf("storeInterval = %v, want 10s", cfg.storeInterval)
	}
	if cfg.fileStoragePath != "/tmp/custom-metrics.json" {
		t.Fatalf("fileStoragePath = %q, want /tmp/custom-metrics.json", cfg.fileStoragePath)
	}
	if cfg.restore {
		t.Fatal("restore = true, want false")
	}
}

func TestParseConfigEnvOverridesFlags(t *testing.T) {
	unsetServerEnv(t)
	t.Setenv("ADDRESS", "localhost:7070")
	t.Setenv("STORE_INTERVAL", "0")
	t.Setenv("FILE_STORAGE_PATH", "/tmp/env-metrics.json")
	t.Setenv("RESTORE", "false")

	cfg, err := parseConfig([]string{
		"-a", "localhost:9090",
		"-i", "10",
		"-f", "/tmp/flag-metrics.json",
		"-r=true",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.serverAddress != "localhost:7070" {
		t.Fatalf("serverAddress = %q, want localhost:7070", cfg.serverAddress)
	}
	if cfg.storeInterval != 0 {
		t.Fatalf("storeInterval = %v, want 0", cfg.storeInterval)
	}
	if cfg.fileStoragePath != "/tmp/env-metrics.json" {
		t.Fatalf("fileStoragePath = %q, want /tmp/env-metrics.json", cfg.fileStoragePath)
	}
	if cfg.restore {
		t.Fatal("restore = true, want false")
	}
}

func TestNewServerStorageRestoresMetrics(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	fileStorage := repository.NewFileStorage(path)
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
		fileStoragePath: path,
		restore:         true,
	})
	if err != nil {
		t.Fatalf("newServerStorage() error = %v", err)
	}
	if restoredFileStorage == nil {
		t.Fatal("file storage = nil, want configured storage")
	}

	if value, ok := storage.GetGauge("Alloc"); !ok || value != models.Gauge(12.5) {
		t.Fatalf("GetGauge() = %v, %v, want 12.5, true", value, ok)
	}
	if value, ok := storage.GetCounter("PollCount"); !ok || value != models.Counter(3) {
		t.Fatalf("GetCounter() = %v, %v, want 3, true", value, ok)
	}
}

func TestNewServerStorageSkipsRestore(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	fileStorage := repository.NewFileStorage(path)
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
		fileStoragePath: path,
		restore:         false,
	})
	if err != nil {
		t.Fatalf("newServerStorage() error = %v", err)
	}

	if _, ok := storage.GetGauge("Alloc"); ok {
		t.Fatal("GetGauge() ok = true, want false")
	}
}

func unsetServerEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{"ADDRESS", "STORE_INTERVAL", "FILE_STORAGE_PATH", "RESTORE"} {
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
