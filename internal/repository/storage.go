package repository

import models "github.com/tomkqwe/metrics/internal/model"

type Storage interface {
	UpdateGauge(name string, value models.Gauge)
	UpdateCounter(name string, value models.Counter)
	GetGauge(name string) (models.Gauge, bool)
	GetCounter(name string) (models.Counter, bool)
	Snapshot() []models.Metric
}

type BatchStorage interface {
	UpdateMetrics(metrics []models.Metric)
}
