package main

import (
	"bytes"
	"context"
	"github.com/tomkqwe/metrics/internal/repository/filestorage"
	"go.uber.org/zap"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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

func TestServerSignalHelper(t *testing.T) {
	if os.Getenv("METRICS_SERVER_SIGNAL_HELPER") != "1" {
		return
	}
	os.Args = []string{"server", "-a", os.Getenv("METRICS_TEST_ADDRESS"), "-f", os.Getenv("METRICS_TEST_STORAGE"), "-i", "3600", "-r=false"}
	main()
	os.Exit(0)
}

func TestServerSignalsSavePendingMetrics(t *testing.T) {
	unsetServerEnv(t)
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT} {
		t.Run(sig.String(), func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			address := listener.Addr().String()
			listener.Close()
			path := filepath.Join(t.TempDir(), "metrics.json")
			cmd := exec.Command(os.Args[0], "-test.run=^TestServerSignalHelper$")
			cmd.Env = append(os.Environ(), "METRICS_SERVER_SIGNAL_HELPER=1", "METRICS_TEST_ADDRESS="+address, "METRICS_TEST_STORAGE="+path)
			var output bytes.Buffer
			cmd.Stdout, cmd.Stderr = &output, &output
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = cmd.Process.Kill() })
			client := &http.Client{Timeout: time.Second}
			deadline := time.Now().Add(5 * time.Second)
			for {
				resp, err := client.Post("http://"+address+"/update/gauge/FinalValue/42.5", "text/plain", nil)
				if err == nil {
					resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						t.Fatalf("status %d", resp.StatusCode)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("server did not start")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("unexpected save before shutdown: %v", err)
			}
			if err := cmd.Process.Signal(sig); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() { done <- cmd.Wait() }()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("exit: %v\n%s", err, output.String())
				}
			case <-time.After(5 * time.Second):
				t.Fatal("shutdown timeout")
			}
			metrics, err := filestorage.NewFileStorage(path).Load()
			if err != nil || len(metrics) != 1 || metrics[0].Value == nil || *metrics[0].Value != 42.5 {
				t.Fatalf("saved metrics: %v, %v", metrics, err)
			}
		})
	}
}

func TestFinalSaveFailureIsReturned(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(parent, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := run(ctx, config{ServerAddress: "127.0.0.1:0", FileStoragePath: filepath.Join(parent, "metrics.json"), StoreInterval: time.Hour}, zap.NewNop())
	if err == nil || !strings.Contains(err.Error(), "save final metrics") {
		t.Fatalf("error = %v", err)
	}
}
