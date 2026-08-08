package agent

import (
	"context"
	"errors"
	"log"
	"time"
)

var (
	ErrInvalidCollector = errors.New("collector is invalid")
	ErrInvalidStorage   = errors.New("storage is invalid")
	ErrInvalidSender    = errors.New("sender is invalid")
)

type Agent struct {
	collector      Collector
	storage        Storage
	sender         Sender
	pollInterval   time.Duration
	reportInterval time.Duration
}

func NewAgent(collector Collector, storage Storage, sender Sender, pollInterval, reportInterval time.Duration) (*Agent, error) {
	if collector == nil {
		return nil, ErrInvalidCollector
	}
	if storage == nil {
		return nil, ErrInvalidStorage
	}
	if sender == nil {
		return nil, ErrInvalidSender
	}

	return &Agent{
		collector:      collector,
		storage:        storage,
		sender:         sender,
		pollInterval:   pollInterval,
		reportInterval: reportInterval,
	}, nil
}

func (a *Agent) PollOnce() {
	a.storage.Update(a.collector.Collect())
}

func (a *Agent) ReportOnce(ctx context.Context) error {
	return a.sender.Send(ctx, a.storage.Snapshot())
}

func (a *Agent) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	go func() {
		pollTicker := time.NewTicker(a.pollInterval)
		defer pollTicker.Stop()

		for {
			a.PollOnce()
			select {
			case <-ctx.Done():
				return
			case <-pollTicker.C:
			}
		}
	}()

	reportTicker := time.NewTicker(a.reportInterval)
	defer reportTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reportTicker.C:
		}

		if err := a.ReportOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("send metrics: %v", err)
		}
	}
}
