package service

import (
	"context"
	"errors"
	"strconv"
	"sync"

	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/repository"
)

var (
	ErrInvalidStorage     = errors.New("storage is invalid")
	ErrUnknownMetricType  = errors.New("unknown metric type")
	ErrMetricNotFound     = errors.New("metric not found")
	ErrNilMetric          = errors.New("nil metric")
	ErrInvalidMetricName  = errors.New("invalid metric name")
	ErrInvalidMetricValue = errors.New("invalid metric value")
)

type MetricService struct {
	storage      repository.Storage
	batchStorage repository.BatchStorage
	saveOnUpdate func([]models.Metric) error
	saveMu       sync.Mutex
}

type MetricServiceOption func(*MetricService)

func WithUpdatePersister(save func([]models.Metric) error) MetricServiceOption {
	return func(service *MetricService) {
		service.saveOnUpdate = save
	}
}

func NewMetricService(storage repository.Storage, options ...MetricServiceOption) (*MetricService, error) {
	if storage == nil {
		return nil, ErrInvalidStorage
	}
	service := &MetricService{storage: storage}
	if batchStorage, ok := storage.(repository.BatchStorage); ok {
		service.batchStorage = batchStorage
	}
	for _, option := range options {
		option(service)
	}

	return service, nil
}

func (m *MetricService) UpdateMetric(ctx context.Context, metricType, metricName, value string) error {
	switch metricType {
	case models.MetricTypeGauge:
		float, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		return m.updateAndPersist(ctx, func(ctx context.Context) error {
			return m.storage.UpdateGauge(ctx, metricName, models.Gauge(float))
		})
	case models.MetricTypeCounter:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		return m.updateAndPersist(ctx, func(ctx context.Context) error {
			return m.storage.UpdateCounter(ctx, metricName, models.Counter(i))
		})
	default:
		return ErrUnknownMetricType
	}
}

func (m *MetricService) GetMetricValue(ctx context.Context, metricType, metricName string) (string, error) {
	switch metricType {
	case models.MetricTypeGauge:
		value, ok, err := m.storage.GetGauge(ctx, metricName)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", ErrMetricNotFound
		}
		return strconv.FormatFloat(float64(value), 'f', -1, 64), nil
	case models.MetricTypeCounter:
		value, ok, err := m.storage.GetCounter(ctx, metricName)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", ErrMetricNotFound
		}
		return strconv.FormatInt(int64(value), 10), nil
	default:
		return "", ErrUnknownMetricType
	}
}

func (m *MetricService) ListMetrics(ctx context.Context) ([]models.Metric, error) {
	return m.storage.Snapshot(ctx)
}

func (m *MetricService) UpdateMetricJSON(ctx context.Context, metric *models.Metric) error {
	if metric == nil {
		return ErrNilMetric
	}
	if err := validateMetric(*metric); err != nil {
		return err
	}

	return m.updateAndPersist(ctx, func(ctx context.Context) error {
		return m.updateMetric(ctx, *metric)
	})
}

func (m *MetricService) UpdateMetricsJSON(ctx context.Context, metrics []models.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	for _, metric := range metrics {
		if err := validateMetric(metric); err != nil {
			return err
		}
	}

	return m.updateAndPersist(ctx, func(ctx context.Context) error {
		return m.updateMetrics(ctx, metrics)
	})
}

func (m *MetricService) GetMetricJSON(ctx context.Context, metric *models.Metric) (models.Metric, error) {
	if metric == nil {
		return models.Metric{}, ErrNilMetric
	}
	if metric.ID == "" {
		return models.Metric{}, ErrInvalidMetricName
	}
	switch metric.MType {
	case models.MetricTypeGauge:
		gauge, ok, err := m.storage.GetGauge(ctx, metric.ID)
		if err != nil {
			return models.Metric{}, err
		}
		if !ok {
			return models.Metric{}, ErrMetricNotFound
		}
		value := float64(gauge)
		return models.Metric{ID: metric.ID, MType: metric.MType, Value: &value}, nil

	case models.MetricTypeCounter:
		counter, ok, err := m.storage.GetCounter(ctx, metric.ID)
		if err != nil {
			return models.Metric{}, err
		}
		if !ok {
			return models.Metric{}, ErrMetricNotFound
		}
		delta := int64(counter)
		return models.Metric{ID: metric.ID, MType: metric.MType, Delta: &delta}, nil
	default:
		return models.Metric{}, ErrUnknownMetricType
	}
}

func (m *MetricService) updateAndPersist(ctx context.Context, update func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if m.saveOnUpdate == nil {
		return update(ctx)
	}

	m.saveMu.Lock()
	defer m.saveMu.Unlock()

	if err := update(ctx); err != nil {
		return err
	}

	metrics, err := m.storage.Snapshot(ctx)
	if err != nil {
		return err
	}

	return m.saveOnUpdate(metrics)
}

func (m *MetricService) updateMetrics(ctx context.Context, metrics []models.Metric) error {
	if m.batchStorage != nil {
		return m.batchStorage.UpdateMetrics(ctx, metrics)
	}

	for _, metric := range metrics {
		if err := m.updateMetric(ctx, metric); err != nil {
			return err
		}
	}

	return nil
}

func (m *MetricService) updateMetric(ctx context.Context, metric models.Metric) error {
	switch metric.MType {
	case models.MetricTypeGauge:
		return m.storage.UpdateGauge(ctx, metric.ID, models.Gauge(*metric.Value))
	case models.MetricTypeCounter:
		return m.storage.UpdateCounter(ctx, metric.ID, models.Counter(*metric.Delta))
	}

	return ErrUnknownMetricType
}

func validateMetric(metric models.Metric) error {
	if metric.ID == "" {
		return ErrInvalidMetricName
	}

	switch metric.MType {
	case models.MetricTypeGauge:
		if metric.Value == nil {
			return ErrInvalidMetricValue
		}
	case models.MetricTypeCounter:
		if metric.Delta == nil {
			return ErrInvalidMetricValue
		}
	default:
		return ErrUnknownMetricType
	}

	return nil
}
