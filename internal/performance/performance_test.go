// Package performance_test benchmarks the same fixed workload before and after
// optimization. Fixture creation is excluded from benchmark timing.
package performance_test

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/tomkqwe/metrics/internal/agent/sender"
	agentstorage "github.com/tomkqwe/metrics/internal/agent/storage"
	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/middleware"
	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/repository/memstorage"
	"github.com/tomkqwe/metrics/internal/service"
)

// A batch of 128 metrics represents runtime statistics from four instances.
func metricBatch() []models.Metric {
	names := []string{"Alloc", "TotalAlloc", "Sys", "Lookups", "Mallocs", "Frees", "HeapAlloc", "HeapSys", "HeapIdle", "HeapInuse", "HeapReleased", "HeapObjects", "StackInuse", "StackSys", "MSpanInuse", "MSpanSys", "MCacheInuse", "MCacheSys", "BuckHashSys", "GCSys", "OtherSys", "NextGC", "LastGC", "GCCPUFraction", "PollCount", "Requests", "Errors", "BytesSent", "BytesReceived", "CacheHits", "CacheMisses", "Jobs"}
	metrics := make([]models.Metric, 0, 128)
	for instance := 0; instance < 4; instance++ {
		for i, name := range names {
			m := models.Metric{ID: fmt.Sprintf("instance_%d_%s", instance, name)}
			if i < 24 {
				value := float64((instance+1)*(i+1)*1024) + 0.125
				m.MType, m.Value = models.MetricTypeGauge, &value
			} else {
				delta := int64(i + 1)
				m.MType, m.Delta = models.MetricTypeCounter, &delta
			}
			metrics = append(metrics, m)
		}
	}
	return metrics
}

func BenchmarkServiceUpdateBatch(b *testing.B) {
	metrics := metricBatch()
	srv, err := service.NewMetricService(memstorage.NewMemStorage())
	if err != nil {
		b.Fatal(err)
	}
	ctx := context.Background()
	if err := srv.UpdateMetricsJSON(ctx, metrics); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := srv.UpdateMetricsJSON(ctx, metrics); err != nil {
			b.Fatal(err)
		}
	}
}

var snapshot []models.Metric

func BenchmarkServerSnapshot(b *testing.B) {
	storage := memstorage.NewMemStorage()
	ctx := context.Background()
	if err := storage.UpdateMetrics(ctx, metricBatch()); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var err error
		snapshot, err = storage.Snapshot(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAgentSnapshot(b *testing.B) {
	storage := agentstorage.NewMemoryStorage()
	storage.Update(metricBatch())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		snapshot = storage.Snapshot()
	}
}

// BenchmarkPipeline exercises actual loopback HTTP: agent JSON/gzip encoding,
// server decompression, routing, JSON decoding, validation and batch storage.
// No database, file persistence or audit receivers are configured.
func BenchmarkPipeline(b *testing.B) {
	metrics := metricBatch()
	storage := memstorage.NewMemStorage()
	srv, err := service.NewMetricService(storage)
	if err != nil {
		b.Fatal(err)
	}
	h, err := handler.NewMetricsHandler(srv)
	if err != nil {
		b.Fatal(err)
	}
	router := chi.NewRouter()
	router.Use(middleware.WithGzip)
	router.Post("/updates/", h.UpdateMetricsJSON)
	server := httptest.NewServer(router)
	defer server.Close()
	client := server.Client()
	defer client.CloseIdleConnections()
	agent := sender.NewHTTPSenderWithClient(server.URL, client)
	ctx := context.Background()
	// Warm the HTTP connection and populate the store before measuring.
	if err := agent.Send(ctx, metrics); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := agent.Send(ctx, metrics); err != nil {
			b.Fatal(err)
		}
	}
	b.StopTimer()
	result, err := storage.Snapshot(ctx)
	if err != nil || len(result) != len(metrics) {
		b.Fatalf("snapshot length=%d error=%v", len(result), err)
	}
}
