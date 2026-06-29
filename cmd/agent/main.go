package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

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

	app.Run()
}

func parseConfig(args []string) (config, error) {
	var cfg config
	var pollIntervalSeconds int
	var reportIntervalSeconds int

	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.serverAddress, "a", defaultServerAddress, "HTTP server address")
	flags.IntVar(&reportIntervalSeconds, "r", defaultReportIntervalSeconds, "metrics report interval in seconds")
	flags.IntVar(&pollIntervalSeconds, "p", defaultPollIntervalSeconds, "metrics poll interval in seconds")

	if err := flags.Parse(args); err != nil {
		return config{}, err
	}

	cfg.pollInterval = time.Duration(pollIntervalSeconds) * time.Second
	cfg.reportInterval = time.Duration(reportIntervalSeconds) * time.Second

	return cfg, nil
}

func newAgent(cfg config) (*agent.Agent, error) {
	return agent.NewAgent(
		collector.NewRuntimeCollector(),
		storage.NewMemoryStorage(),
		sender.NewHTTPSender(serverURL(cfg.serverAddress)),
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
