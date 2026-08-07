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
}

func NewHTTPSender(baseURL string) *HTTPSender {
	return NewHTTPSenderWithClient(baseURL, http.DefaultClient)
}

func NewHTTPSenderWithClient(baseURL string, client *http.Client) *HTTPSender {
	if client == nil {
		client = http.DefaultClient
	}

	return &HTTPSender{
		baseURL:     strings.TrimRight(baseURL, "/"),
		client:      client,
		retryDelays: retry.DefaultDelays(),
	}
}

func (s *HTTPSender) Send(metrics []models.Metric) error {
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

	return retry.DoWithDelays(context.Background(), s.retryDelays, func() error {
		return s.sendCompressedBody(body)
	}, isRetriableSendError)
}

func (s *HTTPSender) sendCompressedBody(body []byte) error {
	req, err := http.NewRequest(http.MethodPost, s.metricsURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

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
