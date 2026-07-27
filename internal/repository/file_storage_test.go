package repository

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestFileStorageSaveAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics", "storage.json")
	storage := NewFileStorage(path)

	gaugeValue := 12.5
	counterValue := int64(3)
	metrics := []models.Metric{
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
	}

	if err := storage.Save(metrics); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	rawFile, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read storage file: %v", err)
	}
	var storedMetrics []models.Metric
	if err = json.Unmarshal(rawFile, &storedMetrics); err != nil {
		t.Fatalf("storage file contains invalid JSON: %v", err)
	}
	assertMetrics(t, storedMetrics, metrics)

	loadedMetrics, err := storage.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	assertMetrics(t, loadedMetrics, metrics)
}

func TestFileStorageLoadMissingFile(t *testing.T) {
	storage := NewFileStorage(filepath.Join(t.TempDir(), "missing.json"))

	metrics, err := storage.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(metrics) != 0 {
		t.Fatalf("Load() len = %d, want 0", len(metrics))
	}
}

func assertMetrics(t *testing.T, got, want []models.Metric) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("metrics len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID {
			t.Fatalf("metric %d ID = %q, want %q", i, got[i].ID, want[i].ID)
		}
		if got[i].MType != want[i].MType {
			t.Fatalf("metric %d MType = %q, want %q", i, got[i].MType, want[i].MType)
		}
		assertOptionalFloat(t, got[i].Value, want[i].Value)
		assertOptionalInt(t, got[i].Delta, want[i].Delta)
	}
}

func assertOptionalFloat(t *testing.T, got, want *float64) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("Value = %v, want %v", got, want)
	}
	if *got != *want {
		t.Fatalf("Value = %v, want %v", *got, *want)
	}
}

func assertOptionalInt(t *testing.T, got, want *int64) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("Delta = %v, want %v", got, want)
	}
	if *got != *want {
		t.Fatalf("Delta = %v, want %v", *got, *want)
	}
}
