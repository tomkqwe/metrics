package handler

import (
	"context"
	"errors"
	"net/http"
	"testing"
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

func TestPingHandlerReturnsInternalServerErrorForNilDatabase(t *testing.T) {
	handler := NewPingHandler(nil)

	response := executeRequest(http.HandlerFunc(handler.Ping), http.MethodGet, "/ping")

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
}

type fakeDatabasePinger struct {
	called bool
	err    error
}

func (p *fakeDatabasePinger) PingContext(context.Context) error {
	p.called = true

	return p.err
}
