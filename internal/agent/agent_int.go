package agent

import (
	"context"

	models "github.com/tomkqwe/metrics/internal/model"
)

// Collector provides a batch of current measurements.
type Collector interface {
	// Collect returns the current measurements.
	Collect() []models.Metric
}

// Sender delivers metric batches using the supplied context.
type Sender interface {
	// Send delivers the supplied metrics or returns a delivery error.
	Send(context.Context, []models.Metric) error
}

// Storage keeps the latest collected metrics and returns independent snapshots.
type Storage interface {
	// Update stores copies of the supplied metrics.
	Update([]models.Metric)
	// Snapshot returns independent copies of the latest stored measurements.
	Snapshot() []models.Metric
}
