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
	"github.com/tomkqwe/metrics/internal/buildinfo"
	configfile "github.com/tomkqwe/metrics/internal/config"
	"github.com/tomkqwe/metrics/internal/encryption"
)

// Build metadata is set at link time using -ldflags -X.
var (
	buildVersion = "N/A"
	buildDate    = "N/A"
	buildCommit  = "N/A"
)

const (
	defaultServerAddress         = "localhost:8080"
	defaultPollIntervalSeconds   = 2
	defaultReportIntervalSeconds = 10
	defaultRateLimit             = 1
)

type config struct {
	serverAddress  string
	pollInterval   time.Duration
	reportInterval time.Duration
	key            string
	cryptoKey      string
	rateLimit      int
}

type envConfig struct {
	ServerAddress  string `env:"ADDRESS"`
	PollInterval   int    `env:"POLL_INTERVAL"`
	ReportInterval int    `env:"REPORT_INTERVAL"`
	Key            string `env:"KEY"`
	CryptoKey      string `env:"CRYPTO_KEY"`
	RateLimit      int    `env:"RATE_LIMIT"`
}

func main() {
	buildinfo.Print(os.Stdout, buildVersion, buildDate, buildCommit)

	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "parse config: %v\n", err)
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
	cfg := config{pollInterval: defaultPollIntervalSeconds * time.Second, reportInterval: defaultReportIntervalSeconds * time.Second}
	var configPath string

	flags := flag.NewFlagSet("agent", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&configPath, "c", "", "JSON configuration file path")
	flags.StringVar(&configPath, "config", "", "JSON configuration file path")
	flags.StringVar(&cfg.serverAddress, "a", defaultServerAddress, "HTTP server address")
	flags.Var((*configfile.Duration)(&cfg.reportInterval), "r", "metrics report interval in seconds")
	flags.Var((*configfile.Duration)(&cfg.pollInterval), "p", "metrics poll interval in seconds")
	flags.StringVar(&cfg.cryptoKey, "crypto-key", "", "RSA public key PEM file path")
	flags.StringVar(&cfg.key, "k", cfg.key, "SHA256 hash key")
	flags.IntVar(&cfg.rateLimit, "l", defaultRateLimit, "maximum number of concurrent outgoing requests")

	if err := flags.Parse(args); err != nil {
		return config{}, err
	}

	// Parse once to discover the file, then replay explicit flags over its values.
	if err := configfile.Load(configPath, &struct {
		Address        *string              `json:"address"`
		PollInterval   *configfile.Duration `json:"poll_interval"`
		ReportInterval *configfile.Duration `json:"report_interval"`
		Key            *string              `json:"key"`
		CryptoKey      *string              `json:"crypto_key"`
		RateLimit      *int                 `json:"rate_limit"`
	}{&cfg.serverAddress, (*configfile.Duration)(&cfg.pollInterval), (*configfile.Duration)(&cfg.reportInterval), &cfg.key, &cfg.cryptoKey, &cfg.rateLimit}); err != nil {
		return config{}, err
	}
	if err := flags.Parse(args); err != nil {
		return config{}, err
	}

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
	if _, ok := os.LookupEnv("CRYPTO_KEY"); ok {
		cfg.cryptoKey = eCfg.CryptoKey
	}
	if _, ok := os.LookupEnv("KEY"); ok {
		cfg.key = eCfg.Key
	}
	if _, ok := os.LookupEnv("RATE_LIMIT"); ok {
		cfg.rateLimit = eCfg.RateLimit
	}

	if cfg.rateLimit <= 0 {
		return config{}, fmt.Errorf("rate limit must be positive")
	}

	return cfg, nil
}

func newAgent(cfg config) (*agent.Agent, error) {
	publicKey, err := encryption.LoadPublicKey(cfg.cryptoKey)
	if err != nil {
		return nil, fmt.Errorf("load agent crypto key: %w", err)
	}
	return agent.NewAgent(
		collector.NewRuntimeCollector(),
		storage.NewMemoryStorage(),
		sender.NewHTTPSender(serverURL(cfg.serverAddress), sender.WithKey(cfg.key), sender.WithPublicKey(publicKey)),
		cfg.pollInterval,
		cfg.reportInterval,
		agent.WithAdditionalCollector(collector.NewSystemCollector()),
		agent.WithRateLimit(cfg.rateLimit),
	)
}

func serverURL(address string) string {
	if strings.HasPrefix(address, "http://") || strings.HasPrefix(address, "https://") {
		return address
	}

	return "http://" + address
}
