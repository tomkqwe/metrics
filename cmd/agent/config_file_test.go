package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeAgentConfig(t *testing.T, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAgentJSONConfig(t *testing.T) {
	unsetAgentEnv(t)
	path := writeAgentConfig(t, `{"grpc_address":"file:9090","address":"file:8081","poll_interval":"500ms","report_interval":"2m","key":"secret","crypto_key":"public.pem","rate_limit":4}`)
	want := config{grpcAddress: "file:9090", serverAddress: "file:8081", pollInterval: 500 * time.Millisecond, reportInterval: 2 * time.Minute, key: "secret", cryptoKey: "public.pem", rateLimit: 4}
	for _, flag := range []string{"-c", "-config"} {
		cfg, err := parseConfig([]string{flag, path})
		if err != nil || cfg != want {
			t.Fatalf("%s: got %+v, %v; want %+v", flag, cfg, err, want)
		}
	}
	t.Setenv("CONFIG", path)
	cfg, err := parseConfig([]string{"-c", "missing.json"})
	if err != nil || cfg != want {
		t.Fatalf("CONFIG: %+v, %v", cfg, err)
	}
}

func TestAgentJSONPrecedence(t *testing.T) {
	unsetAgentEnv(t)
	path := writeAgentConfig(t, `{"grpc_address":"file:9090","address":"file","poll_interval":"500ms","report_interval":"2m","key":"secret","crypto_key":"public.pem","rate_limit":4}`)
	args := []string{"-config", path, "-grpc-address", "flag:9090", "-a", "flag", "-p", "2", "-r", "10", "-k=", "-crypto-key=", "-l", "1"}
	want := config{serverAddress: "flag", grpcAddress: "flag:9090", pollInterval: 2 * time.Second, reportInterval: 10 * time.Second, rateLimit: 1}
	cfg, err := parseConfig(args)
	if err != nil || cfg != want {
		t.Fatalf("flags: %+v, %v; want %+v", cfg, err, want)
	}
	for name, value := range map[string]string{"GRPC_ADDRESS": "env:9090", "ADDRESS": "env", "POLL_INTERVAL": "3", "REPORT_INTERVAL": "4", "KEY": "env-key", "CRYPTO_KEY": "env.pem", "RATE_LIMIT": "5"} {
		t.Setenv(name, value)
	}
	want = config{serverAddress: "env", grpcAddress: "env:9090", pollInterval: 3 * time.Second, reportInterval: 4 * time.Second, key: "env-key", cryptoKey: "env.pem", rateLimit: 5}
	cfg, err = parseConfig(args)
	if err != nil || cfg != want {
		t.Fatalf("env: %+v, %v; want %+v", cfg, err, want)
	}
}

func TestAgentJSONDefaultsAndErrors(t *testing.T) {
	unsetAgentEnv(t)
	defaults, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := parseConfig([]string{"-c", writeAgentConfig(t, `{}`)})
	if err != nil || cfg != defaults {
		t.Fatalf("defaults: %+v, %v", cfg, err)
	}
	for _, data := range []string{``, `null`, `[]`, `{`, `{} {}`, `{"poll_interval":1}`, `{"report_interval":"bad"}`, `{"rate_limit":0}`, `{"address":123}`} {
		if _, err := parseConfig([]string{"-c", writeAgentConfig(t, data)}); err == nil {
			t.Errorf("accepted %q", data)
		}
	}
	if _, err := parseConfig([]string{"-c", filepath.Join(t.TempDir(), "missing")}); err == nil {
		t.Fatal("accepted missing file")
	}
}
