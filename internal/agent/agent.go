// Package agent coordinates periodic metric collection and delivery.
package agent

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	models "github.com/tomkqwe/metrics/internal/model"
)

var (
	// ErrInvalidCollector indicates that no collector was provided.
	ErrInvalidCollector = errors.New("collector is invalid")
	// ErrInvalidStorage indicates that no agent storage was provided.
	ErrInvalidStorage = errors.New("storage is invalid")
	// ErrInvalidSender indicates that no sender was provided.
	ErrInvalidSender = errors.New("sender is invalid")
	// ErrInvalidRateLimit indicates a non-positive worker limit.
	ErrInvalidRateLimit = errors.New("rate limit is invalid")
)

const defaultRateLimit = 1

// Agent periodically collects metrics and sends snapshots using a bounded worker pool.
type Agent struct {
	collectors     []Collector
	storage        Storage
	sender         Sender
	pollInterval   time.Duration
	reportInterval time.Duration
	rateLimit      int
}

// Option configures an agent during construction.
type Option func(*Agent) error

// WithAdditionalCollector adds another collector; nil returns ErrInvalidCollector.
func WithAdditionalCollector(collector Collector) Option {
	return func(a *Agent) error {
		if collector == nil {
			return ErrInvalidCollector
		}
		a.collectors = append(a.collectors, collector)
		return nil
	}
}

// WithRateLimit sets the maximum number of concurrent sending workers.
// A non-positive limit returns ErrInvalidRateLimit.
func WithRateLimit(rateLimit int) Option {
	return func(a *Agent) error {
		if rateLimit <= 0 {
			return ErrInvalidRateLimit
		}
		a.rateLimit = rateLimit
		return nil
	}
}

// NewAgent creates an agent with a collector, storage and sender.
// Poll and report intervals must be positive when Run is used.
func NewAgent(collector Collector, storage Storage, sender Sender, pollInterval, reportInterval time.Duration, opts ...Option) (*Agent, error) {
	if collector == nil {
		return nil, ErrInvalidCollector
	}
	if storage == nil {
		return nil, ErrInvalidStorage
	}
	if sender == nil {
		return nil, ErrInvalidSender
	}

	a := &Agent{
		collectors:     []Collector{collector},
		storage:        storage,
		sender:         sender,
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
		rateLimit:      defaultRateLimit,
	}
	for _, opt := range opts {
		if err := opt(a); err != nil {
			return nil, err
		}
	}

	return a, nil
}

// PollOnce collects and stores metrics from each collector synchronously.
func (a *Agent) PollOnce() {
	for _, collector := range a.collectors {
		a.pollOnce(collector)
	}
}

// ReportOnce sends the current storage snapshot and returns any delivery error.
func (a *Agent) ReportOnce(ctx context.Context) error {
	return a.sender.Send(ctx, a.storage.Snapshot())
}

// Run collects and reports until ctx is cancelled, then waits for its workers.
// A nil context is replaced with context.Background.
func (a *Agent) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	reportJobs, stopPipeline := a.startPipeline(ctx)
	defer stopPipeline()

	a.runReportLoop(ctx, reportJobs)
}

func (a *Agent) startPipeline(ctx context.Context) (chan<- []models.Metric, func()) {
	reportJobs := make(chan []models.Metric)
	var wg sync.WaitGroup

	for _, collector := range a.collectors {
		a.startCollector(ctx, &wg, collector)
	}
	a.startReportWorkers(ctx, &wg, reportJobs)

	stop := func() {
		close(reportJobs)
		wg.Wait()
	}

	return reportJobs, stop
}

func (a *Agent) runReportLoop(ctx context.Context, reportJobs chan<- []models.Metric) {
	reportTicker := time.NewTicker(a.reportInterval)
	defer reportTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reportTicker.C:
		}

		select {
		case <-ctx.Done():
			return
		case reportJobs <- a.storage.Snapshot():
		}
	}
}

func (a *Agent) startCollector(ctx context.Context, wg *sync.WaitGroup, collector Collector) {
	wg.Add(1)
	go func() {
		defer wg.Done()

		pollTicker := time.NewTicker(a.pollInterval)
		defer pollTicker.Stop()

		for {
			a.pollOnce(collector)
			select {
			case <-ctx.Done():
				return
			case <-pollTicker.C:
			}
		}
	}()
}

func (a *Agent) startReportWorkers(ctx context.Context, wg *sync.WaitGroup, reportJobs <-chan []models.Metric) {
	for i := 0; i < a.rateLimit; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for metrics := range reportJobs {
				if err := a.sender.Send(ctx, metrics); err != nil && !errors.Is(err, context.Canceled) {
					log.Printf("send metrics: %v", err)
				}
			}
		}()
	}
}

func (a *Agent) pollOnce(collector Collector) {
	a.storage.Update(collector.Collect())
}
