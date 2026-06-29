package service

import (
	"errors"
	"strconv"

	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/repository"
)

var (
	ErrInvalidStorage    = errors.New("storage is invalid")
	ErrUnknownMetricType = errors.New("unknown metric type")
	ErrMetricNotFound    = errors.New("metric not found")
)

type MetricService struct {
	storage repository.Storage
}

func NewMetricService(storage repository.Storage) (*MetricService, error) {
	if storage == nil || storage == repository.Storage(nil) {
		return nil, ErrInvalidStorage
	}
	return &MetricService{storage: storage}, nil
}

func (m *MetricService) UpdateMetric(metricName, key, value string) error {
	if metricName == models.MetricTypeGauge {
		float, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		m.storage.UpdateGauge(key, models.Gauge(float))
		return nil
	} else if metricName == models.MetricTypeCounter {
		i, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return err
		}
		m.storage.UpdateCounter(key, models.Counter(i))
	} else {
		return ErrUnknownMetricType
	}
	return nil
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
