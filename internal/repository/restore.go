package repository

import (
	"context"
	"fmt"

	models "github.com/tomkqwe/metrics/internal/model"
)

func RestoreMetrics(ctx context.Context, storage Storage, metrics []models.Metric) error {
	if ctx == nil {
		ctx = context.Background()
	}

	for _, metric := range metrics {
		switch metric.MType {
		case models.MetricTypeGauge:
			if metric.Value == nil {
				return fmt.Errorf("restore gauge %q: missing value", metric.ID)
			}
			if err := storage.UpdateGauge(ctx, metric.ID, models.Gauge(*metric.Value)); err != nil {
				return fmt.Errorf("restore gauge %q: %w", metric.ID, err)
			}
		case models.MetricTypeCounter:
			if metric.Delta == nil {
				return fmt.Errorf("restore counter %q: missing delta", metric.ID)
			}
			if err := storage.UpdateCounter(ctx, metric.ID, models.Counter(*metric.Delta)); err != nil {
				return fmt.Errorf("restore counter %q: %w", metric.ID, err)
			}
		default:
			return fmt.Errorf("restore metric %q: unknown type %q", metric.ID, metric.MType)
		}
	}

	return nil
}
