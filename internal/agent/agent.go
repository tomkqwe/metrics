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
	ErrInvalidCollector = errors.New("collector is invalid")
	ErrInvalidStorage   = errors.New("storage is invalid")
	ErrInvalidSender    = errors.New("sender is invalid")
	ErrInvalidRateLimit = errors.New("rate limit is invalid")
)

const defaultRateLimit = 1

type Agent struct {
	collector      Collector
	collectors     []Collector
	storage        Storage
	sender         Sender
	pollInterval   time.Duration
	reportInterval time.Duration
	rateLimit      int
}

type Option func(*Agent) error

func WithAdditionalCollector(collector Collector) Option {
	return func(a *Agent) error {
		if collector == nil {
			return ErrInvalidCollector
		}
		a.collectors = append(a.collectors, collector)
		return nil
	}
}

func WithRateLimit(rateLimit int) Option {
	return func(a *Agent) error {
		if rateLimit <= 0 {
			return ErrInvalidRateLimit
		}
		a.rateLimit = rateLimit
		return nil
	}
}

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
		collector:      collector,
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

func (a *Agent) PollOnce() {
	for _, collector := range a.collectors {
		a.pollOnce(collector)
	}
}

func (a *Agent) ReportOnce(ctx context.Context) error {
	return a.sender.Send(ctx, a.storage.Snapshot())
}

func (a *Agent) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	reportJobs := make(chan []models.Metric)
	var wg sync.WaitGroup

	for _, collector := range a.collectors {
		a.startCollector(ctx, &wg, collector)
	}
	a.startReportWorkers(ctx, &wg, reportJobs)

	reportTicker := time.NewTicker(a.reportInterval)
	defer reportTicker.Stop()
	defer func() {
		close(reportJobs)
		wg.Wait()
	}()

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
