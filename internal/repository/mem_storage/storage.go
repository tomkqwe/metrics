package mem_storage

import (
	"context"
	"sort"
	"sync"

	"github.com/tomkqwe/metrics/internal/model"
)

type Storage struct {
	mu           sync.RWMutex
	gaugeStore   map[string]models.Gauge   // name => value
	counterStore map[string]models.Counter // name => value
}

func NewMemStorage() *Storage {
	return &Storage{
		gaugeStore:   make(map[string]models.Gauge),
		counterStore: make(map[string]models.Counter),
	}
}

func (s *Storage) UpdateGauge(_ context.Context, name string, value models.Gauge) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.gaugeStore[name] = value
	return nil
}

func (s *Storage) UpdateCounter(_ context.Context, name string, value models.Counter) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counterStore[name] += value
	return nil
}

func (s *Storage) UpdateMetrics(_ context.Context, metrics []models.Metric) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, metric := range metrics {
		switch metric.MType {
		case models.MetricTypeGauge:
			if metric.Value != nil {
				s.gaugeStore[metric.ID] = models.Gauge(*metric.Value)
			}
		case models.MetricTypeCounter:
			if metric.Delta != nil {
				s.counterStore[metric.ID] += models.Counter(*metric.Delta)
			}
		}
	}
	return nil
}

func (s *Storage) GetGauge(_ context.Context, name string) (models.Gauge, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.gaugeStore[name]
	return value, ok, nil
}

func (s *Storage) GetCounter(_ context.Context, name string) (models.Counter, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.counterStore[name]
	return value, ok, nil
}

func (s *Storage) Snapshot(_ context.Context) ([]models.Metric, error) {
	s.mu.RLock()
	metrics := make([]models.Metric, 0, len(s.gaugeStore)+len(s.counterStore))
	for name, value := range s.gaugeStore {
		gaugeValue := float64(value)
		metrics = append(metrics, models.Metric{
			ID:    name,
			MType: models.MetricTypeGauge,
			Value: &gaugeValue,
		})
	}
	for name, value := range s.counterStore {
		counterValue := int64(value)
		metrics = append(metrics, models.Metric{
			ID:    name,
			MType: models.MetricTypeCounter,
			Delta: &counterValue,
		})
	}
	s.mu.RUnlock()

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].MType == metrics[j].MType {
			return metrics[i].ID < metrics[j].ID
		}
		return metrics[i].MType < metrics[j].MType
	})

	return metrics, nil
}
