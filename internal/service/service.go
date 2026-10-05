// Package service validates metrics and coordinates storage and optional persistence.
package service

import (
	"context"

	models "github.com/tomkqwe/metrics/internal/model"
)

// Service defines metric updates and queries used by HTTP handlers.
// Gauge updates replace values; counter updates add deltas.
type Service interface {
	// UpdateMetric parses a textual value and updates a metric.
	UpdateMetric(ctx context.Context, metricType, metricName, value string) error
	// GetMetricValue returns the current value as text.
	GetMetricValue(ctx context.Context, metricType, metricName string) (string, error)
	// ListMetrics returns the stored metric snapshot.
	ListMetrics(ctx context.Context) ([]models.Metric, error)
	// UpdateMetricJSON validates and updates one JSON metric.
	UpdateMetricJSON(ctx context.Context, metric *models.Metric) error
	// UpdateMetricsJSON validates and updates a batch of metrics.
	UpdateMetricsJSON(ctx context.Context, metrics []models.Metric) error
	// GetMetricJSON looks up a metric by ID and type.
	GetMetricJSON(ctx context.Context, metric *models.Metric) (models.Metric, error)
}
