package service

type Service interface {
	UpdateMetric(metricName, key, value string) error
}
