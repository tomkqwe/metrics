package models

import "testing"

func TestMetricTypeConstants(t *testing.T) {
	if MetricTypeGauge != "gauge" {
		t.Fatalf("MetricTypeGauge = %q, want gauge", MetricTypeGauge)
	}
	if MetricTypeCounter != "counter" {
		t.Fatalf("MetricTypeCounter = %q, want counter", MetricTypeCounter)
	}
}
