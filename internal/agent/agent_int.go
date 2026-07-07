package agent

import models "github.com/tomkqwe/metrics/internal/model"

type Collector interface {
	Collect() []models.Metric
}

type Sender interface {
	Send([]models.Metric) error
}

type Storage interface {
	Update([]models.Metric)
	Snapshot() []models.Metric
}
