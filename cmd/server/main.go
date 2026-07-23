package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/caarlos0/env"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/middleware"
	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/repository"
	"github.com/tomkqwe/metrics/internal/service"
)

const (
	defaultServerAddress        = "localhost:8080"
	defaultStoreIntervalSeconds = 300
	defaultFileStoragePath      = "/tmp/metrics-db.json"
	defaultRestore              = true
)

type config struct {
	serverAddress   string
	storeInterval   time.Duration
	fileStoragePath string
	restore         bool
}

type envConfig struct {
	ServerAddress   string `env:"ADDRESS"`
	StoreInterval   int    `env:"STORE_INTERVAL"`
	FileStoragePath string `env:"FILE_STORAGE_PATH"`
	Restore         bool   `env:"RESTORE"`
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

	storage, fileStorage, err := newServerStorage(cfg)
	if err != nil {
		panic(err)
	}

	serviceOptions := make([]service.MetricServiceOption, 0, 1)
	if cfg.storeInterval == 0 && fileStorage != nil {
		serviceOptions = append(serviceOptions, service.WithUpdatePersister(fileStorage.Save))
	}

	metricService, err := service.NewMetricService(storage, serviceOptions...)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startPeriodicSave(ctx, cfg.storeInterval, storage, fileStorage, logger)

	handler, err := newServerHandler(logger, metricService)
	if err != nil {
		panic(err)
	}
	if err := http.ListenAndServe(cfg.serverAddress, handler); err != nil {
		panic(err)
	}
}

func parseConfig(args []string) (config, error) {
	var cfg config

	var storeInterval int

	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.serverAddress, "a", defaultServerAddress, "HTTP server address")
	flags.IntVar(&storeInterval, "i", defaultStoreIntervalSeconds, "metrics store interval in seconds")
	flags.StringVar(&cfg.fileStoragePath, "f", defaultFileStoragePath, "metrics storage file path")
	flags.BoolVar(&cfg.restore, "r", defaultRestore, "restore metrics from storage file")

	if err := flags.Parse(args); err != nil {
		return config{}, err
	}
	cfg.storeInterval = time.Duration(storeInterval) * time.Second

	var eCfg envConfig
	if err := env.Parse(&eCfg); err != nil {
		return config{}, fmt.Errorf("failed parse env: %w", err)
	}

	if _, ok := os.LookupEnv("ADDRESS"); ok {
		cfg.serverAddress = eCfg.ServerAddress
	}

	if _, ok := os.LookupEnv("STORE_INTERVAL"); ok {
		cfg.storeInterval = time.Duration(eCfg.StoreInterval) * time.Second
	}

	if _, ok := os.LookupEnv("FILE_STORAGE_PATH"); ok {
		cfg.fileStoragePath = eCfg.FileStoragePath
	}

	if _, ok := os.LookupEnv("RESTORE"); ok {
		cfg.restore = eCfg.Restore
	}

	if cfg.storeInterval < 0 {
		return config{}, fmt.Errorf("store interval must be non-negative")
	}

	return cfg, nil
}

func newServerHandler(logger *zap.Logger, srv service.Service) (http.Handler, error) {
	router := chi.NewRouter()
	router.Use(middleware.WithLogging(logger))
	router.Use(middleware.WithGzip)

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

func newServerStorage(cfg config) (repository.Storage, *repository.FileStorage, error) {
	storage := repository.NewMemStorage()
	if cfg.fileStoragePath == "" {
		return storage, nil, nil
	}

	fileStorage := repository.NewFileStorage(cfg.fileStoragePath)
	if cfg.restore {
		if err := restoreMetrics(storage, fileStorage); err != nil {
			return nil, nil, err
		}
	}

	return storage, fileStorage, nil
}

func restoreMetrics(storage repository.Storage, fileStorage *repository.FileStorage) error {
	metrics, err := fileStorage.Load()
	if err != nil {
		return err
	}

	for _, metric := range metrics {
		switch metric.MType {
		case models.MetricTypeGauge:
			if metric.Value == nil {
				return fmt.Errorf("restore gauge %q: missing value", metric.ID)
			}
			storage.UpdateGauge(metric.ID, models.Gauge(*metric.Value))
		case models.MetricTypeCounter:
			if metric.Delta == nil {
				return fmt.Errorf("restore counter %q: missing delta", metric.ID)
			}
			storage.UpdateCounter(metric.ID, models.Counter(*metric.Delta))
		default:
			return fmt.Errorf("restore metric %q: unknown type %q", metric.ID, metric.MType)
		}
	}

	return nil
}

func startPeriodicSave(
	ctx context.Context,
	interval time.Duration,
	storage repository.Storage,
	fileStorage *repository.FileStorage,
	logger *zap.Logger,
) {
	if interval <= 0 || fileStorage == nil {
		return
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := fileStorage.Save(storage.Snapshot()); err != nil && logger != nil {
					logger.Info("save metrics failed", zap.Error(err))
				}
			}
		}
	}()
}
