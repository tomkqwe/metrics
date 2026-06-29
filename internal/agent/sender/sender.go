package sender

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
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
	value, err := metricValue(metric)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.metricURL(metric, value), http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "text/plain")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, resp.StatusCode)
	}

	return nil
}

func (s *HTTPSender) metricURL(metric models.Metric, value string) string {
	return s.baseURL +
		"/update/" +
		url.PathEscape(metric.MType) +
		"/" +
		url.PathEscape(metric.ID) +
		"/" +
		url.PathEscape(value)
}

func metricValue(metric models.Metric) (string, error) {
	switch metric.MType {
	case models.MetricTypeGauge:
		if metric.Value == nil {
			return "", fmt.Errorf("%w: gauge metric %q has nil value", ErrInvalidMetric, metric.ID)
		}
		return strconv.FormatFloat(*metric.Value, 'f', -1, 64), nil
	case models.MetricTypeCounter:
		if metric.Delta == nil {
			return "", fmt.Errorf("%w: counter metric %q has nil delta", ErrInvalidMetric, metric.ID)
		}
		return strconv.FormatInt(*metric.Delta, 10), nil
	default:
		return "", fmt.Errorf("%w: unknown metric type %q", ErrInvalidMetric, metric.MType)
	}
}
