package service

import (
	"errors"
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestNewMetricServiceReturnsErrorForNilStorage(t *testing.T) {
	_, err := NewMetricService(nil)
	if !errors.Is(err, ErrInvalidStorage) {
		t.Fatalf("NewMetricService() error = %v, want %v", err, ErrInvalidStorage)
	}
}

func TestMetricServiceUpdateGauge(t *testing.T) {
	storage := &fakeStorage{}
	service, err := NewMetricService(storage)
	if err != nil {
		t.Fatalf("NewMetricService() error = %v", err)
	}

	err = service.UpdateMetric(models.MetricTypeGauge, "Alloc", "12.5")
	if err != nil {
		t.Fatalf("UpdateMetric() error = %v", err)
	}

	if storage.gaugeName != "Alloc" {
		t.Fatalf("UpdateGauge() name = %q, want Alloc", storage.gaugeName)
	}
	if storage.gaugeValue != models.Gauge(12.5) {
		t.Fatalf("UpdateGauge() value = %v, want %v", storage.gaugeValue, models.Gauge(12.5))
	}
	if storage.counterCalled {
		t.Fatal("UpdateCounter() was called for gauge metric")
	}
}

func TestMetricServiceUpdateCounter(t *testing.T) {
	storage := &fakeStorage{}
	service, err := NewMetricService(storage)
	if err != nil {
		t.Fatalf("NewMetricService() error = %v", err)
	}

	err = service.UpdateMetric(models.MetricTypeCounter, "PollCount", "3")
	if err != nil {
		t.Fatalf("UpdateMetric() error = %v", err)
	}

	if storage.counterName != "PollCount" {
		t.Fatalf("UpdateCounter() name = %q, want PollCount", storage.counterName)
	}
	if storage.counterValue != models.Counter(3) {
		t.Fatalf("UpdateCounter() value = %v, want %v", storage.counterValue, models.Counter(3))
	}
	if storage.gaugeCalled {
		t.Fatal("UpdateGauge() was called for counter metric")
	}
}

func TestMetricServiceUpdateMetricReturnsErrorForInvalidValue(t *testing.T) {
	tests := []struct {
		name       string
		metricType string
		value      string
	}{
		{
			name:       "invalid gauge",
			metricType: models.MetricTypeGauge,
			value:      "not-float",
		},
		{
			name:       "invalid counter",
			metricType: models.MetricTypeCounter,
			value:      "not-int",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeStorage{}
			service, err := NewMetricService(storage)
			if err != nil {
				t.Fatalf("NewMetricService() error = %v", err)
			}

			err = service.UpdateMetric(tt.metricType, "MetricName", tt.value)
			if err == nil {
				t.Fatal("UpdateMetric() error = nil, want error")
			}
			if storage.gaugeCalled || storage.counterCalled {
				t.Fatal("storage update was called for invalid value")
			}
		})
	}
}

func TestMetricServiceUpdateMetricReturnsErrorForUnknownMetricType(t *testing.T) {
	storage := &fakeStorage{}
	service, err := NewMetricService(storage)
	if err != nil {
		t.Fatalf("NewMetricService() error = %v", err)
	}

	err = service.UpdateMetric("unknown", "Alloc", "12.5")
	if !errors.Is(err, ErrUnknownMetricType) {
		t.Fatalf("UpdateMetric() error = %v, want %v", err, ErrUnknownMetricType)
	}
	if storage.gaugeCalled || storage.counterCalled {
		t.Fatal("storage update was called for unknown metric type")
	}
}

func TestMetricServiceGetMetricValue(t *testing.T) {
	storage := &fakeStorage{
		gauges: map[string]models.Gauge{
			"Alloc": models.Gauge(12.5),
		},
		counters: map[string]models.Counter{
			"PollCount": models.Counter(3),
		},
	}
	service, err := NewMetricService(storage)
	if err != nil {
		t.Fatalf("NewMetricService() error = %v", err)
	}

	tests := []struct {
		name       string
		metricType string
		metricName string
		want       string
	}{
		{
			name:       "gauge",
			metricType: models.MetricTypeGauge,
			metricName: "Alloc",
			want:       "12.5",
		},
		{
			name:       "counter",
			metricType: models.MetricTypeCounter,
			metricName: "PollCount",
			want:       "3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := service.GetMetricValue(tt.metricType, tt.metricName)
			if err != nil {
				t.Fatalf("GetMetricValue() error = %v", err)
			}
			if value != tt.want {
				t.Fatalf("GetMetricValue() value = %q, want %q", value, tt.want)
			}
		})
	}
}

func TestMetricServiceGetMetricValueReturnsNotFoundForUnknownMetric(t *testing.T) {
	storage := &fakeStorage{
		gauges:   make(map[string]models.Gauge),
		counters: make(map[string]models.Counter),
	}
	service, err := NewMetricService(storage)
	if err != nil {
		t.Fatalf("NewMetricService() error = %v", err)
	}

	_, err = service.GetMetricValue(models.MetricTypeGauge, "Unknown")
	if !errors.Is(err, ErrMetricNotFound) {
		t.Fatalf("GetMetricValue() error = %v, want %v", err, ErrMetricNotFound)
	}
}

func TestMetricServiceGetMetricValueReturnsErrorForUnknownMetricType(t *testing.T) {
	storage := &fakeStorage{}
	service, err := NewMetricService(storage)
	if err != nil {
		t.Fatalf("NewMetricService() error = %v", err)
	}

	_, err = service.GetMetricValue("unknown", "Alloc")
	if !errors.Is(err, ErrUnknownMetricType) {
		t.Fatalf("GetMetricValue() error = %v, want %v", err, ErrUnknownMetricType)
	}
}

func TestMetricServiceListMetrics(t *testing.T) {
	value := 12.5
	storage := &fakeStorage{
		snapshot: []models.Metric{
			{
				ID:    "Alloc",
				MType: models.MetricTypeGauge,
				Value: &value,
			},
		},
	}
	service, err := NewMetricService(storage)
	if err != nil {
		t.Fatalf("NewMetricService() error = %v", err)
	}

	metrics := service.ListMetrics()
	if len(metrics) != 1 {
		t.Fatalf("ListMetrics() len = %d, want 1", len(metrics))
	}
	if metrics[0].ID != "Alloc" {
		t.Fatalf("ListMetrics()[0].ID = %q, want Alloc", metrics[0].ID)
	}
}

type fakeStorage struct {
	gaugeCalled   bool
	gaugeName     string
	gaugeValue    models.Gauge
	counterCalled bool
	counterName   string
	counterValue  models.Counter
	gauges        map[string]models.Gauge
	counters      map[string]models.Counter
	snapshot      []models.Metric
}

func (s *fakeStorage) UpdateGauge(name string, value models.Gauge) {
	s.gaugeCalled = true
	s.gaugeName = name
	s.gaugeValue = value
}

func (s *fakeStorage) UpdateCounter(name string, value models.Counter) {
	s.counterCalled = true
	s.counterName = name
	s.counterValue = value
}

func (s *fakeStorage) GetGauge(name string) (models.Gauge, bool) {
	value, ok := s.gauges[name]
	return value, ok
}

func (s *fakeStorage) GetCounter(name string) (models.Counter, bool) {
	value, ok := s.counters[name]
	return value, ok
}

func (s *fakeStorage) Snapshot() []models.Metric {
	return s.snapshot
}
