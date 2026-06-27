package service

import (
	"errors"
	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/repository"
	"strconv"
)

var (
	ErrInvalidStorage    = errors.New("storage is invalid")
	ErrUnknownMetricType = errors.New("unknown metric type")
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
	if metricName == "gauge" {
		float, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return err
		}
		m.storage.UpdateGauge(key, models.Gauge(float))
		return nil
	} else if metricName == "counter" {
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
