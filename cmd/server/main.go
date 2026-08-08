package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/caarlos0/env"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/tomkqwe/metrics/internal/database"
	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/middleware"
	"github.com/tomkqwe/metrics/internal/repository"
	"github.com/tomkqwe/metrics/internal/repository/file_storage"
	"github.com/tomkqwe/metrics/internal/repository/mem_storage"
	"github.com/tomkqwe/metrics/internal/repository/postgres"
	"github.com/tomkqwe/metrics/internal/service"
)

const (
	defaultServerAddress        = "localhost:8080"
	defaultStoreIntervalSeconds = 300
	defaultRestore              = true
)

type config struct {
	ServerAddress   string        `env:"ADDRESS"`
	StoreInterval   time.Duration `env:"STORE_INTERVAL"`
	FileStoragePath string        `env:"FILE_STORAGE_PATH"`
	Restore         bool          `env:"RESTORE"`
	DatabaseDSN     string        `env:"DATABASE_DSN"`
}

func main() {
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "parse config: %v\n", err)
		os.Exit(1)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = logger.Sync()
	}()

	db, err := database.OpenPostgres(cfg.DatabaseDSN)
	if err != nil {
		logger.Fatal("open postgres", zap.Error(err))
	}
	if db != nil {
		defer func() {
			_ = db.Close()
		}()
	}

	if err := database.RunMigrations(cfg.DatabaseDSN); err != nil {
		logger.Fatal("run database migrations", zap.Error(err))
	}

	storage, fileStorage, err := newServerStorage(cfg, db)
	if err != nil {
		logger.Fatal("create server storage", zap.Error(err))
	}

	serviceOptions := make([]service.MetricServiceOption, 0, 1)
	if cfg.StoreInterval == 0 && fileStorage != nil {
		serviceOptions = append(serviceOptions, service.WithUpdatePersister(fileStorage.Save))
	}

	metricService, err := service.NewMetricService(storage, serviceOptions...)
	if err != nil {
		logger.Fatal("create metric service", zap.Error(err))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	startPeriodicSave(ctx, cfg.StoreInterval, storage, fileStorage, logger)

	handler, err := newServerHandler(logger, metricService, db)
	if err != nil {
		logger.Fatal("create server handler", zap.Error(err))
	}
	if err := http.ListenAndServe(cfg.ServerAddress, handler); err != nil {
		logger.Fatal("listen and serve", zap.Error(err))
	}
}

func parseConfig(args []string) (config, error) {
	cfg := config{
		ServerAddress: defaultServerAddress,
		StoreInterval: defaultStoreIntervalSeconds * time.Second,
		Restore:       defaultRestore,
	}

	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.ServerAddress, "a", cfg.ServerAddress, "HTTP server address")
	flags.Var(secondsDurationFlag{value: &cfg.StoreInterval}, "i", "metrics store interval in seconds")
	flags.StringVar(&cfg.FileStoragePath, "f", cfg.FileStoragePath, "metrics storage file path")
	flags.BoolVar(&cfg.Restore, "r", cfg.Restore, "restore metrics from storage file")
	flags.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "PostgreSQL database DSN")

	if err := flags.Parse(args); err != nil {
		return config{}, err
	}

	if err := env.ParseWithFuncs(&cfg, env.CustomParsers{
		reflect.TypeOf(time.Duration(0)): parseDurationSecondsEnv,
	}); err != nil {
		return config{}, fmt.Errorf("failed parse env: %w", err)
	}

	if cfg.StoreInterval < 0 {
		return config{}, fmt.Errorf("store interval must be non-negative")
	}

	return cfg, nil
}

type secondsDurationFlag struct {
	value *time.Duration
}

func (f secondsDurationFlag) String() string {
	if f.value == nil {
		return ""
	}

	return strconv.FormatInt(int64(*f.value/time.Second), 10)
}

func (f secondsDurationFlag) Set(value string) error {
	duration, err := parseDurationSeconds(value)
	if err != nil {
		return err
	}

	*f.value = duration
	return nil
}

func parseDurationSecondsEnv(value string) (interface{}, error) {
	return parseDurationSeconds(value)
}

func parseDurationSeconds(value string) (time.Duration, error) {
	seconds, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, err
	}

	return time.Duration(seconds) * time.Second, nil
}

func newServerHandler(logger *zap.Logger, srv service.Service, databasePinger handler.DatabasePinger) (http.Handler, error) {
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
	router.Post("/update", metricsHandler.UpdateMetricJSON)
	router.Post("/update/", metricsHandler.UpdateMetricJSON)
	router.Post("/updates", metricsHandler.UpdateMetricsJSON)
	router.Post("/updates/", metricsHandler.UpdateMetricsJSON)
	router.Post("/value", metricsHandler.GetMetricJSON)
	router.Post("/value/", metricsHandler.GetMetricJSON)
	router.Get("/ping", handler.NewPingHandler(databasePinger).Ping)

	return router, nil
}

func newServerStorage(cfg config, db *sql.DB) (repository.Storage, *file_storage.FileStorage, error) {
	if cfg.DatabaseDSN != "" {
		if db == nil {
			return nil, nil, fmt.Errorf("postgres database is nil")
		}

		return postgres.NewPgStorage(db), nil, nil
	}

	storage := mem_storage.NewMemStorage()
	if cfg.FileStoragePath == "" {
		return storage, nil, nil
	}

	fileStorage := file_storage.NewFileStorage(cfg.FileStoragePath)
	if cfg.Restore {
		metrics, err := fileStorage.Load()
		if err != nil {
			return nil, nil, err
		}
		if err := repository.RestoreMetrics(context.Background(), storage, metrics); err != nil {
			return nil, nil, err
		}
	}

	return storage, fileStorage, nil
}

func startPeriodicSave(
	ctx context.Context,
	interval time.Duration,
	storage repository.Storage,
	fileStorage *file_storage.FileStorage,
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
				metrics, err := storage.Snapshot(ctx)
				if err != nil {
					if logger != nil {
						logger.Info("snapshot metrics failed", zap.Error(err))
					}
					continue
				}
				if err := fileStorage.Save(metrics); err != nil && logger != nil {
					logger.Info("save metrics failed", zap.Error(err))
				}
			}
		}
	}()
}
