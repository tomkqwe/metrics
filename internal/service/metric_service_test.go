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

type fakeStorage struct {
	gaugeCalled   bool
	gaugeName     string
	gaugeValue    models.Gauge
	counterCalled bool
	counterName   string
	counterValue  models.Counter
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
