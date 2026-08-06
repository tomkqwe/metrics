package repository

import (
	"github.com/tomkqwe/metrics/internal/repository/mem_storage"
	"strings"
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestRestoreMetrics(t *testing.T) {
	storage := mem_storage.NewMemStorage()
	gaugeValue := 12.5
	counterValue := int64(3)

	err := RestoreMetrics(storage, []models.Metric{
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
	if err != nil {
		t.Fatalf("RestoreMetrics() error = %v", err)
	}

	if value, ok := storage.GetGauge("Alloc"); !ok || value != models.Gauge(12.5) {
		t.Fatalf("GetGauge() = %v, %v, want 12.5, true", value, ok)
	}
	if value, ok := storage.GetCounter("PollCount"); !ok || value != models.Counter(3) {
		t.Fatalf("GetCounter() = %v, %v, want 3, true", value, ok)
	}
}

func TestRestoreMetricsReturnsErrorForInvalidMetric(t *testing.T) {
	tests := []struct {
		name      string
		metric    models.Metric
		wantError string
	}{
		{
			name: "gauge without value",
			metric: models.Metric{
				ID:    "Alloc",
				MType: models.MetricTypeGauge,
			},
			wantError: `restore gauge "Alloc": missing value`,
		},
		{
			name: "counter without delta",
			metric: models.Metric{
				ID:    "PollCount",
				MType: models.MetricTypeCounter,
			},
			wantError: `restore counter "PollCount": missing delta`,
		},
		{
			name: "unknown metric type",
			metric: models.Metric{
				ID:    "Unknown",
				MType: "unknown",
			},
			wantError: `restore metric "Unknown": unknown type "unknown"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RestoreMetrics(mem_storage.NewMemStorage(), []models.Metric{tt.metric})
			if err == nil {
				t.Fatal("RestoreMetrics() error = nil, want error")
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("RestoreMetrics() error = %q, want %q", err, tt.wantError)
			}
		})
	}
}
