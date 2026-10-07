package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"syscall"
	"time"

	"github.com/caarlos0/env"
	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/tomkqwe/metrics/internal/audit"
	"github.com/tomkqwe/metrics/internal/database"
	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/middleware"
	"github.com/tomkqwe/metrics/internal/repository"
	"github.com/tomkqwe/metrics/internal/repository/filestorage"
	"github.com/tomkqwe/metrics/internal/repository/memstorage"
	"github.com/tomkqwe/metrics/internal/repository/postgres"
	"github.com/tomkqwe/metrics/internal/service"
)

const (
	defaultServerAddress        = "localhost:8080"
	defaultStoreIntervalSeconds = 300
	defaultRestore              = true
)

type config struct {
	AuditFile       string        `env:"AUDIT_FILE"`
	AuditURL        string        `env:"AUDIT_URL"`
	ServerAddress   string        `env:"ADDRESS"`
	StoreInterval   time.Duration `env:"STORE_INTERVAL"`
	FileStoragePath string        `env:"FILE_STORAGE_PATH"`
	Restore         bool          `env:"RESTORE"`
	DatabaseDSN     string        `env:"DATABASE_DSN"`
	Key             string        `env:"KEY"`
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err = run(ctx, cfg, logger)
	stop()
	if err != nil {
		logger.Error("server stopped", zap.Error(err))
		_ = logger.Sync()
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config, logger *zap.Logger) error {
	db, err := database.OpenPostgres(cfg.DatabaseDSN)
	if err != nil {
		return fmt.Errorf("open postgres: %w", err)
	}
	if db != nil {
		defer func() {
			_ = db.Close()
		}()
	}

	if err := database.RunMigrations(cfg.DatabaseDSN); err != nil {
		return fmt.Errorf("run database migrations: %w", err)
	}

	storage, fileStorage, err := newServerStorage(cfg, db)
	if err != nil {
		return fmt.Errorf("create server storage: %w", err)
	}

	serviceOptions := make([]service.MetricServiceOption, 0, 1)
	if cfg.StoreInterval == 0 && fileStorage != nil {
		serviceOptions = append(serviceOptions, service.WithUpdatePersister(fileStorage.Save))
	}

	metricService, err := service.NewMetricService(storage, serviceOptions...)
	if err != nil {
		return fmt.Errorf("create metric service: %w", err)
	}

	stopSave := startPeriodicSave(context.Background(), cfg.StoreInterval, storage, fileStorage, logger)
	defer stopSave()

	var observers []audit.Observer
	if cfg.AuditFile != "" {
		observer, err := audit.NewFileObserver(cfg.AuditFile)
		if err != nil {
			return fmt.Errorf("create file audit observer: %w", err)
		}
		defer func() {
			if err := observer.Close(); err != nil {
				logger.Error("close audit file", zap.Error(err))
			}
		}()
		observers = append(observers, observer)
	}
	if cfg.AuditURL != "" {
		observer, err := audit.NewHTTPObserver(cfg.AuditURL)
		if err != nil {
			return fmt.Errorf("create HTTP audit observer: %w", err)
		}
		observers = append(observers, observer)
	}
	handler, err := newServerHandler(logger, metricService, db, cfg.Key, observers...)
	if err != nil {
		return fmt.Errorf("create server handler: %w", err)
	}
	defer handler.Close()
	server := &http.Server{Addr: cfg.ServerAddress, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second}
	return serveUntilCancelled(ctx, server)
}

// serveUntilCancelled stops accepting requests and drains active handlers on
// cancellation or a listener error. It does not cancel in-flight request contexts.
func serveUntilCancelled(ctx context.Context, server *http.Server) error {
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	var serveErr error
	select {
	case serveErr = <-result:
	case <-ctx.Done():
		// Use a fresh context: the signal context has already been cancelled.
		shutdownErr := server.Shutdown(context.Background())
		serveErr = <-result
		if errors.Is(serveErr, http.ErrServerClosed) {
			serveErr = nil
		}
		return errors.Join(serveErr, shutdownErr)
	}
	shutdownErr := server.Shutdown(context.Background())
	if errors.Is(serveErr, http.ErrServerClosed) {
		serveErr = nil
	}
	return errors.Join(serveErr, shutdownErr)
}

// serverHandler owns the audit workers attached to its metric handlers.
type serverHandler struct {
	http.Handler
	closeAudit func()
}

func (h *serverHandler) Close() { h.closeAudit() }

func parseConfig(args []string) (config, error) {
	cfg := config{
		ServerAddress: defaultServerAddress,
		StoreInterval: defaultStoreIntervalSeconds * time.Second,
		Restore:       defaultRestore,
	}

	flags := flag.NewFlagSet("server", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&cfg.AuditFile, "audit-file", "", "audit log file path")
	flags.StringVar(&cfg.AuditURL, "audit-url", "", "audit receiver URL")
	flags.StringVar(&cfg.ServerAddress, "a", cfg.ServerAddress, "HTTP server address")
	flags.Var(secondsDurationFlag{value: &cfg.StoreInterval}, "i", "metrics store interval in seconds")
	flags.StringVar(&cfg.FileStoragePath, "f", cfg.FileStoragePath, "metrics storage file path")
	flags.BoolVar(&cfg.Restore, "r", cfg.Restore, "restore metrics from storage file")
	flags.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "PostgreSQL database DSN")
	flags.StringVar(&cfg.Key, "k", cfg.Key, "SHA256 hash key")

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

func newServerHandler(logger *zap.Logger, srv service.Service, databasePinger handler.DatabasePinger, key string, observers ...audit.Observer) (*serverHandler, error) {
	router := chi.NewRouter()
	router.Use(middleware.WithLogging(logger))
	router.Use(middleware.WithHashSHA256(key))
	router.Use(middleware.WithGzip)

	metricsHandler, err := handler.NewMetricsHandler(srv, logger, observers...)
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

	return &serverHandler{Handler: router, closeAudit: metricsHandler.Close}, nil
}

func newServerStorage(cfg config, db *sql.DB) (repository.Storage, *filestorage.FileStorage, error) {
	if cfg.DatabaseDSN != "" {
		if db == nil {
			return nil, nil, fmt.Errorf("postgres database is nil")
		}

		return postgres.NewPgStorage(db), nil, nil
	}

	storage := memstorage.NewMemStorage()
	if cfg.FileStoragePath == "" {
		return storage, nil, nil
	}

	fileStorage := filestorage.NewFileStorage(cfg.FileStoragePath)
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
	fileStorage *filestorage.FileStorage,
	logger *zap.Logger,
) func() {
	if interval <= 0 || fileStorage == nil {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})

	ticker := time.NewTicker(interval)
	go func() {
		defer close(done)
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
	return func() { cancel(); <-done }
}
