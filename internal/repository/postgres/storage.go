package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/postgreserr"
	"github.com/tomkqwe/metrics/internal/repository"
	"github.com/tomkqwe/metrics/internal/retry"
)

var _ repository.Storage = (*Storage)(nil)
var _ repository.BatchStorage = (*Storage)(nil)

var errNilDatabase = errors.New("postgres storage database is nil")

const (
	upsertGaugeQuery = `
		INSERT INTO gauges (id, value)
		VALUES ($1, $2)
		ON CONFLICT (id)
		DO UPDATE SET value = EXCLUDED.value
	`
	upsertCounterQuery = `
		INSERT INTO counters (id, value)
		VALUES ($1, $2)
		ON CONFLICT (id)
		DO UPDATE SET value = counters.value + EXCLUDED.value
	`
)

type Storage struct {
	db          *sql.DB
	retryDelays []time.Duration
}

func NewPgStorage(db *sql.DB) *Storage {
	return &Storage{
		db:          db,
		retryDelays: retry.DefaultDelays(),
	}
}

func (s *Storage) UpdateGauge(ctx context.Context, name string, value models.Gauge) error {
	if err := s.ready(); err != nil {
		return err
	}

	if err := s.executeWithRetry(ctx, func(ctx context.Context) error {
		_, err := s.db.ExecContext(ctx, upsertGaugeQuery, name, float64(value))
		return err
	}); err != nil {
		return fmt.Errorf("update gauge %q: %w", name, err)
	}

	return nil
}

func (s *Storage) UpdateCounter(ctx context.Context, name string, value models.Counter) error {
	if err := s.ready(); err != nil {
		return err
	}

	if err := s.executeWithRetry(ctx, func(ctx context.Context) error {
		_, err := s.db.ExecContext(ctx, upsertCounterQuery, name, int64(value))
		return err
	}); err != nil {
		return fmt.Errorf("update counter %q: %w", name, err)
	}

	return nil
}

func (s *Storage) UpdateMetrics(ctx context.Context, metrics []models.Metric) error {
	if len(metrics) == 0 {
		return nil
	}
	if err := s.ready(); err != nil {
		return err
	}

	if err := s.executeWithRetry(ctx, func(ctx context.Context) error {
		return s.updateMetricsOnce(ctx, metrics)
	}); err != nil {
		return fmt.Errorf("update metrics batch: %w", err)
	}

	return nil
}

func (s *Storage) GetGauge(ctx context.Context, name string) (models.Gauge, bool, error) {
	if err := s.ready(); err != nil {
		return 0, false, err
	}

	var value float64
	err := s.executeWithRetry(ctx, func(ctx context.Context) error {
		return s.db.QueryRowContext(ctx, `
			SELECT value
			FROM gauges
			WHERE id = $1
		`, name).Scan(&value)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("get gauge %q: %w", name, err)
	}

	return models.Gauge(value), true, nil
}

func (s *Storage) GetCounter(ctx context.Context, name string) (models.Counter, bool, error) {
	if err := s.ready(); err != nil {
		return 0, false, err
	}

	var value int64
	err := s.executeWithRetry(ctx, func(ctx context.Context) error {
		return s.db.QueryRowContext(ctx, `
			SELECT value
			FROM counters
			WHERE id = $1
		`, name).Scan(&value)
	})
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("get counter %q: %w", name, err)
	}

	return models.Counter(value), true, nil
}

func (s *Storage) Snapshot(ctx context.Context) ([]models.Metric, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}

	var metrics []models.Metric
	if err := s.executeWithRetry(ctx, func(ctx context.Context) error {
		gauges, err := s.snapshotGauges(ctx)
		if err != nil {
			return err
		}

		counters, err := s.snapshotCounters(ctx)
		if err != nil {
			return err
		}

		metrics = append(metrics[:0], gauges...)
		metrics = append(metrics, counters...)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("snapshot metrics: %w", err)
	}

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].MType == metrics[j].MType {
			return metrics[i].ID < metrics[j].ID
		}
		return metrics[i].MType < metrics[j].MType
	})

	return metrics, nil
}

func (s *Storage) snapshotGauges(ctx context.Context) (metrics []models.Metric, err error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, value
		FROM gauges
	`)
	if err != nil {
		return nil, fmt.Errorf("query gauges: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close gauges rows: %w", closeErr)
		}
	}()

	for rows.Next() {
		var (
			name  string
			value float64
		)
		if err = rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("scan gauge: %w", err)
		}

		gaugeValue := value
		metrics = append(metrics, models.Metric{
			ID:    name,
			MType: models.MetricTypeGauge,
			Value: &gaugeValue,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate gauges: %w", err)
	}

	return metrics, nil
}

func (s *Storage) snapshotCounters(ctx context.Context) (metrics []models.Metric, err error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, value
		FROM counters
	`)
	if err != nil {
		return nil, fmt.Errorf("query counters: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close counters rows: %w", closeErr)
		}
	}()

	for rows.Next() {
		var (
			name  string
			value int64
		)
		if err = rows.Scan(&name, &value); err != nil {
			return nil, fmt.Errorf("scan counter: %w", err)
		}

		counterValue := value
		metrics = append(metrics, models.Metric{
			ID:    name,
			MType: models.MetricTypeCounter,
			Delta: &counterValue,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate counters: %w", err)
	}

	return metrics, nil
}

func (s *Storage) updateMetricsOnce(ctx context.Context, metrics []models.Metric) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin metrics batch transaction: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
				err = errors.Join(err, fmt.Errorf("rollback metrics batch transaction: %w", rollbackErr))
			}
		}
	}()

	for _, metric := range metrics {
		if err = updateMetricTx(ctx, tx, metric); err != nil {
			return fmt.Errorf("update metric %q in batch: %w", metric.ID, err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit metrics batch transaction: %w", err)
	}
	committed = true

	return nil
}

func updateMetricTx(ctx context.Context, tx *sql.Tx, metric models.Metric) error {
	switch metric.MType {
	case models.MetricTypeGauge:
		if metric.Value == nil {
			return fmt.Errorf("gauge metric %q has nil value", metric.ID)
		}
		if _, err := tx.ExecContext(ctx, upsertGaugeQuery, metric.ID, *metric.Value); err != nil {
			return fmt.Errorf("upsert gauge: %w", err)
		}
	case models.MetricTypeCounter:
		if metric.Delta == nil {
			return fmt.Errorf("counter metric %q has nil delta", metric.ID)
		}
		if _, err := tx.ExecContext(ctx, upsertCounterQuery, metric.ID, *metric.Delta); err != nil {
			return fmt.Errorf("upsert counter: %w", err)
		}
	default:
		return fmt.Errorf("unknown metric type %q", metric.MType)
	}

	return nil
}

func (s *Storage) executeWithRetry(ctx context.Context, operation func(context.Context) error) error {
	if ctx == nil {
		ctx = context.Background()
	}

	return retry.DoWithDelays(ctx, s.retryDelays, func() error {
		return operation(ctx)
	}, postgreserr.IsConnectionException)
}

func (s *Storage) ready() error {
	if s != nil && s.db != nil {
		return nil
	}

	return errNilDatabase
}
