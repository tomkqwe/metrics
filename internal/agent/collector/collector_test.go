package collector

import (
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestRuntimeCollectorCollectReturnsRequiredMetrics(t *testing.T) {
	c := NewRuntimeCollector()

	metrics := c.Collect()

	byName := metricsByName(metrics)
	for _, name := range requiredRuntimeGaugeNames() {
		metric, ok := byName[name]
		if !ok {
			t.Fatalf("metric %q was not collected", name)
		}
		assertGaugeMetric(t, metric)
	}

	pollCount, ok := byName["PollCount"]
	if !ok {
		t.Fatal("metric PollCount was not collected")
	}
	if pollCount.MType != models.MetricTypeCounter {
		t.Fatalf("PollCount type = %q, want %q", pollCount.MType, models.MetricTypeCounter)
	}
	if pollCount.Delta == nil {
		t.Fatal("PollCount Delta is nil")
	}
	if *pollCount.Delta != 1 {
		t.Fatalf("PollCount Delta = %d, want 1", *pollCount.Delta)
	}
	if pollCount.Value != nil {
		t.Fatal("PollCount Value must be nil")
	}

	randomValue, ok := byName["RandomValue"]
	if !ok {
		t.Fatal("metric RandomValue was not collected")
	}
	assertGaugeMetric(t, randomValue)
	if *randomValue.Value < 0 || *randomValue.Value >= 1 {
		t.Fatalf("RandomValue = %f, want value in [0, 1)", *randomValue.Value)
	}
}

func TestRuntimeCollectorPollCountIncrements(t *testing.T) {
	c := NewRuntimeCollector()

	first := metricsByName(c.Collect())
	second := metricsByName(c.Collect())

	if *first["PollCount"].Delta != 1 {
		t.Fatalf("first PollCount = %d, want 1", *first["PollCount"].Delta)
	}
	if *second["PollCount"].Delta != 2 {
		t.Fatalf("second PollCount = %d, want 2", *second["PollCount"].Delta)
	}
}

func metricsByName(metrics []models.Metric) map[string]models.Metric {
	byName := make(map[string]models.Metric, len(metrics))
	for _, metric := range metrics {
		byName[metric.ID] = metric
	}
	return byName
}

func assertGaugeMetric(t *testing.T, metric models.Metric) {
	t.Helper()

	if metric.MType != models.MetricTypeGauge {
		t.Fatalf("%s type = %q, want %q", metric.ID, metric.MType, models.MetricTypeGauge)
	}
	if metric.Value == nil {
		t.Fatalf("%s Value is nil", metric.ID)
	}
	if metric.Delta != nil {
		t.Fatalf("%s Delta must be nil", metric.ID)
	}
}

func requiredRuntimeGaugeNames() []string {
	return []string{
		"Alloc",
		"BuckHashSys",
		"Frees",
		"GCCPUFraction",
		"GCSys",
		"HeapAlloc",
		"HeapIdle",
		"HeapInuse",
		"HeapObjects",
		"HeapReleased",
		"HeapSys",
		"LastGC",
		"Lookups",
		"MCacheInuse",
		"MCacheSys",
		"MSpanInuse",
		"MSpanSys",
		"Mallocs",
		"NextGC",
		"NumForcedGC",
		"NumGC",
		"OtherSys",
		"PauseTotalNs",
		"StackInuse",
		"StackSys",
		"Sys",
		"TotalAlloc",
	}
}
