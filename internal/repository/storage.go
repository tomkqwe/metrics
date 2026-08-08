package repository

import "context"

import models "github.com/tomkqwe/metrics/internal/model"

type Storage interface {
	UpdateGauge(ctx context.Context, name string, value models.Gauge) error
	UpdateCounter(ctx context.Context, name string, value models.Counter) error
	GetGauge(ctx context.Context, name string) (models.Gauge, bool, error)
	GetCounter(ctx context.Context, name string) (models.Counter, bool, error)
	Snapshot(ctx context.Context) ([]models.Metric, error)
}

type BatchStorage interface {
	UpdateMetrics(ctx context.Context, metrics []models.Metric) error
}
