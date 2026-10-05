package audit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type observerFunc func(context.Context, Event) error

func (f observerFunc) Notify(ctx context.Context, event Event) error { return f(ctx, event) }

func TestPublisherContinuesAfterFailure(t *testing.T) {
	failure := errors.New("unavailable")
	called := false
	p := NewPublisher(observerFunc(func(_ context.Context, e Event) error { e.Metrics[0] = "changed"; return failure }), observerFunc(func(_ context.Context, e Event) error {
		called = true
		if e.Metrics[0] != "Alloc" {
			t.Errorf("event mutated: %v", e)
		}
		return nil
	}))
	if err := p.Notify(context.Background(), Event{Metrics: []string{"Alloc"}}); !errors.Is(err, failure) || !called {
		t.Fatalf("error=%v called=%v", err, called)
	}
	if err := NewPublisher().Notify(context.Background(), Event{}); err != nil {
		t.Fatal(err)
	}
}

func TestFileObserverAppendsConcurrentEvents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	if err := os.WriteFile(path, []byte("existing\n"), 0600); err != nil {
		t.Fatal(err)
	}
	observer, err := NewFileObserver(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := observer.Notify(context.Background(), Event{TS: 123, Metrics: []string{"Alloc"}, IPAddress: "::1"}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := observer.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(string(data), "\n")
	if len(lines) != 52 || lines[0] != "existing" || lines[51] != "" {
		t.Fatalf("unexpected file: %s", data)
	}
	for _, line := range lines[1:51] {
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.TS != 123 || event.IPAddress != "::1" || len(event.Metrics) != 1 || event.Metrics[0] != "Alloc" {
			t.Fatalf("event=%+v", event)
		}
	}
}

func TestHTTPObserver(t *testing.T) {
	for _, status := range []int{http.StatusNoContent, http.StatusInternalServerError, http.StatusFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.Header.Get("Content-Type") != "application/json" || r.URL.Path != "/audit" {
					t.Errorf("unexpected request: %v", r)
				}
				var event Event
				if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
					t.Error(err)
				}
				if event.TS != 123 || event.IPAddress != "127.0.0.1" || len(event.Metrics) != 1 {
					t.Errorf("event=%+v", event)
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			observer, err := NewHTTPObserver(server.URL + "/audit")
			if err != nil {
				t.Fatal(err)
			}
			err = observer.Notify(context.Background(), Event{TS: 123, Metrics: []string{"Alloc"}, IPAddress: "127.0.0.1"})
			if (err != nil) != (status >= 300) {
				t.Fatalf("status=%d error=%v", status, err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := observer.Notify(ctx, Event{}); err == nil {
				t.Fatal("expected cancellation error")
			}
		})
	}
	for _, address := range []string{"", "/audit", "ftp://localhost/audit", "http://"} {
		if _, err := NewHTTPObserver(address); err == nil {
			t.Errorf("accepted invalid URL %q", address)
		}
	}
}
