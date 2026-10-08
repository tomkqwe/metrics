// Package audit delivers metric update events to configured observers.
package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

// Event describes the metrics accepted in one request.
type Event struct {
	// TS is the event time in Unix seconds.
	TS int64 `json:"ts"`
	// Metrics contains the names accepted in the request.
	Metrics []string `json:"metrics"`
	// IPAddress is the remote connection address without its port.
	IPAddress string `json:"ip_address"`
}

// Observer receives an event after metrics have been successfully updated.
// Implementations must support concurrent calls.
type Observer interface {
	// Notify delivers an event and reports a delivery failure, if any.
	Notify(context.Context, Event) error
}

// ErrClosed indicates that the publisher no longer accepts events.
var ErrClosed = errors.New("audit publisher is closed")

const workersPerObserver = 2
const queueCapacity = 64

type delivery struct {
	ctx   context.Context
	event Event
}

// Publisher delivers events asynchronously using a bounded queue and two workers
// per observer. Close must be called before closing the observers themselves.
type Publisher struct {
	mu      sync.RWMutex
	closed  bool
	queues  []chan delivery
	workers sync.WaitGroup
}

// NewPublisher starts independent workers for each observer. Delivery failures
// are logged using logger; a nil logger disables logging.
func NewPublisher(logger *zap.Logger, observers ...Observer) *Publisher {
	if logger == nil {
		logger = zap.NewNop()
	}
	p := &Publisher{}
	for _, observer := range observers {
		queue := make(chan delivery, queueCapacity)
		p.queues = append(p.queues, queue)
		for i := 0; i < workersPerObserver; i++ {
			p.workers.Add(1)
			go func() {
				defer p.workers.Done()
				for job := range queue {
					if err := observer.Notify(job.ctx, job.event); err != nil {
						logger.Error("deliver audit event", zap.String("observer", fmt.Sprintf("%T", observer)), zap.Error(err))
					}
				}
			}()
		}
	}
	return p
}

// Notify queues an independent event copy for each observer. Delivery survives
// request cancellation. A full queue applies backpressure until space becomes
// available or ctx is cancelled; cancellation can leave an event partly queued.
func (p *Publisher) Notify(ctx context.Context, event Event) error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.closed {
		return ErrClosed
	}
	for _, queue := range p.queues {
		copyEvent := event
		copyEvent.Metrics = append([]string{}, event.Metrics...)
		select {
		case queue <- delivery{ctx: context.WithoutCancel(ctx), event: copyEvent}:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

// Close stops admission and waits for all queued and active deliveries.
// It is safe to call concurrently or repeatedly.
func (p *Publisher) Close() {
	p.mu.Lock()
	if !p.closed {
		p.closed = true
		for _, queue := range p.queues {
			close(queue)
		}
	}
	p.mu.Unlock()
	p.workers.Wait()
}
