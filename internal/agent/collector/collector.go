package collector

import (
	"math/rand"
	"runtime"
	"time"

	models "github.com/tomkqwe/metrics/internal/model"
)

type RuntimeCollector struct {
	pollCount int64
	rand      *rand.Rand
}

func NewRuntimeCollector() *RuntimeCollector {
	return &RuntimeCollector{
		rand: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func (c *RuntimeCollector) Collect() []models.Metric {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	c.pollCount++

	return []models.Metric{
		newGaugeMetric("Alloc", float64(memStats.Alloc)),
		newGaugeMetric("BuckHashSys", float64(memStats.BuckHashSys)),
		newGaugeMetric("Frees", float64(memStats.Frees)),
		newGaugeMetric("GCCPUFraction", memStats.GCCPUFraction),
		newGaugeMetric("GCSys", float64(memStats.GCSys)),
		newGaugeMetric("HeapAlloc", float64(memStats.HeapAlloc)),
		newGaugeMetric("HeapIdle", float64(memStats.HeapIdle)),
		newGaugeMetric("HeapInuse", float64(memStats.HeapInuse)),
		newGaugeMetric("HeapObjects", float64(memStats.HeapObjects)),
		newGaugeMetric("HeapReleased", float64(memStats.HeapReleased)),
		newGaugeMetric("HeapSys", float64(memStats.HeapSys)),
		newGaugeMetric("LastGC", float64(memStats.LastGC)),
		newGaugeMetric("Lookups", float64(memStats.Lookups)),
		newGaugeMetric("MCacheInuse", float64(memStats.MCacheInuse)),
		newGaugeMetric("MCacheSys", float64(memStats.MCacheSys)),
		newGaugeMetric("MSpanInuse", float64(memStats.MSpanInuse)),
		newGaugeMetric("MSpanSys", float64(memStats.MSpanSys)),
		newGaugeMetric("Mallocs", float64(memStats.Mallocs)),
		newGaugeMetric("NextGC", float64(memStats.NextGC)),
		newGaugeMetric("NumForcedGC", float64(memStats.NumForcedGC)),
		newGaugeMetric("NumGC", float64(memStats.NumGC)),
		newGaugeMetric("OtherSys", float64(memStats.OtherSys)),
		newGaugeMetric("PauseTotalNs", float64(memStats.PauseTotalNs)),
		newGaugeMetric("StackInuse", float64(memStats.StackInuse)),
		newGaugeMetric("StackSys", float64(memStats.StackSys)),
		newGaugeMetric("Sys", float64(memStats.Sys)),
		newGaugeMetric("TotalAlloc", float64(memStats.TotalAlloc)),
		newCounterMetric("PollCount", c.pollCount),
		newGaugeMetric("RandomValue", c.rand.Float64()),
	}
}

func newGaugeMetric(name string, value float64) models.Metric {
	return models.Metric{
		ID:    name,
		MType: models.MetricTypeGauge,
		Value: &value,
	}
}

func newCounterMetric(name string, delta int64) models.Metric {
	return models.Metric{
		ID:    name,
		MType: models.MetricTypeCounter,
		Delta: &delta,
	}
}
