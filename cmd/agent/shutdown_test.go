package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func TestAgentSignalHelper(t *testing.T) {
	if os.Getenv("METRICS_AGENT_SIGNAL_HELPER") != "1" {
		return
	}
	os.Args = []string{"agent", "-a", os.Getenv("METRICS_TEST_ADDRESS"), "-r", "1", "-p", "3600"}
	main()
	os.Exit(0)
}

func TestAgentSignalsFinishDelivery(t *testing.T) {
	unsetAgentEnv(t)
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT} {
		t.Run(sig.String(), func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			var requests atomic.Int32
			var cancelled atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				if requests.Add(1) == 1 {
					close(entered)
					select {
					case <-release:
					case <-r.Context().Done():
						cancelled.Store(true)
					}
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			defer unblock()
			cmd := exec.Command(os.Args[0], "-test.run=^TestAgentSignalHelper$")
			cmd.Env = append(os.Environ(), "METRICS_AGENT_SIGNAL_HELPER=1", "METRICS_TEST_ADDRESS="+server.URL)
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case <-entered:
			case err := <-done:
				t.Fatalf("early exit: %v\n%s", err, output.String())
			case <-time.After(5 * time.Second):
				t.Fatal("agent did not send")
			}
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				t.Fatalf("exit before delivery finished: %v\n%s", err, output.String())
			case <-time.After(50 * time.Millisecond):
			}
			unblock()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("exit: %v\n%s", err, output.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("shutdown timeout")
			}
			if cancelled.Load() {
				t.Error("active request was cancelled")
			}
			if got := requests.Load(); got != 2 {
				t.Errorf("requests = %d, want active and final reports", got)
			}
		})
	}
}
