package repository

import (
	"sort"
	"sync"

	"github.com/tomkqwe/metrics/internal/model"
)

type MemStorage struct {
	mu           sync.RWMutex
	gaugeStore   map[string]models.Gauge   // name => value
	counterStore map[string]models.Counter // name => value
}

func NewMemStorage() *MemStorage {
	return &MemStorage{
		gaugeStore:   make(map[string]models.Gauge),
		counterStore: make(map[string]models.Counter),
	}
}

func (m *MemStorage) UpdateGauge(name string, value models.Gauge) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.gaugeStore[name] = value
}

func (m *MemStorage) UpdateCounter(name string, value models.Counter) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counterStore[name] += value
}

func (m *MemStorage) GetGauge(name string) (models.Gauge, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, ok := m.gaugeStore[name]
	return value, ok
}

func (m *MemStorage) GetCounter(name string) (models.Counter, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	value, ok := m.counterStore[name]
	return value, ok
}

func (m *MemStorage) Snapshot() []models.Metric {
	m.mu.RLock()
	metrics := make([]models.Metric, 0, len(m.gaugeStore)+len(m.counterStore))
	for name, value := range m.gaugeStore {
		gaugeValue := float64(value)
		metrics = append(metrics, models.Metric{
			ID:    name,
			MType: models.MetricTypeGauge,
			Value: &gaugeValue,
		})
	}
	for name, value := range m.counterStore {
		counterValue := int64(value)
		metrics = append(metrics, models.Metric{
			ID:    name,
			MType: models.MetricTypeCounter,
			Delta: &counterValue,
		})
	}
	m.mu.RUnlock()

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].MType == metrics[j].MType {
			return metrics[i].ID < metrics[j].ID
		}
		return metrics[i].MType < metrics[j].MType
	})

	return metrics
}
