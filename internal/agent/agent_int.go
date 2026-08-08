package agent

import (
	"context"

	models "github.com/tomkqwe/metrics/internal/model"
)

type Collector interface {
	Collect() []models.Metric
}

type Sender interface {
	Send(context.Context, []models.Metric) error
}

type Storage interface {
	Update([]models.Metric)
	Snapshot() []models.Metric
}
