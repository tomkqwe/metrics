package main

import (
	"log"
	"time"

	"github.com/tomkqwe/metrics/internal/agent"
	"github.com/tomkqwe/metrics/internal/agent/collector"
	"github.com/tomkqwe/metrics/internal/agent/sender"
	"github.com/tomkqwe/metrics/internal/agent/storage"
)

const (
	serverURL      = "http://localhost:8080"
	pollInterval   = 2 * time.Second
	reportInterval = 10 * time.Second
)

func main() {
	app, err := newAgent()
	if err != nil {
		log.Fatal(err)
	}

	app.Run()
}

func newAgent() (*agent.Agent, error) {
	return agent.NewAgent(
		collector.NewRuntimeCollector(),
		storage.NewMemoryStorage(),
		sender.NewHTTPSender(serverURL),
		pollInterval,
		reportInterval,
	)
}
