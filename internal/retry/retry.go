// Package retry retries operations after configurable delays.
package retry

import (
	"context"
	"fmt"
	"time"
)

var defaultDelays = []time.Duration{
	1 * time.Second,
	3 * time.Second,
	5 * time.Second,
}

// DefaultDelays returns a new slice containing retry delays of 1, 3 and 5 seconds.
func DefaultDelays() []time.Duration {
	delays := make([]time.Duration, len(defaultDelays))
	copy(delays, defaultDelays)

	return delays
}

// Do runs an operation and retries eligible errors using DefaultDelays.
func Do(ctx context.Context, operation func() error, isRetriable func(error) bool) error {
	return DoWithDelays(ctx, DefaultDelays(), operation, isRetriable)
}

// DoWithDelays runs operation immediately, then retries eligible errors after each delay.
// Waiting for positive delays respects context cancellation.
// The operation itself must observe cancellation when necessary.
func DoWithDelays(ctx context.Context, delays []time.Duration, operation func() error, isRetriable func(error) bool) error {
	err := operation()
	if err == nil {
		return nil
	}
	if !isRetriable(err) {
		return err
	}

	for _, delay := range delays {
		if err = sleep(ctx, delay); err != nil {
			return err
		}

		err = operation()
		if err == nil {
			return nil
		}
		if !isRetriable(err) {
			return err
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", len(delays), err)
}

func sleep(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
