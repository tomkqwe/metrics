package sender

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestHTTPSenderSendPostsMetrics(t *testing.T) {
	var requests []receivedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Fatalf("create gzip reader: %v", err)
		}
		defer func() {
			_ = reader.Close()
		}()

		var metrics []models.Metric
		if err := json.NewDecoder(reader).Decode(&metrics); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests = append(requests, receivedRequest{
			method:          r.Method,
			path:            r.URL.Path,
			contentType:     r.Header.Get("Content-Type"),
			contentEncoding: r.Header.Get("Content-Encoding"),
			acceptEncoding:  r.Header.Get("Accept-Encoding"),
			metrics:         metrics,
		})
		w.Header().Set("Content-Encoding", "gzip")
		w.WriteHeader(http.StatusOK)
		gzipWriter := gzip.NewWriter(w)
		_, _ = io.WriteString(gzipWriter, `{"status":"ok"}`)
		_ = gzipWriter.Close()
	}))
	defer server.Close()

	gaugeValue := 12.5
	counterValue := int64(3)
	s := NewHTTPSender(server.URL)

	err := s.Send([]models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &gaugeValue,
		},
		{
			ID:    "PollCount",
			MType: models.MetricTypeCounter,
			Delta: &counterValue,
		},
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	expected := []receivedRequest{
		{
			method:          http.MethodPost,
			path:            "/updates/",
			contentType:     "application/json",
			contentEncoding: "gzip",
			acceptEncoding:  "gzip",
			metrics: []models.Metric{
				{
					ID:    "Alloc",
					MType: models.MetricTypeGauge,
					Value: &gaugeValue,
				},
				{
					ID:    "PollCount",
					MType: models.MetricTypeCounter,
					Delta: &counterValue,
				},
			},
		},
	}
	assertRequests(t, requests, expected)
}

func TestHTTPSenderSendSkipsEmptyBatch(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	s := NewHTTPSender(server.URL)
	if err := s.Send(nil); err != nil {
		t.Fatalf("Send(nil) error = %v", err)
	}
	if err := s.Send([]models.Metric{}); err != nil {
		t.Fatalf("Send(empty) error = %v", err)
	}

	if requests != 0 {
		t.Fatalf("requests = %d, want 0", requests)
	}
}

func TestHTTPSenderSendReturnsErrorOnUnexpectedStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	value := 12.5
	s := NewHTTPSender(server.URL)

	err := s.Send([]models.Metric{
		{
			ID:    "Alloc",
			MType: models.MetricTypeGauge,
			Value: &value,
		},
	})
	if !errors.Is(err, ErrUnexpectedStatusCode) {
		t.Fatalf("Send() error = %v, want ErrUnexpectedStatusCode", err)
	}
}

func TestHTTPSenderSendReturnsErrorForInvalidMetric(t *testing.T) {
	s := NewHTTPSender("http://localhost:8080")

	tests := []struct {
		name   string
		metric models.Metric
	}{
		{
			name: "gauge without value",
			metric: models.Metric{
				ID:    "Alloc",
				MType: models.MetricTypeGauge,
			},
		},
		{
			name: "counter without delta",
			metric: models.Metric{
				ID:    "PollCount",
				MType: models.MetricTypeCounter,
			},
		},
		{
			name: "unknown type",
			metric: models.Metric{
				ID:    "Alloc",
				MType: "unknown",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := s.Send([]models.Metric{tt.metric})
			if !errors.Is(err, ErrInvalidMetric) {
				t.Fatalf("Send() error = %v, want ErrInvalidMetric", err)
			}
		})
	}
}

type receivedRequest struct {
	method          string
	path            string
	contentType     string
	contentEncoding string
	acceptEncoding  string
	metrics         []models.Metric
}

func assertRequests(t *testing.T, got, want []receivedRequest) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("requests count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].method != want[i].method {
			t.Fatalf("request %d method = %q, want %q", i, got[i].method, want[i].method)
		}
		if got[i].path != want[i].path {
			t.Fatalf("request %d path = %q, want %q", i, got[i].path, want[i].path)
		}
		if got[i].contentType != want[i].contentType {
			t.Fatalf("request %d Content-Type = %q, want %q", i, got[i].contentType, want[i].contentType)
		}
		if got[i].contentEncoding != want[i].contentEncoding {
			t.Fatalf("request %d Content-Encoding = %q, want %q", i, got[i].contentEncoding, want[i].contentEncoding)
		}
		if got[i].acceptEncoding != want[i].acceptEncoding {
			t.Fatalf("request %d Accept-Encoding = %q, want %q", i, got[i].acceptEncoding, want[i].acceptEncoding)
		}
		assertMetrics(t, i, got[i].metrics, want[i].metrics)
	}
}

func assertMetrics(t *testing.T, requestIndex int, got, want []models.Metric) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("request %d metrics len = %d, want %d", requestIndex, len(got), len(want))
	}
	for i := range want {
		if got[i].ID != want[i].ID {
			t.Fatalf("request %d metric %d ID = %q, want %q", requestIndex, i, got[i].ID, want[i].ID)
		}
		if got[i].MType != want[i].MType {
			t.Fatalf("request %d metric %d MType = %q, want %q", requestIndex, i, got[i].MType, want[i].MType)
		}
		assertFloat64Ptr(t, requestIndex, i, got[i].Value, want[i].Value)
		assertInt64Ptr(t, requestIndex, i, got[i].Delta, want[i].Delta)
	}
}

func assertFloat64Ptr(t *testing.T, requestIndex, metricIndex int, got, want *float64) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("request %d metric %d Value = %v, want %v", requestIndex, metricIndex, got, want)
	}
	if *got != *want {
		t.Fatalf("request %d metric %d Value = %v, want %v", requestIndex, metricIndex, *got, *want)
	}
}

func assertInt64Ptr(t *testing.T, requestIndex, metricIndex int, got, want *int64) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("request %d metric %d Delta = %v, want %v", requestIndex, metricIndex, got, want)
	}
	if *got != *want {
		t.Fatalf("request %d metric %d Delta = %v, want %v", requestIndex, metricIndex, *got, *want)
	}
}
