package service

import models "github.com/tomkqwe/metrics/internal/model"

type Service interface {
	UpdateMetric(metricType, metricName, value string) error
	GetMetricValue(metricType, metricName string) (string, error)
	ListMetrics() []models.Metric
	UpdateMetricJson(metric *models.Metric) error
	GetMetricJson(metric *models.Metric) (models.Metric, error)
}
