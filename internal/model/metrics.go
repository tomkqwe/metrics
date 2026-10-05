// Package models defines metric types and their JSON representation.
package models

const (
	// MetricTypeCounter identifies an integer metric updated by adding deltas.
	MetricTypeCounter = "counter"
	// MetricTypeGauge identifies a floating-point metric replaced on update.
	MetricTypeGauge = "gauge"
)

// Metrics is the JSON representation of a named gauge or counter.
// Pointer values distinguish a missing field from an explicitly supplied zero.
type Metrics struct {
	// ID is the metric name.
	ID string `json:"id"`
	// MType is MetricTypeGauge or MetricTypeCounter.
	MType string `json:"type"`
	// Delta is the counter increment on update or its accumulated value on read.
	Delta *int64 `json:"delta,omitempty"`
	// Value is the gauge measurement.
	Value *float64 `json:"value,omitempty"`
	// Hash is an optional hash field in the metric JSON representation.
	Hash string `json:"hash,omitempty"`
}

// Metric is an alias for the JSON metric representation.
type Metric = Metrics

// Gauge is a floating-point measurement.
type Gauge float64

// Counter is an accumulated integer measurement.
type Counter int64
