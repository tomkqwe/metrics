package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/caarlos0/env"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/middleware"
	"github.com/tomkqwe/metrics/internal/repository"
	"github.com/tomkqwe/metrics/internal/service"
)

const defaultServerAddress = "localhost:8080"

type config struct {
	serverAddress string
}

type envConfig struct {
	ServerAddress string `env:"ADDRESS"`
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		os.Exit(1)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	handler, err := newServerHandler(logger)
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

	var eCfg envConfig
	if err := env.Parse(&eCfg); err != nil {
		return config{}, fmt.Errorf("failed parse env: %w", err)
	}

	if _, ok := os.LookupEnv("ADDRESS"); ok {
		cfg.serverAddress = eCfg.ServerAddress
	}

	return cfg, nil
}

func newServerHandler(logger *zap.Logger) (http.Handler, error) {
	router := chi.NewRouter()
	router.Use(middleware.WithLogging(logger))
	router.Use(middleware.WithGzip)

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
	router.Post("/update", metricsHandler.UpdateMetricJson)
	router.Post("/update/", metricsHandler.UpdateMetricJson)
	router.Post("/value", metricsHandler.GetMetricJson)
	router.Post("/value/", metricsHandler.GetMetricJson)

	return router, nil
}
