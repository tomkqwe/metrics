package storage

import (
	"sort"
	"sync"

	models "github.com/tomkqwe/metrics/internal/model"
)

type MemoryStorage struct {
	mu      sync.RWMutex
	metrics map[metricKey]models.Metric
}

type metricKey struct {
	mType string
	id    string
}

func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		metrics: make(map[metricKey]models.Metric),
	}
}

func (s *MemoryStorage) Update(metrics []models.Metric) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, metric := range metrics {
		s.metrics[keyFromMetric(metric)] = cloneMetric(metric)
	}
}

func (s *MemoryStorage) Snapshot() []models.Metric {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]metricKey, 0, len(s.metrics))
	for key := range s.metrics {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].mType == keys[j].mType {
			return keys[i].id < keys[j].id
		}
		return keys[i].mType < keys[j].mType
	})

	metrics := make([]models.Metric, 0, len(s.metrics))
	for _, key := range keys {
		metrics = append(metrics, cloneMetric(s.metrics[key]))
	}

	return metrics
}

func keyFromMetric(metric models.Metric) metricKey {
	return metricKey{
		mType: metric.MType,
		id:    metric.ID,
	}
}

func cloneMetric(metric models.Metric) models.Metric {
	clone := metric

	if metric.Value != nil {
		value := *metric.Value
		clone.Value = &value
	}
	if metric.Delta != nil {
		delta := *metric.Delta
		clone.Delta = &delta
	}

	return clone
}
