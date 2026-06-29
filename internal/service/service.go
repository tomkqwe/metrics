package service

import models "github.com/tomkqwe/metrics/internal/model"

type Service interface {
	UpdateMetric(metricName, key, value string) error
	GetMetricValue(metricType, metricName string) (string, error)
	ListMetrics() []models.Metric
}
