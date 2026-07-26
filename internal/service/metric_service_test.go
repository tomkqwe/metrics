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

func TestMetricServiceUpdateMetricJSON(t *testing.T) {
	gaugeValue := 12.5
	counterDelta := int64(3)

	tests := []struct {
		name        string
		metric      models.Metric
		wantGauge   bool
		wantCounter bool
	}{
		{
			name: "gauge",
			metric: models.Metric{
				ID:    "Alloc",
				MType: models.MetricTypeGauge,
				Value: &gaugeValue,
			},
			wantGauge: true,
		},
		{
			name: "counter",
			metric: models.Metric{
				ID:    "PollCount",
				MType: models.MetricTypeCounter,
				Delta: &counterDelta,
			},
			wantCounter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeStorage{}
			service, err := NewMetricService(storage)
			if err != nil {
				t.Fatalf("NewMetricService() error = %v", err)
			}

			err = service.UpdateMetricJSON(&tt.metric)
			if err != nil {
				t.Fatalf("UpdateMetricJSON() error = %v", err)
			}

			if storage.gaugeCalled != tt.wantGauge {
				t.Fatalf("UpdateGauge() called = %v, want %v", storage.gaugeCalled, tt.wantGauge)
			}
			if storage.counterCalled != tt.wantCounter {
				t.Fatalf("UpdateCounter() called = %v, want %v", storage.counterCalled, tt.wantCounter)
			}
			if tt.wantGauge {
				if storage.gaugeName != tt.metric.ID {
					t.Fatalf("UpdateGauge() name = %q, want %q", storage.gaugeName, tt.metric.ID)
				}
				if storage.gaugeValue != models.Gauge(*tt.metric.Value) {
					t.Fatalf("UpdateGauge() value = %v, want %v", storage.gaugeValue, models.Gauge(*tt.metric.Value))
				}
			}
			if tt.wantCounter {
				if storage.counterName != tt.metric.ID {
					t.Fatalf("UpdateCounter() name = %q, want %q", storage.counterName, tt.metric.ID)
				}
				if storage.counterValue != models.Counter(*tt.metric.Delta) {
					t.Fatalf("UpdateCounter() value = %v, want %v", storage.counterValue, models.Counter(*tt.metric.Delta))
				}
			}
		})
	}
}

func TestMetricServicePersistsAfterSuccessfulUpdate(t *testing.T) {
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
	var savedMetrics []models.Metric
	service, err := NewMetricService(storage, WithUpdatePersister(func(metrics []models.Metric) error {
		savedMetrics = metrics
		return nil
	}))
	if err != nil {
		t.Fatalf("NewMetricService() error = %v", err)
	}

	err = service.UpdateMetric(models.MetricTypeGauge, "Alloc", "12.5")
	if err != nil {
		t.Fatalf("UpdateMetric() error = %v", err)
	}

	if len(savedMetrics) != 1 {
		t.Fatalf("saved metrics len = %d, want 1", len(savedMetrics))
	}
	if savedMetrics[0].ID != "Alloc" {
		t.Fatalf("saved metric ID = %q, want Alloc", savedMetrics[0].ID)
	}
}

func TestMetricServiceReturnsPersisterError(t *testing.T) {
	wantErr := errors.New("save failed")
	service, err := NewMetricService(&fakeStorage{}, WithUpdatePersister(func(metrics []models.Metric) error {
		return wantErr
	}))
	if err != nil {
		t.Fatalf("NewMetricService() error = %v", err)
	}

	err = service.UpdateMetric(models.MetricTypeCounter, "PollCount", "3")
	if !errors.Is(err, wantErr) {
		t.Fatalf("UpdateMetric() error = %v, want %v", err, wantErr)
	}
}

func TestMetricServiceUpdateMetricJSONReturnsErrorForInvalidMetric(t *testing.T) {
	tests := []struct {
		name    string
		metric  *models.Metric
		wantErr error
	}{
		{
			name:    "nil metric",
			wantErr: ErrNilMetric,
		},
		{
			name: "empty name",
			metric: &models.Metric{
				MType: models.MetricTypeGauge,
			},
			wantErr: ErrInvalidMetricName,
		},
		{
			name: "gauge without value",
			metric: &models.Metric{
				ID:    "Alloc",
				MType: models.MetricTypeGauge,
			},
			wantErr: ErrInvalidMetricValue,
		},
		{
			name: "counter without delta",
			metric: &models.Metric{
				ID:    "PollCount",
				MType: models.MetricTypeCounter,
			},
			wantErr: ErrInvalidMetricValue,
		},
		{
			name: "unknown type",
			metric: &models.Metric{
				ID:    "Alloc",
				MType: "unknown",
			},
			wantErr: ErrUnknownMetricType,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeStorage{}
			service, err := NewMetricService(storage)
			if err != nil {
				t.Fatalf("NewMetricService() error = %v", err)
			}

			err = service.UpdateMetricJSON(tt.metric)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("UpdateMetricJSON() error = %v, want %v", err, tt.wantErr)
			}
			if storage.gaugeCalled || storage.counterCalled {
				t.Fatal("storage update was called for invalid metric")
			}
		})
	}
}

func TestMetricServiceGetMetricJSON(t *testing.T) {
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
		name      string
		metric    models.Metric
		wantValue *float64
		wantDelta *int64
	}{
		{
			name: "gauge",
			metric: models.Metric{
				ID:    "Alloc",
				MType: models.MetricTypeGauge,
			},
			wantValue: ptrFloat64(12.5),
		},
		{
			name: "counter",
			metric: models.Metric{
				ID:    "PollCount",
				MType: models.MetricTypeCounter,
			},
			wantDelta: ptrInt64(3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metric, err := service.GetMetricJSON(&tt.metric)
			if err != nil {
				t.Fatalf("GetMetricJSON() error = %v", err)
			}
			if metric.ID != tt.metric.ID {
				t.Fatalf("GetMetricJSON() ID = %q, want %q", metric.ID, tt.metric.ID)
			}
			if metric.MType != tt.metric.MType {
				t.Fatalf("GetMetricJSON() MType = %q, want %q", metric.MType, tt.metric.MType)
			}
			assertMetricValue(t, metric.Value, tt.wantValue)
			assertMetricDelta(t, metric.Delta, tt.wantDelta)
		})
	}
}

func TestMetricServiceGetMetricJSONReturnsError(t *testing.T) {
	tests := []struct {
		name    string
		metric  *models.Metric
		wantErr error
	}{
		{
			name:    "nil metric",
			wantErr: ErrNilMetric,
		},
		{
			name: "empty name",
			metric: &models.Metric{
				MType: models.MetricTypeGauge,
			},
			wantErr: ErrInvalidMetricName,
		},
		{
			name: "unknown type",
			metric: &models.Metric{
				ID:    "Alloc",
				MType: "unknown",
			},
			wantErr: ErrUnknownMetricType,
		},
		{
			name: "missing gauge",
			metric: &models.Metric{
				ID:    "Unknown",
				MType: models.MetricTypeGauge,
			},
			wantErr: ErrMetricNotFound,
		},
		{
			name: "missing counter",
			metric: &models.Metric{
				ID:    "Unknown",
				MType: models.MetricTypeCounter,
			},
			wantErr: ErrMetricNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			storage := &fakeStorage{
				gauges:   make(map[string]models.Gauge),
				counters: make(map[string]models.Counter),
			}
			service, err := NewMetricService(storage)
			if err != nil {
				t.Fatalf("NewMetricService() error = %v", err)
			}

			_, err = service.GetMetricJSON(tt.metric)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("GetMetricJSON() error = %v, want %v", err, tt.wantErr)
			}
		})
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

func ptrFloat64(value float64) *float64 {
	return &value
}

func ptrInt64(value int64) *int64 {
	return &value
}

func assertMetricValue(t *testing.T, got, want *float64) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("metric Value = %v, want %v", got, want)
	}
	if *got != *want {
		t.Fatalf("metric Value = %v, want %v", *got, *want)
	}
}

func assertMetricDelta(t *testing.T, got, want *int64) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("metric Delta = %v, want %v", got, want)
	}
	if *got != *want {
		t.Fatalf("metric Delta = %v, want %v", *got, *want)
	}
}
