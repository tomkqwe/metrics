package sender

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	models "github.com/tomkqwe/metrics/internal/model"
)

var (
	ErrInvalidMetric        = errors.New("invalid metric")
	ErrUnexpectedStatusCode = errors.New("unexpected status code")
)

type HTTPSender struct {
	baseURL string
	client  *http.Client
}

func NewHTTPSender(baseURL string) *HTTPSender {
	return NewHTTPSenderWithClient(baseURL, http.DefaultClient)
}

func NewHTTPSenderWithClient(baseURL string, client *http.Client) *HTTPSender {
	if client == nil {
		client = http.DefaultClient
	}

	return &HTTPSender{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  client,
	}
}

func (s *HTTPSender) Send(metrics []models.Metric) error {
	for _, metric := range metrics {
		if err := s.sendMetric(metric); err != nil {
			return err
		}
	}

	return nil
}

func (s *HTTPSender) sendMetric(metric models.Metric) error {
	if err := validateMetric(metric); err != nil {
		return err
	}

	body, err := compressedBody(metric)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.metricURL(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Accept-Encoding", "gzip")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
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

func (s *HTTPSender) metricURL() string {
	return s.baseURL + "/update"
}

func compressedBody(metric models.Metric) (*bytes.Buffer, error) {
	var body bytes.Buffer
	writer := gzip.NewWriter(&body)
	if err := json.NewEncoder(writer).Encode(metric); err != nil {
		_ = writer.Close()
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	return &body, nil
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
