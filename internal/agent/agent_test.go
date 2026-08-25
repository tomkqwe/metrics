package agent

import (
	"context"
	"errors"
	"sync/atomic"
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

func TestAgentPollOnceUsesAdditionalCollectors(t *testing.T) {
	firstValue := 12.5
	secondValue := 40.5
	storage := &fakeStorage{}
	app, err := NewAgent(
		&fakeCollector{metrics: []models.Metric{
			{
				ID:    "Alloc",
				MType: models.MetricTypeGauge,
				Value: &firstValue,
			},
		}},
		storage,
		&fakeSender{},
		time.Second,
		time.Second,
		WithAdditionalCollector(&fakeCollector{metrics: []models.Metric{
			{
				ID:    "FreeMemory",
				MType: models.MetricTypeGauge,
				Value: &secondValue,
			},
		}}),
	)
	if err != nil {
		t.Fatalf("NewAgent() error = %v", err)
	}

	app.PollOnce()

	if len(storage.updatedWith) != 2 {
		t.Fatalf("Update() metrics len = %d, want 2", len(storage.updatedWith))
	}
	if storage.updatedWith[0].ID != "Alloc" {
		t.Fatalf("first Update() metric ID = %q, want Alloc", storage.updatedWith[0].ID)
	}
	if storage.updatedWith[1].ID != "FreeMemory" {
		t.Fatalf("second Update() metric ID = %q, want FreeMemory", storage.updatedWith[1].ID)
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

func TestAgentRunLimitsConcurrentReports(t *testing.T) {
	value := 12.5
	sender := &blockingSender{
		started: make(chan struct{}, 10),
		release: make(chan struct{}),
	}
	app, err := NewAgent(
		&fakeCollector{},
		&fakeStorage{snapshot: []models.Metric{
			{
				ID:    "Alloc",
				MType: models.MetricTypeGauge,
				Value: &value,
			},
		}},
		sender,
		time.Hour,
		time.Millisecond,
		WithRateLimit(2),
	)
	if err != nil {
		t.Fatalf("NewAgent() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		app.Run(ctx)
		close(done)
	}()

	waitForStartedReports(t, sender.started, 2)
	time.Sleep(10 * time.Millisecond)

	if got := sender.maxActive.Load(); got > 2 {
		t.Fatalf("max active reports = %d, want <= 2", got)
	}

	cancel()
	close(sender.release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run() did not stop after context cancellation")
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

func TestNewAgentReturnsErrorForInvalidRateLimit(t *testing.T) {
	_, err := NewAgent(
		&fakeCollector{},
		&fakeStorage{},
		&fakeSender{},
		time.Second,
		time.Second,
		WithRateLimit(0),
	)
	if !errors.Is(err, ErrInvalidRateLimit) {
		t.Fatalf("NewAgent() error = %v, want %v", err, ErrInvalidRateLimit)
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
	s.updatedWith = append(s.updatedWith, metrics...)
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

type blockingSender struct {
	active    atomic.Int32
	maxActive atomic.Int32
	started   chan struct{}
	release   chan struct{}
}

func (s *blockingSender) Send(ctx context.Context, _ []models.Metric) error {
	active := s.active.Add(1)
	defer s.active.Add(-1)
	s.storeMaxActive(active)

	select {
	case s.started <- struct{}{}:
	default:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.release:
		return nil
	}
}

func (s *blockingSender) storeMaxActive(active int32) {
	for {
		maxActive := s.maxActive.Load()
		if active <= maxActive {
			return
		}
		if s.maxActive.CompareAndSwap(maxActive, active) {
			return
		}
	}
}

func waitForStartedReports(t *testing.T, started <-chan struct{}, count int) {
	t.Helper()

	for i := 0; i < count; i++ {
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatalf("started reports = %d, want %d", i, count)
		}
	}
}
