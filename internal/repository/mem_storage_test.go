package repository

import (
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestMemStorageUpdateGauge(t *testing.T) {
	storage := NewMemStorage()

	storage.UpdateGauge("Alloc", models.Gauge(12.5))

	value, ok := storage.GetGauge("Alloc")
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

	value, ok := storage.GetCounter("PollCount")
	if !ok {
		t.Fatal("counter metric PollCount was not stored")
	}
	if value != models.Counter(3) {
		t.Fatalf("counter metric PollCount = %v, want %v", value, models.Counter(3))
	}
}

func TestMemStorageUpdateReplacesGaugeAndAccumulatesCounter(t *testing.T) {
	storage := NewMemStorage()

	storage.UpdateGauge("Alloc", models.Gauge(12.5))
	storage.UpdateGauge("Alloc", models.Gauge(25.5))
	storage.UpdateCounter("PollCount", models.Counter(3))
	storage.UpdateCounter("PollCount", models.Counter(5))

	if value, _ := storage.GetGauge("Alloc"); value != models.Gauge(25.5) {
		t.Fatalf("gauge metric Alloc = %v, want %v", value, models.Gauge(25.5))
	}
	if value, _ := storage.GetCounter("PollCount"); value != models.Counter(8) {
		t.Fatalf("counter metric PollCount = %v, want %v", value, models.Counter(8))
	}
}

func TestMemStorageGetUnknownMetric(t *testing.T) {
	storage := NewMemStorage()

	if _, ok := storage.GetGauge("UnknownGauge"); ok {
		t.Fatal("GetGauge() ok = true, want false")
	}
	if _, ok := storage.GetCounter("UnknownCounter"); ok {
		t.Fatal("GetCounter() ok = true, want false")
	}
}

func TestMemStorageSnapshot(t *testing.T) {
	storage := NewMemStorage()

	storage.UpdateGauge("Alloc", models.Gauge(12.5))
	storage.UpdateCounter("PollCount", models.Counter(3))

	snapshot := storage.Snapshot()
	if len(snapshot) != 2 {
		t.Fatalf("Snapshot() len = %d, want 2", len(snapshot))
	}

	byName := make(map[string]models.Metric, len(snapshot))
	for _, metric := range snapshot {
		byName[metric.ID] = metric
	}

	if metric := byName["Alloc"]; metric.MType != models.MetricTypeGauge || metric.Value == nil || *metric.Value != 12.5 {
		t.Fatalf("Snapshot() Alloc = %+v, want gauge value 12.5", metric)
	}
	if metric := byName["PollCount"]; metric.MType != models.MetricTypeCounter || metric.Delta == nil || *metric.Delta != 3 {
		t.Fatalf("Snapshot() PollCount = %+v, want counter delta 3", metric)
	}
}
