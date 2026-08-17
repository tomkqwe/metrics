package main

import (
	"os"
	"testing"
	"time"
)

func TestParseConfigUsesDefaults(t *testing.T) {
	unsetAgentEnv(t)

	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.serverAddress != defaultServerAddress {
		t.Fatalf("serverAddress = %q, want %q", cfg.serverAddress, defaultServerAddress)
	}
	if cfg.pollInterval != defaultPollIntervalSeconds*time.Second {
		t.Fatalf("pollInterval = %v, want %v", cfg.pollInterval, defaultPollIntervalSeconds*time.Second)
	}
	if cfg.reportInterval != defaultReportIntervalSeconds*time.Second {
		t.Fatalf("reportInterval = %v, want %v", cfg.reportInterval, defaultReportIntervalSeconds*time.Second)
	}
	if cfg.key != "" {
		t.Fatalf("key = %q, want empty", cfg.key)
	}
}

func TestParseConfigUsesFlags(t *testing.T) {
	unsetAgentEnv(t)

	cfg, err := parseConfig([]string{
		"-a", "localhost:9090",
		"-p", "3",
		"-r", "15",
		"-k", "flag-key",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.serverAddress != "localhost:9090" {
		t.Fatalf("serverAddress = %q, want localhost:9090", cfg.serverAddress)
	}
	if cfg.pollInterval != 3*time.Second {
		t.Fatalf("pollInterval = %v, want 3s", cfg.pollInterval)
	}
	if cfg.reportInterval != 15*time.Second {
		t.Fatalf("reportInterval = %v, want 15s", cfg.reportInterval)
	}
	if cfg.key != "flag-key" {
		t.Fatalf("key = %q, want flag-key", cfg.key)
	}
}

func TestParseConfigEnvOverridesFlags(t *testing.T) {
	unsetAgentEnv(t)
	t.Setenv("ADDRESS", "localhost:7070")
	t.Setenv("POLL_INTERVAL", "5")
	t.Setenv("REPORT_INTERVAL", "20")
	t.Setenv("KEY", "env-key")

	cfg, err := parseConfig([]string{
		"-a", "localhost:9090",
		"-p", "3",
		"-r", "15",
		"-k", "flag-key",
	})
	if err != nil {
		t.Fatalf("parseConfig() error = %v", err)
	}

	if cfg.serverAddress != "localhost:7070" {
		t.Fatalf("serverAddress = %q, want localhost:7070", cfg.serverAddress)
	}
	if cfg.pollInterval != 5*time.Second {
		t.Fatalf("pollInterval = %v, want 5s", cfg.pollInterval)
	}
	if cfg.reportInterval != 20*time.Second {
		t.Fatalf("reportInterval = %v, want 20s", cfg.reportInterval)
	}
	if cfg.key != "env-key" {
		t.Fatalf("key = %q, want env-key", cfg.key)
	}
}

func unsetAgentEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{"ADDRESS", "POLL_INTERVAL", "REPORT_INTERVAL", "KEY"} {
		oldValue, ok := os.LookupEnv(key)
		if err := os.Unsetenv(key); err != nil {
			t.Fatalf("unset env %s: %v", key, err)
		}
		t.Cleanup(func() {
			if ok {
				_ = os.Setenv(key, oldValue)
				return
			}
			_ = os.Unsetenv(key)
		})
	}
}
