package service

import (
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
	for _, option := range options {
		option(service)
	}

	return service, nil
}

func (m *MetricService) UpdateMetric(metricType, metricName, value string) error {
	switch metricType {
	case models.MetricTypeGauge:
		float, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		return m.updateAndPersist(func() {
			m.storage.UpdateGauge(metricName, models.Gauge(float))
		})
	case models.MetricTypeCounter:
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		return m.updateAndPersist(func() {
			m.storage.UpdateCounter(metricName, models.Counter(i))
		})
	default:
		return ErrUnknownMetricType
	}
}

func (m *MetricService) GetMetricValue(metricType, metricName string) (string, error) {
	switch metricType {
	case models.MetricTypeGauge:
		value, ok := m.storage.GetGauge(metricName)
		if !ok {
			return "", ErrMetricNotFound
		}
		return strconv.FormatFloat(float64(value), 'f', -1, 64), nil
	case models.MetricTypeCounter:
		value, ok := m.storage.GetCounter(metricName)
		if !ok {
			return "", ErrMetricNotFound
		}
		return strconv.FormatInt(int64(value), 10), nil
	default:
		return "", ErrUnknownMetricType
	}
}

func (m *MetricService) ListMetrics() []models.Metric {
	return m.storage.Snapshot()
}

func (m *MetricService) UpdateMetricJson(metric *models.Metric) error {
	if metric == nil {
		return ErrNilMetric
	}
	if metric.ID == "" {
		return ErrInvalidMetricName
	}

	switch metric.MType {
	case models.MetricTypeGauge:
		if metric.Value == nil {
			return ErrInvalidMetricValue
		}
		return m.updateAndPersist(func() {
			m.storage.UpdateGauge(metric.ID, models.Gauge(*metric.Value))
		})
	case models.MetricTypeCounter:
		if metric.Delta == nil {
			return ErrInvalidMetricValue
		}
		return m.updateAndPersist(func() {
			m.storage.UpdateCounter(metric.ID, models.Counter(*metric.Delta))
		})
	default:
		return ErrUnknownMetricType
	}
}

func (m *MetricService) GetMetricJson(metric *models.Metric) (models.Metric, error) {
	if metric == nil {
		return models.Metric{}, ErrNilMetric
	}
	if metric.ID == "" {
		return models.Metric{}, ErrInvalidMetricName
	}
	switch metric.MType {
	case models.MetricTypeGauge:
		gauge, b := m.storage.GetGauge(metric.ID)
		if !b {
			return models.Metric{}, ErrMetricNotFound
		}
		value := float64(gauge)
		return models.Metric{ID: metric.ID, MType: metric.MType, Value: &value}, nil

	case models.MetricTypeCounter:
		counter, b := m.storage.GetCounter(metric.ID)
		if !b {
			return models.Metric{}, ErrMetricNotFound
		}
		delta := int64(counter)
		return models.Metric{ID: metric.ID, MType: metric.MType, Delta: &delta}, nil
	default:
		return models.Metric{}, ErrUnknownMetricType
	}
}

func (m *MetricService) updateAndPersist(update func()) error {
	if m.saveOnUpdate == nil {
		update()
		return nil
	}

	m.saveMu.Lock()
	defer m.saveMu.Unlock()

	update()
	return m.persist()
}

func (m *MetricService) persist() error {
	if m.saveOnUpdate == nil {
		return nil
	}

	return m.saveOnUpdate(m.storage.Snapshot())
}
