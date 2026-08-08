package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestAgentPollOnceUpdatesStorageWithCollectedMetrics(t *testing.T) {
	value := 12.5
	metrics := []models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &value,
		},
	}
	storage := &fakeStorage{}
	app, err := NewAgent(&fakeCollector{metrics: metrics}, storage, &fakeSender{}, time.Second, time.Second)
	if err != nil {
		t.Fatalf("NewAgent() error = %v", err)
	}

	app.PollOnce()

	if len(storage.updatedWith) != 1 {
		t.Fatalf("Update() metrics len = %d, want 1", len(storage.updatedWith))
	}
	if storage.updatedWith[0].ID != "Alloc" {
		t.Fatalf("Update() metric ID = %q, want Alloc", storage.updatedWith[0].ID)
	}
}

func TestAgentReportOnceSendsStorageSnapshot(t *testing.T) {
	value := 12.5
	metrics := []models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &value,
		},
	}
	sender := &fakeSender{}
	app, err := NewAgent(&fakeCollector{}, &fakeStorage{snapshot: metrics}, sender, time.Second, time.Second)
	if err != nil {
		t.Fatalf("NewAgent() error = %v", err)
	}

	err = app.ReportOnce(context.Background())
	if err != nil {
		t.Fatalf("ReportOnce() error = %v", err)
	}

	if len(sender.sent) != 1 {
		t.Fatalf("Send() metrics len = %d, want 1", len(sender.sent))
	}
	if sender.sent[0].ID != "Alloc" {
		t.Fatalf("Send() metric ID = %q, want Alloc", sender.sent[0].ID)
	}
}

func TestAgentReportOnceReturnsSenderError(t *testing.T) {
	wantErr := errors.New("send failed")
	app, err := NewAgent(&fakeCollector{}, &fakeStorage{}, &fakeSender{err: wantErr}, time.Second, time.Second)
	if err != nil {
		t.Fatalf("NewAgent() error = %v", err)
	}

	err = app.ReportOnce(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("ReportOnce() error = %v, want %v", err, wantErr)
	}
}

func TestNewAgentReturnsErrorForInvalidDependencies(t *testing.T) {
	tests := []struct {
		name      string
		collector Collector
		storage   Storage
		sender    Sender
		wantErr   error
	}{
		{
			name:    "nil collector",
			storage: &fakeStorage{},
			sender:  &fakeSender{},
			wantErr: ErrInvalidCollector,
		},
		{
			name:      "nil storage",
			collector: &fakeCollector{},
			sender:    &fakeSender{},
			wantErr:   ErrInvalidStorage,
		},
		{
			name:      "nil sender",
			collector: &fakeCollector{},
			storage:   &fakeStorage{},
			wantErr:   ErrInvalidSender,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewAgent(tt.collector, tt.storage, tt.sender, time.Second, time.Second)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("NewAgent() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

type fakeCollector struct {
	metrics []models.Metric
}

func (c *fakeCollector) Collect() []models.Metric {
	return c.metrics
}

type fakeStorage struct {
	updatedWith []models.Metric
	snapshot    []models.Metric
}

func (s *fakeStorage) Update(metrics []models.Metric) {
	s.updatedWith = metrics
}

func (s *fakeStorage) Snapshot() []models.Metric {
	return s.snapshot
}

type fakeSender struct {
	sent []models.Metric
	err  error
}

func (s *fakeSender) Send(_ context.Context, metrics []models.Metric) error {
	s.sent = metrics
	return s.err
}
