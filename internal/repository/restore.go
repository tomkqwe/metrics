package repository

import (
	"fmt"

	models "github.com/tomkqwe/metrics/internal/model"
)

func RestoreMetrics(storage Storage, metrics []models.Metric) error {
	for _, metric := range metrics {
		switch metric.MType {
		case models.MetricTypeGauge:
			if metric.Value == nil {
				return fmt.Errorf("restore gauge %q: missing value", metric.ID)
			}
			storage.UpdateGauge(metric.ID, models.Gauge(*metric.Value))
		case models.MetricTypeCounter:
			if metric.Delta == nil {
				return fmt.Errorf("restore counter %q: missing delta", metric.ID)
			}
			storage.UpdateCounter(metric.ID, models.Counter(*metric.Delta))
		default:
			return fmt.Errorf("restore metric %q: unknown type %q", metric.ID, metric.MType)
		}
	}

	return nil
}
