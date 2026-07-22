package sender

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	models "github.com/tomkqwe/metrics/internal/model"
)

func TestHTTPSenderSendPostsMetrics(t *testing.T) {
	var requests []receivedRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var metric models.Metric
		if err := json.NewDecoder(r.Body).Decode(&metric); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		requests = append(requests, receivedRequest{
			method:      r.Method,
			path:        r.URL.Path,
			contentType: r.Header.Get("Content-Type"),
			metric:      metric,
		})
		w.WriteHeader(http.StatusOK)
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
			method:      http.MethodPost,
			path:        "/update",
			contentType: "application/json",
			metric: models.Metric{
				ID:    "Alloc",
				MType: models.MetricTypeGauge,
				Value: &gaugeValue,
			},
		},
		{
			method:      http.MethodPost,
			path:        "/update",
			contentType: "application/json",
			metric: models.Metric{
				ID:    "PollCount",
				MType: models.MetricTypeCounter,
				Delta: &counterValue,
			},
		},
	}
	assertRequests(t, requests, expected)
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
	method      string
	path        string
	contentType string
	metric      models.Metric
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
		if got[i].metric.ID != want[i].metric.ID {
			t.Fatalf("request %d metric ID = %q, want %q", i, got[i].metric.ID, want[i].metric.ID)
		}
		if got[i].metric.MType != want[i].metric.MType {
			t.Fatalf("request %d metric MType = %q, want %q", i, got[i].metric.MType, want[i].metric.MType)
		}
		assertFloat64Ptr(t, i, got[i].metric.Value, want[i].metric.Value)
		assertInt64Ptr(t, i, got[i].metric.Delta, want[i].metric.Delta)
	}
}

func assertFloat64Ptr(t *testing.T, requestIndex int, got, want *float64) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("request %d metric Value = %v, want %v", requestIndex, got, want)
	}
	if *got != *want {
		t.Fatalf("request %d metric Value = %v, want %v", requestIndex, *got, *want)
	}
}

func assertInt64Ptr(t *testing.T, requestIndex int, got, want *int64) {
	t.Helper()

	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Fatalf("request %d metric Delta = %v, want %v", requestIndex, got, want)
	}
	if *got != *want {
		t.Fatalf("request %d metric Delta = %v, want %v", requestIndex, *got, *want)
	}
}
