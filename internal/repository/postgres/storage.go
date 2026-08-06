package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"

	"go.uber.org/zap"

	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/repository"
)

const (
	logFieldMetric = "metric"
	logFieldValue  = "value"
)

var _ repository.Storage = (*Storage)(nil)

type Storage struct {
	db  *sql.DB
	log *zap.Logger
}

func NewPgStorage(db *sql.DB) *Storage {
	return NewPgStorageWithLogger(db, nil)
}

func NewPgStorageWithLogger(db *sql.DB, logger *zap.Logger) *Storage {
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Storage{
		db:  db,
		log: logger,
	}
}

func (s *Storage) UpdateGauge(name string, value models.Gauge) {
	if !s.ready("failed to update gauge") {
		return
	}

	query := `
		INSERT INTO gauges (id, value)
		VALUES ($1, $2)
		ON CONFLICT (id)
		DO UPDATE SET value = EXCLUDED.value
	`
	_, err := s.db.ExecContext(context.Background(), query, name, float64(value))
	if err != nil {
		s.logger().Error("failed to update gauge",
			zap.Error(err),
			zap.String(logFieldMetric, name),
			zap.Float64(logFieldValue, float64(value)),
		)
	}
}

func (s *Storage) UpdateCounter(name string, value models.Counter) {
	if !s.ready("failed to update counter") {
		return
	}

	query := `
		INSERT INTO counters (id, value)
		VALUES ($1, $2)
		ON CONFLICT (id)
		DO UPDATE SET value = counters.value + EXCLUDED.value
	`
	_, err := s.db.ExecContext(context.Background(), query, name, int64(value))
	if err != nil {
		s.logger().Error("failed to update counter",
			zap.Error(err),
			zap.String(logFieldMetric, name),
			zap.Int64(logFieldValue, int64(value)),
		)
	}
}

func (s *Storage) GetGauge(name string) (models.Gauge, bool) {
	if !s.ready("failed to get gauge") {
		return 0, false
	}

	var value float64
	err := s.db.QueryRowContext(context.Background(), `
		SELECT value
		FROM gauges
		WHERE id = $1
	`, name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		s.logger().Error("failed to get gauge",
			zap.Error(err),
			zap.String(logFieldMetric, name),
		)
		return 0, false
	}

	return models.Gauge(value), true
}

func (s *Storage) GetCounter(name string) (models.Counter, bool) {
	if !s.ready("failed to get counter") {
		return 0, false
	}

	var value int64
	err := s.db.QueryRowContext(context.Background(), `
		SELECT value
		FROM counters
		WHERE id = $1
	`, name).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false
	}
	if err != nil {
		s.logger().Error("failed to get counter",
			zap.Error(err),
			zap.String(logFieldMetric, name),
		)
		return 0, false
	}

	return models.Counter(value), true
}

func (s *Storage) Snapshot() []models.Metric {
	if !s.ready("failed to snapshot metrics") {
		return nil
	}

	ctx := context.Background()
	metrics := make([]models.Metric, 0)

	gauges, err := s.snapshotGauges(ctx)
	if err != nil {
		s.logger().Error("failed to snapshot gauges", zap.Error(err))
	} else {
		metrics = append(metrics, gauges...)
	}

	counters, err := s.snapshotCounters(ctx)
	if err != nil {
		s.logger().Error("failed to snapshot counters", zap.Error(err))
	} else {
		metrics = append(metrics, counters...)
	}

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].MType == metrics[j].MType {
			return metrics[i].ID < metrics[j].ID
		}
		return metrics[i].MType < metrics[j].MType
	})

	return metrics
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

func (s *Storage) ready(message string) bool {
	if s != nil && s.db != nil {
		return true
	}

	s.logger().Error(message, zap.Error(errors.New("postgres storage database is nil")))
	return false
}

func (s *Storage) logger() *zap.Logger {
	if s == nil || s.log == nil {
		return zap.NewNop()
	}

	return s.log
}
