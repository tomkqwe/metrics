// Package repository defines metric storage contracts and snapshot restoration.
package repository

import "context"

import models "github.com/tomkqwe/metrics/internal/model"

// Storage stores gauge values and cumulative counters.
// Lookup methods return a boolean indicating whether the metric exists.
type Storage interface {
	// UpdateGauge replaces the named gauge value.
	UpdateGauge(ctx context.Context, name string, value models.Gauge) error
	// UpdateCounter adds a delta to the named counter.
	UpdateCounter(ctx context.Context, name string, value models.Counter) error
	// GetGauge returns a gauge value and an existence flag.
	GetGauge(ctx context.Context, name string) (models.Gauge, bool, error)
	// GetCounter returns a counter value and an existence flag.
	GetCounter(ctx context.Context, name string) (models.Counter, bool, error)
	// Snapshot returns all stored metrics.
	Snapshot(ctx context.Context) ([]models.Metric, error)
}

// BatchStorage is the optional storage extension for applying a batch of metrics.
type BatchStorage interface {
	// UpdateMetrics applies gauge replacements and counter increments from a batch.
	UpdateMetrics(ctx context.Context, metrics []models.Metric) error
}
