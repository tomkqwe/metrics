package audit

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestPublisherIndependentWorkersAndDrain(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, 1)
	fast := make(chan Event, 1)
	core, logs := observer.New(zap.ErrorLevel)
	p := NewPublisher(zap.New(core), observerFunc(func(ctx context.Context, e Event) error {
		started <- struct{}{}
		<-release
		if ctx.Err() != nil {
			t.Error("delivery cancelled with request")
		}
		return errors.New("receiver unavailable")
	}), observerFunc(func(_ context.Context, e Event) error { fast <- e; return nil }))
	ctx, cancel := context.WithCancel(context.Background())
	event := Event{Metrics: []string{"Alloc"}}
	if err := p.Notify(ctx, event); err != nil {
		t.Fatal(err)
	}
	cancel()
	event.Metrics[0] = "changed"
	<-started
	select {
	case e := <-fast:
		if e.Metrics[0] != "Alloc" {
			t.Error("event not copied")
		}
	case <-time.After(time.Second):
		t.Error("slow observer blocked fast observer")
	}
	done := make(chan struct{})
	go func() { p.Close(); close(done) }()
	select {
	case <-done:
		t.Error("close did not drain")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close stuck")
	}
	p.Close()
	if err := p.Notify(context.Background(), Event{}); !errors.Is(err, ErrClosed) {
		t.Fatalf("error=%v", err)
	}
	if logs.FilterMessage("deliver audit event").Len() != 1 {
		t.Fatal("delivery error not logged")
	}
}

func TestPublisherBoundsWorkersAndQueue(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{}, workersPerObserver)
	var active, peak, delivered atomic.Int32
	p := NewPublisher(nil, observerFunc(func(context.Context, Event) error {
		n := active.Add(1)
		for old := peak.Load(); n > old; old = peak.Load() {
			if peak.CompareAndSwap(old, n) {
				break
			}
		}
		select {
		case started <- struct{}{}:
		default:
		}
		<-release
		active.Add(-1)
		delivered.Add(1)
		return nil
	}))
	for i := 0; i < workersPerObserver; i++ {
		if err := p.Notify(context.Background(), Event{}); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < workersPerObserver; i++ {
		<-started
	}
	for i := 0; i < queueCapacity; i++ {
		if err := p.Notify(context.Background(), Event{}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Notify(ctx, Event{}); !errors.Is(err, context.Canceled) {
		t.Errorf("queue overflow error=%v", err)
	}
	close(release)
	p.Close()
	if peak.Load() > workersPerObserver || delivered.Load() != workersPerObserver+queueCapacity {
		t.Fatalf("peak=%d delivered=%d", peak.Load(), delivered.Load())
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestHTTPObserverRetriesTransportOnly(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		failures, status, want int
	}{
		{"recover", 2, 200, 3}, {"exhausted", 10, 200, 4}, {"application failure", 0, 503, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := NewHTTPObserver("http://audit.example/events")
			if err != nil {
				t.Fatal(err)
			}
			h.retryDelays = []time.Duration{0, 0, 0}
			calls := 0
			var first string
			h.client.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
				calls++
				data, err := io.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				r.Body.Close()
				if calls == 1 {
					first = string(data)
				} else if string(data) != first {
					t.Error("retry body differs")
				}
				if calls <= tc.failures {
					return nil, errors.New("network failure")
				}
				return &http.Response{StatusCode: tc.status, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			})
			err = h.Notify(context.Background(), Event{Metrics: []string{"Alloc"}})
			if calls != tc.want || (err != nil) != (tc.status != 200 || tc.failures > 3) {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
		})
	}
}

func TestHTTPObserverPreservesURLParseError(t *testing.T) {
	_, err := NewHTTPObserver("http://%zz")
	var parseErr *url.Error
	if !errors.As(err, &parseErr) {
		t.Fatalf("lost parse error: %v", err)
	}
}
