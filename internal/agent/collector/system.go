package collector

import (
	"fmt"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	models "github.com/tomkqwe/metrics/internal/model"
)

type virtualMemoryFunc func() (*mem.VirtualMemoryStat, error)
type cpuPercentFunc func(interval time.Duration, percpu bool) ([]float64, error)

type SystemCollector struct {
	virtualMemory virtualMemoryFunc
	cpuPercent    cpuPercentFunc
}

func NewSystemCollector() *SystemCollector {
	return &SystemCollector{
		virtualMemory: mem.VirtualMemory,
		cpuPercent:    cpu.Percent,
	}
}

func (c *SystemCollector) Collect() []models.Metric {
	metrics := make([]models.Metric, 0, 2+runtime.NumCPU())

	if memory, err := c.collectMemory(); err == nil {
		metrics = append(metrics,
			newGaugeMetric("TotalMemory", float64(memory.Total)),
			newGaugeMetric("FreeMemory", float64(memory.Free)),
		)
	}

	if utilization, err := c.collectCPU(); err == nil {
		for i, value := range utilization {
			metrics = append(metrics, newGaugeMetric(fmt.Sprintf("CPUutilization%d", i+1), value))
		}
	}

	return metrics
}

func (c *SystemCollector) collectMemory() (*mem.VirtualMemoryStat, error) {
	if c.virtualMemory != nil {
		return c.virtualMemory()
	}
	return mem.VirtualMemory()
}

func (c *SystemCollector) collectCPU() ([]float64, error) {
	if c.cpuPercent != nil {
		return c.cpuPercent(0, true)
	}
	return cpu.Percent(0, true)
}
