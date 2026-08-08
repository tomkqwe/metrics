package service

import (
	"context"

	models "github.com/tomkqwe/metrics/internal/model"
)

type Service interface {
	UpdateMetric(ctx context.Context, metricType, metricName, value string) error
	GetMetricValue(ctx context.Context, metricType, metricName string) (string, error)
	ListMetrics(ctx context.Context) ([]models.Metric, error)
	UpdateMetricJSON(ctx context.Context, metric *models.Metric) error
	UpdateMetricsJSON(ctx context.Context, metrics []models.Metric) error
	GetMetricJSON(ctx context.Context, metric *models.Metric) (models.Metric, error)
}
