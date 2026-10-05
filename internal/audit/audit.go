// Package audit delivers metric update events to configured observers.
package audit

import (
	"context"
	"errors"
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

// Publisher broadcasts events to observers registered at construction time.
// Its observer list is immutable, so requests can publish concurrently.
type Publisher struct {
	observers []Observer
}

// NewPublisher registers observers for subsequent event delivery.
func NewPublisher(observers ...Observer) *Publisher {
	return &Publisher{observers: append([]Observer(nil), observers...)}
}

// Notify attempts every observer, even when another observer fails.
func (p *Publisher) Notify(ctx context.Context, event Event) error {
	var errs []error
	for _, observer := range p.observers {
		copyEvent := event
		copyEvent.Metrics = append([]string{}, event.Metrics...)
		if err := observer.Notify(ctx, copyEvent); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
