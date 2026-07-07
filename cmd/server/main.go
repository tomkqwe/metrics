package main

import (
	"flag"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/repository"
	"github.com/tomkqwe/metrics/internal/service"
)

const defaultServerAddress = "localhost:8080"

type config struct {
	serverAddress string
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}

	handler, err := newServerHandler()
	if err != nil {
		panic(err)
	}
	if err := http.ListenAndServe(cfg.serverAddress, handler); err != nil {
		panic(err)
	}
}

func parseConfig(args []string) (config, error) {
	var cfg config

	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.serverAddress, "a", defaultServerAddress, "HTTP server address")

	if err := flags.Parse(args); err != nil {
		return config{}, err
	}

	return cfg, nil
}

func newServerHandler() (http.Handler, error) {
	router := chi.NewRouter()
	storage := repository.NewMemStorage()
	srv, err := service.NewMetricService(storage)
	if err != nil {
		return nil, err
	}
	metricsHandler, err := handler.NewMetricsHandler(srv)
	if err != nil {
		return nil, err
	}
	router.Post("/update/{metricType}/{metricName}/{rawValue}", metricsHandler.UpdateMetric)
	router.Get("/value/{metricType}/{metricName}", metricsHandler.GetMetricValue)
	router.Get("/", metricsHandler.ListMetrics)

	return router, nil
}
