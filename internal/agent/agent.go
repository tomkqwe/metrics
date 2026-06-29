package agent

import (
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

func (a *Agent) ReportOnce() error {
	return a.sender.Send(a.storage.Snapshot())
}

func (a *Agent) Run() {
	go func() {
		for {
			a.PollOnce()
			time.Sleep(a.pollInterval)
		}
	}()

	for {
		time.Sleep(a.reportInterval)
		if err := a.ReportOnce(); err != nil {
			log.Printf("send metrics: %v", err)
		}
	}
}
