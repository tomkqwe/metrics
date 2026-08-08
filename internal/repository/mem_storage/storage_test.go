package mem_storage

import (
	"context"
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestMemStorageUpdateGauge(t *testing.T) {
	storage := NewMemStorage()

	mustNoError(t, storage.UpdateGauge(context.Background(), "Alloc", models.Gauge(12.5)))

	value, ok, err := storage.GetGauge(context.Background(), "Alloc")
	if err != nil {
		t.Fatalf("GetGauge() error = %v", err)
	}
	if !ok {
		t.Fatal("gauge metric Alloc was not stored")
	}
	if value != models.Gauge(12.5) {
		t.Fatalf("gauge metric Alloc = %v, want %v", value, models.Gauge(12.5))
	}
}

func TestMemStorageUpdateCounter(t *testing.T) {
	storage := NewMemStorage()

	mustNoError(t, storage.UpdateCounter(context.Background(), "PollCount", models.Counter(3)))

	value, ok, err := storage.GetCounter(context.Background(), "PollCount")
	if err != nil {
		t.Fatalf("GetCounter() error = %v", err)
	}
	if !ok {
		t.Fatal("counter metric PollCount was not stored")
	}
	if value != models.Counter(3) {
		t.Fatalf("counter metric PollCount = %v, want %v", value, models.Counter(3))
	}
}

func TestMemStorageUpdateReplacesGaugeAndAccumulatesCounter(t *testing.T) {
	storage := NewMemStorage()

	mustNoError(t, storage.UpdateGauge(context.Background(), "Alloc", models.Gauge(12.5)))
	mustNoError(t, storage.UpdateGauge(context.Background(), "Alloc", models.Gauge(25.5)))
	mustNoError(t, storage.UpdateCounter(context.Background(), "PollCount", models.Counter(3)))
	mustNoError(t, storage.UpdateCounter(context.Background(), "PollCount", models.Counter(5)))

	if value, _, err := storage.GetGauge(context.Background(), "Alloc"); err != nil || value != models.Gauge(25.5) {
		t.Fatalf("gauge metric Alloc = %v, want %v", value, models.Gauge(25.5))
	}
	if value, _, err := storage.GetCounter(context.Background(), "PollCount"); err != nil || value != models.Counter(8) {
		t.Fatalf("counter metric PollCount = %v, want %v", value, models.Counter(8))
	}
}

func TestMemStorageUpdateMetrics(t *testing.T) {
	storage := NewMemStorage()
	gaugeValue := 12.5
	updatedGaugeValue := 25.5
	counterDelta := int64(3)
	updatedCounterDelta := int64(5)

	mustNoError(t, storage.UpdateMetrics(context.Background(), []models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &gaugeValue,
		},
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &updatedGaugeValue,
		},
		{
			ID:    "PollCount",
			MType: models.MetricTypeCounter,
			Delta: &counterDelta,
		},
		{
			ID:    "PollCount",
			MType: models.MetricTypeCounter,
			Delta: &updatedCounterDelta,
		},
	}))

	if value, ok, err := storage.GetGauge(context.Background(), "Alloc"); err != nil || !ok || value != models.Gauge(25.5) {
		t.Fatalf("GetGauge() = %v, %v, want 25.5, true", value, ok)
	}
	if value, ok, err := storage.GetCounter(context.Background(), "PollCount"); err != nil || !ok || value != models.Counter(8) {
		t.Fatalf("GetCounter() = %v, %v, want 8, true", value, ok)
	}
}

func TestMemStorageGetUnknownMetric(t *testing.T) {
	storage := NewMemStorage()

	if _, ok, err := storage.GetGauge(context.Background(), "UnknownGauge"); err != nil || ok {
		t.Fatal("GetGauge() ok = true, want false")
	}
	if _, ok, err := storage.GetCounter(context.Background(), "UnknownCounter"); err != nil || ok {
		t.Fatal("GetCounter() ok = true, want false")
	}
}

func TestMemStorageSnapshot(t *testing.T) {
	storage := NewMemStorage()

	mustNoError(t, storage.UpdateGauge(context.Background(), "Alloc", models.Gauge(12.5)))
	mustNoError(t, storage.UpdateCounter(context.Background(), "PollCount", models.Counter(3)))

	snapshot, err := storage.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
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

func mustNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
