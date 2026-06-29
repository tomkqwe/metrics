package repository

import (
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestMemStorageUpdateGauge(t *testing.T) {
	storage := NewMemStorage()

	storage.UpdateGauge("Alloc", models.Gauge(12.5))

	value, ok := storage.gaugeStore["Alloc"]
	if !ok {
		t.Fatal("gauge metric Alloc was not stored")
	}
	if value != models.Gauge(12.5) {
		t.Fatalf("gauge metric Alloc = %v, want %v", value, models.Gauge(12.5))
	}
}

func TestMemStorageUpdateCounter(t *testing.T) {
	storage := NewMemStorage()

	storage.UpdateCounter("PollCount", models.Counter(3))

	value, ok := storage.counterStore["PollCount"]
	if !ok {
		t.Fatal("counter metric PollCount was not stored")
	}
	if value != models.Counter(3) {
		t.Fatalf("counter metric PollCount = %v, want %v", value, models.Counter(3))
	}
}

func TestMemStorageUpdateReplacesExistingValues(t *testing.T) {
	storage := NewMemStorage()

	storage.UpdateGauge("Alloc", models.Gauge(12.5))
	storage.UpdateGauge("Alloc", models.Gauge(25.5))
	storage.UpdateCounter("PollCount", models.Counter(3))
	storage.UpdateCounter("PollCount", models.Counter(5))

	if value := storage.gaugeStore["Alloc"]; value != models.Gauge(25.5) {
		t.Fatalf("gauge metric Alloc = %v, want %v", value, models.Gauge(25.5))
	}
	if value := storage.counterStore["PollCount"]; value != models.Counter(5) {
		t.Fatalf("counter metric PollCount = %v, want %v", value, models.Counter(5))
	}
}
