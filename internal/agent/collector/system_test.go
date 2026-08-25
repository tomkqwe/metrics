package collector

import (
	"testing"
	"time"

	"github.com/shirou/gopsutil/v3/mem"
	models "github.com/tomkqwe/metrics/internal/model"
)

func TestSystemCollectorCollectReturnsGopsutilMetrics(t *testing.T) {
	c := &SystemCollector{
		virtualMemory: func() (*mem.VirtualMemoryStat, error) {
			return &mem.VirtualMemoryStat{
				Total: 100,
				Free:  40,
			}, nil
		},
		cpuPercent: func(interval time.Duration, percpu bool) ([]float64, error) {
			if interval != 0 {
				t.Fatalf("cpu interval = %v, want 0", interval)
			}
			if !percpu {
				t.Fatal("cpu percpu = false, want true")
			}
			return []float64{10.5, 20.25}, nil
		},
	}

	metrics := metricsByName(c.Collect())

	assertGaugeValue(t, metrics["TotalMemory"], 100)
	assertGaugeValue(t, metrics["FreeMemory"], 40)
	assertGaugeValue(t, metrics["CPUutilization1"], 10.5)
	assertGaugeValue(t, metrics["CPUutilization2"], 20.25)
}

func assertGaugeValue(t *testing.T, metric models.Metric, want float64) {
	t.Helper()

	assertGaugeMetric(t, metric)
	if *metric.Value != want {
		t.Fatalf("%s Value = %f, want %f", metric.ID, *metric.Value, want)
	}
}
