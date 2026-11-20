package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tomkqwe/metrics/internal/agent/storage"
	models "github.com/tomkqwe/metrics/internal/model"
)

type collectorFunc func() []models.Metric

func (f collectorFunc) Collect() []models.Metric { return f() }

type senderFunc func(context.Context, []models.Metric) error

func (f senderFunc) Send(ctx context.Context, metrics []models.Metric) error { return f(ctx, metrics) }

func TestShutdownDrainsReportsAndFinalCollection(t *testing.T) {
	collected := make(chan struct{})
	finishCollection := make(chan struct{})
	sending := make(chan struct{})
	finishSend := make(chan struct{})
	memory := storage.NewMemoryStorage()
	old, latest := 1.0, 2.0
	memory.Update([]models.Metric{{ID: "Alloc", MType: models.MetricTypeGauge, Value: &old}})
	collector := collectorFunc(func() []models.Metric {
		close(collected)
		<-finishCollection
		return []models.Metric{{ID: "Alloc", MType: models.MetricTypeGauge, Value: &latest}}
	})
	var values []float64
	sender := senderFunc(func(ctx context.Context, metrics []models.Metric) error {
		if len(values) == 0 {
			close(sending)
			<-finishSend
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		values = append(values, *metrics[0].Value)
		return nil
	})
	app, err := NewAgent(collector, memory, sender, time.Hour, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- app.Run(ctx) }()
	awaitShutdownEvent(t, collected)
	awaitShutdownEvent(t, sending)
	cancel()
	close(finishCollection)
	select {
	case err := <-done:
		t.Fatalf("returned before active send finished: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(finishSend)
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown stuck")
	}
	if len(values) != 2 || values[0] != old || values[1] != latest {
		t.Fatalf("sent values: %v", values)
	}
}

func TestShutdownReturnsFinalDeliveryError(t *testing.T) {
	want := errors.New("delivery failed")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	app, err := NewAgent(&fakeCollector{}, storage.NewMemoryStorage(), senderFunc(func(ctx context.Context, _ []models.Metric) error {
		if ctx.Err() != nil {
			t.Error("final context cancelled")
		}
		return want
	}), time.Hour, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.Run(ctx); !errors.Is(err, want) {
		t.Fatalf("error = %v", err)
	}
}

func awaitShutdownEvent(t *testing.T, event <-chan struct{}) {
	t.Helper()
	select {
	case <-event:
	case <-time.After(3 * time.Second):
		t.Fatal("event timeout")
	}
}
