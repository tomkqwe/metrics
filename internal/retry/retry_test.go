package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDoWithDelaysRetriesRetriableErrors(t *testing.T) {
	wantErr := errors.New("temporary")
	attempts := 0

	err := DoWithDelays(context.Background(), []time.Duration{0, 0, 0}, func() error {
		attempts++
		if attempts < 4 {
			return wantErr
		}

		return nil
	}, func(err error) bool {
		return errors.Is(err, wantErr)
	})
	if err != nil {
		t.Fatalf("DoWithDelays() error = %v", err)
	}
	if attempts != 4 {
		t.Fatalf("attempts = %d, want 4", attempts)
	}
}

func TestDoWithDelaysStopsAfterRetryLimit(t *testing.T) {
	wantErr := errors.New("temporary")
	attempts := 0

	err := DoWithDelays(context.Background(), []time.Duration{0, 0, 0}, func() error {
		attempts++
		return wantErr
	}, func(err error) bool {
		return errors.Is(err, wantErr)
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("DoWithDelays() error = %v, want %v", err, wantErr)
	}
	if attempts != 4 {
		t.Fatalf("attempts = %d, want 4", attempts)
	}
}

func TestDoWithDelaysDoesNotRetryNonRetriableErrors(t *testing.T) {
	wantErr := errors.New("permanent")
	attempts := 0

	err := DoWithDelays(context.Background(), []time.Duration{0, 0, 0}, func() error {
		attempts++
		return wantErr
	}, func(err error) bool {
		return false
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("DoWithDelays() error = %v, want %v", err, wantErr)
	}
	if attempts != 1 {
		t.Fatalf("attempts = %d, want 1", attempts)
	}
}
