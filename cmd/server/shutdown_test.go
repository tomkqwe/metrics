package main

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestShutdownWaitsForActiveHandler(t *testing.T) {
	address := make(chan string, 1)
	entered := make(chan struct{})
	release := make(chan struct{})
	server := &http.Server{
		Addr:        "127.0.0.1:0",
		BaseContext: func(l net.Listener) context.Context { address <- l.Addr().String(); return context.Background() },
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			close(entered)
			<-release
			if r.Context().Err() != nil {
				t.Error("active request cancelled")
			}
			w.WriteHeader(http.StatusOK)
		}),
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopped := make(chan error, 1)
	go func() { stopped <- serveUntilCancelled(ctx, server) }()
	client := &http.Client{Timeout: 3 * time.Second}
	response := make(chan error, 1)
	var addr string
	select {
	case addr = <-address:
	case err := <-stopped:
		t.Fatalf("server failed: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("startup timeout")
	}
	go func() {
		resp, err := client.Get("http://" + addr)
		if err == nil {
			resp.Body.Close()
		}
		response <- err
	}()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not start")
	}
	cancel()
	select {
	case err := <-stopped:
		t.Errorf("stopped before request completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-response; err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown stuck")
	}
}

func TestServeReturnsListenError(t *testing.T) {
	server := &http.Server{Addr: "invalid address"}
	if err := serveUntilCancelled(context.Background(), server); err == nil {
		t.Fatal("expected listen error")
	}
}
