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
	if cfg.rateLimit != defaultRateLimit {
		t.Fatalf("rateLimit = %d, want %d", cfg.rateLimit, defaultRateLimit)
	}
}

func TestParseConfigUsesFlags(t *testing.T) {
	unsetAgentEnv(t)

	cfg, err := parseConfig([]string{
		"-a", "localhost:9090",
		"-p", "3",
		"-r", "15",
		"-k", "flag-key",
		"-l", "4",
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
	if cfg.rateLimit != 4 {
		t.Fatalf("rateLimit = %d, want 4", cfg.rateLimit)
	}
}

func TestParseConfigEnvOverridesFlags(t *testing.T) {
	unsetAgentEnv(t)
	t.Setenv("ADDRESS", "localhost:7070")
	t.Setenv("POLL_INTERVAL", "5")
	t.Setenv("REPORT_INTERVAL", "20")
	t.Setenv("KEY", "env-key")
	t.Setenv("RATE_LIMIT", "7")

	cfg, err := parseConfig([]string{
		"-a", "localhost:9090",
		"-p", "3",
		"-r", "15",
		"-k", "flag-key",
		"-l", "4",
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
	if cfg.rateLimit != 7 {
		t.Fatalf("rateLimit = %d, want 7", cfg.rateLimit)
	}
}

func TestParseConfigRejectsInvalidRateLimit(t *testing.T) {
	unsetAgentEnv(t)

	_, err := parseConfig([]string{"-l", "0"})
	if err == nil {
		t.Fatal("parseConfig() error = nil, want error")
	}
}

func unsetAgentEnv(t *testing.T) {
	t.Helper()

	for _, key := range []string{"ADDRESS", "POLL_INTERVAL", "REPORT_INTERVAL", "KEY", "RATE_LIMIT"} {
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
