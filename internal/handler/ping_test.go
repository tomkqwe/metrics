package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/lib/pq"
	"github.com/lib/pq/pqerror"
)

func TestPingHandlerSuccess(t *testing.T) {
	pinger := &fakeDatabasePinger{}
	handler := NewPingHandler(pinger)

	response := executeRequest(http.HandlerFunc(handler.Ping), http.MethodGet, "/ping")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !pinger.called {
		t.Fatal("PingContext() was not called")
	}
}

func TestPingHandlerReturnsInternalServerErrorForFailedPing(t *testing.T) {
	pinger := &fakeDatabasePinger{err: errors.New("ping failed")}
	handler := NewPingHandler(pinger)

	response := executeRequest(http.HandlerFunc(handler.Ping), http.MethodGet, "/ping")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if !pinger.called {
		t.Fatal("PingContext() was not called")
	}
}

func TestPingHandlerRetriesConnectionException(t *testing.T) {
	pinger := &fakeDatabasePinger{
		errs: []error{
			&pq.Error{Code: pqerror.Code(pgerrcode.ConnectionFailure)},
			&pq.Error{Code: pqerror.Code(pgerrcode.ConnectionFailure)},
			&pq.Error{Code: pqerror.Code(pgerrcode.ConnectionFailure)},
			nil,
		},
	}
	handler := NewPingHandler(pinger)
	handler.retryDelays = []time.Duration{0, 0, 0}

	response := executeRequest(http.HandlerFunc(handler.Ping), http.MethodGet, "/ping")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if pinger.calls != 4 {
		t.Fatalf("PingContext() calls = %d, want 4", pinger.calls)
	}
}

func TestPingHandlerReturnsInternalServerErrorForNilDatabase(t *testing.T) {
	handler := NewPingHandler(nil)

	response := executeRequest(http.HandlerFunc(handler.Ping), http.MethodGet, "/ping")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

type fakeDatabasePinger struct {
	called bool
	calls  int
	err    error
	errs   []error
}

func (p *fakeDatabasePinger) PingContext(context.Context) error {
	p.called = true
	p.calls++
	if len(p.errs) > 0 {
		err := p.errs[0]
		p.errs = p.errs[1:]
		return err
	}

	return p.err
}
