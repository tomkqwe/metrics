package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/caarlos0/env"
	"github.com/tomkqwe/metrics/internal/agent"
	"github.com/tomkqwe/metrics/internal/agent/collector"
	"github.com/tomkqwe/metrics/internal/agent/sender"
	"github.com/tomkqwe/metrics/internal/agent/storage"
)

const (
	defaultServerAddress         = "localhost:8080"
	defaultPollIntervalSeconds   = 2
	defaultReportIntervalSeconds = 10
)

type config struct {
	serverAddress  string
	pollInterval   time.Duration
	reportInterval time.Duration
	key            string
}

type envConfig struct {
	ServerAddress  string `env:"ADDRESS"`
	PollInterval   int    `env:"POLL_INTERVAL"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	Key            string `env:"KEY"`
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}

	app, err := newAgent(cfg)
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	app.Run(ctx)
}

func parseConfig(args []string) (config, error) {
	var cfg config

	var pollInterval int
	var reportInterval int

	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.serverAddress, "a", defaultServerAddress, "HTTP server address")
	flags.IntVar(&reportInterval, "r", defaultReportIntervalSeconds, "metrics report interval in seconds")
	flags.IntVar(&pollInterval, "p", defaultPollIntervalSeconds, "metrics poll interval in seconds")
	flags.StringVar(&cfg.key, "k", cfg.key, "SHA256 hash key")

	if err := flags.Parse(args); err != nil {
		return config{}, err
	}

	cfg.pollInterval = time.Duration(pollInterval) * time.Second
	cfg.reportInterval = time.Duration(reportInterval) * time.Second

	var eCfg envConfig
	if err := env.Parse(&eCfg); err != nil {
		return config{}, fmt.Errorf("parse env config failed: %w", err)
	}

	if _, ok := os.LookupEnv("ADDRESS"); ok {
		cfg.serverAddress = eCfg.ServerAddress
	}

	if _, ok := os.LookupEnv("POLL_INTERVAL"); ok {
		cfg.pollInterval = time.Duration(eCfg.PollInterval) * time.Second
	}

	if _, ok := os.LookupEnv("REPORT_INTERVAL"); ok {
		cfg.reportInterval = time.Duration(eCfg.ReportInterval) * time.Second
	}
	if _, ok := os.LookupEnv("KEY"); ok {
		cfg.key = eCfg.Key
	}

	return cfg, nil
}

func newAgent(cfg config) (*agent.Agent, error) {
	return agent.NewAgent(
		collector.NewRuntimeCollector(),
		storage.NewMemoryStorage(),
		sender.NewHTTPSender(serverURL(cfg.serverAddress), sender.WithKey(cfg.key)),
		cfg.pollInterval,
		cfg.reportInterval,
	)
}

func serverURL(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return address
	}

	return "http://" + address
}
