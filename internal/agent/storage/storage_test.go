package storage

import (
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestMemoryStorageUpdateAndSnapshot(t *testing.T) {
	s := NewMemoryStorage()
	gaugeValue := 12.5
	counterValue := int64(3)

	s.Update([]models.Metric{
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
	})

	metrics := metricsByName(s.Snapshot())
	assertGaugeValue(t, metrics["Alloc"], gaugeValue)
	assertCounterValue(t, metrics["PollCount"], counterValue)
}

func TestMemoryStorageUpdateReplacesExistingMetric(t *testing.T) {
	s := NewMemoryStorage()
	firstValue := 12.5
	secondValue := 25.5

	s.Update([]models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &firstValue,
		},
	})
	s.Update([]models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &secondValue,
		},
	})

	metrics := s.Snapshot()
	if len(metrics) != 1 {
		t.Fatalf("Snapshot() len = %d, want 1", len(metrics))
	}
	assertGaugeValue(t, metrics[0], secondValue)
}

func TestMemoryStorageDoesNotExposeInternalState(t *testing.T) {
	s := NewMemoryStorage()
	originalValue := 12.5

	input := []models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &originalValue,
		},
	}
	s.Update(input)

	originalValue = 99.9
	firstSnapshot := s.Snapshot()
	*firstSnapshot[0].Value = 55.5

	secondSnapshot := s.Snapshot()
	assertGaugeValue(t, secondSnapshot[0], 12.5)
}

func TestMemoryStorageSnapshotReturnsEmptySliceForEmptyStorage(t *testing.T) {
	s := NewMemoryStorage()

	metrics := s.Snapshot()

	if metrics == nil {
		t.Fatal("Snapshot() returned nil, want empty slice")
	}
	if len(metrics) != 0 {
		t.Fatalf("Snapshot() len = %d, want 0", len(metrics))
	}
}

func metricsByName(metrics []models.Metric) map[string]models.Metric {
	byName := make(map[string]models.Metric, len(metrics))
	for _, metric := range metrics {
		byName[metric.ID] = metric
	}
	return byName
}

func assertGaugeValue(t *testing.T, metric models.Metric, want float64) {
	t.Helper()

	if metric.MType != models.MetricTypeGauge {
		t.Fatalf("%s type = %q, want %q", metric.ID, metric.MType, models.MetricTypeGauge)
	}
	if metric.Value == nil {
		t.Fatalf("%s Value is nil", metric.ID)
	}
	if *metric.Value != want {
		t.Fatalf("%s Value = %f, want %f", metric.ID, *metric.Value, want)
	}
}

func assertCounterValue(t *testing.T, metric models.Metric, want int64) {
	t.Helper()

	if metric.MType != models.MetricTypeCounter {
		t.Fatalf("%s type = %q, want %q", metric.ID, metric.MType, models.MetricTypeCounter)
	}
	if metric.Delta == nil {
		t.Fatalf("%s Delta is nil", metric.ID)
	}
	if *metric.Delta != want {
		t.Fatalf("%s Delta = %d, want %d", metric.ID, *metric.Delta, want)
	}
}
