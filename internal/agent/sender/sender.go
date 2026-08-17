package sender

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/retry"
	"github.com/tomkqwe/metrics/internal/signature"
)

var (
	ErrInvalidMetric        = errors.New("invalid metric")
	ErrTransport            = errors.New("transport error")
	ErrUnexpectedStatusCode = errors.New("unexpected status code")
)

type HTTPSender struct {
	baseURL     string
	client      *http.Client
	retryDelays []time.Duration
	key         string
}

type HTTPSenderOption func(*HTTPSender)

func WithKey(key string) HTTPSenderOption {
	return func(s *HTTPSender) {
		s.key = key
	}
}

func NewHTTPSender(baseURL string, opts ...HTTPSenderOption) *HTTPSender {
	return NewHTTPSenderWithClient(baseURL, http.DefaultClient, opts...)
}

func NewHTTPSenderWithClient(baseURL string, client *http.Client, opts ...HTTPSenderOption) *HTTPSender {
	if client == nil {
		client = http.DefaultClient
	}

	s := &HTTPSender{
		baseURL:     strings.TrimRight(baseURL, "/"),
		client:      client,
		retryDelays: retry.DefaultDelays(),
	}
	for _, opt := range opts {
		opt(s)
	}

	return s
}

func (s *HTTPSender) Send(ctx context.Context, metrics []models.Metric) error {
	if ctx == nil {
		ctx = context.Background()
	}

	for _, metric := range metrics {
		if err := validateMetric(metric); err != nil {
			return err
		}
	}

	if len(metrics) == 0 {
		return nil
	}

	body, err := compressedBody(metrics)
	if err != nil {
		return err
	}

	return retry.DoWithDelays(ctx, s.retryDelays, func() error {
		return s.sendCompressedBody(ctx, body)
	}, isRetriableSendError)
}

func (s *HTTPSender) sendCompressedBody(ctx context.Context, body []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.metricsURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")
	if s.key != "" {
		req.Header.Set(signature.Header, signature.Calculate(body, s.key))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrTransport, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, resp.StatusCode)
	}

	return nil
}

func (s *HTTPSender) metricsURL() string {
	return s.baseURL + "/updates/"
}

func compressedBody(metrics []models.Metric) ([]byte, error) {
	var body bytes.Buffer
	writer := gzip.NewWriter(&body)
	if err := json.NewEncoder(writer).Encode(metrics); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return body.Bytes(), nil
}

func isRetriableSendError(err error) bool {
	return errors.Is(err, ErrTransport)
}

func validateMetric(metric models.Metric) error {
	switch metric.MType {
	case models.MetricTypeGauge:
		if metric.Value == nil {
			return fmt.Errorf("%w: gauge metric %q has nil value", ErrInvalidMetric, metric.ID)
		}
		return nil
	case models.MetricTypeCounter:
		if metric.Delta == nil {
			return fmt.Errorf("%w: counter metric %q has nil delta", ErrInvalidMetric, metric.ID)
		}
		return nil
	default:
		return fmt.Errorf("%w: unknown metric type %q", ErrInvalidMetric, metric.MType)
	}
}
