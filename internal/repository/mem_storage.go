package repository

import "github.com/tomkqwe/metrics/internal/model"

type MemStorage struct {
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
	m.gaugeStore[name] = value
}

func (m *MemStorage) UpdateCounter(name string, value models.Counter) {
	m.counterStore[name] = value
}
