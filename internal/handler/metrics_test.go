package handler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	models "github.com/tomkqwe/metrics/internal/model"
)

func TestNewMetricsHandlerReturnsErrorForNilService(t *testing.T) {
	_, err := NewMetricsHandler(nil)
	if !errors.Is(err, ErrServiceInvalid) {
		t.Fatalf("NewMetricsHandler() error = %v, want %v", err, ErrServiceInvalid)
	}
}

func TestMetricsHandlerUpdateMetricSuccess(t *testing.T) {
	service := &fakeService{}
	handler, err := NewMetricsHandler(service)
	if err != nil {
		t.Fatalf("NewMetricsHandler() error = %v", err)
	}

	response := executeRequest(newTestRouter(handler), http.MethodPost, "/update/gauge/Alloc/12.5")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", contentType, "text/plain; charset=utf-8")
	}
	if body := response.Body.String(); body != "gauge Alloc = 12.5" {
		t.Fatalf("body = %q, want %q", body, "gauge Alloc = 12.5")
	}
	if service.metricType != "gauge" {
		t.Fatalf("UpdateMetric() metricType = %q, want gauge", service.metricType)
	}
	if service.metricName != "Alloc" {
		t.Fatalf("UpdateMetric() metricName = %q, want Alloc", service.metricName)
	}
	if service.value != "12.5" {
		t.Fatalf("UpdateMetric() value = %q, want 12.5", service.value)
	}
}

func TestMetricsHandlerUpdateMetricRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		serviceErr error
		wantStatus int
		wantCalled bool
	}{
		{
			name:       "wrong method",
			method:     http.MethodGet,
			path:       "/update/gauge/Alloc/12.5",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "short path",
			method:     http.MethodPost,
			path:       "/update/gauge/Alloc",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "long path",
			method:     http.MethodPost,
			path:       "/update/gauge/Alloc/12.5/extra",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "empty metric name",
			method:     http.MethodPost,
			path:       "/update/gauge//12.5",
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "service error",
			method:     http.MethodPost,
			path:       "/update/gauge/Alloc/not-float",
			serviceErr: errors.New("service error"),
			wantStatus: http.StatusBadRequest,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeService{err: tt.serviceErr}
			handler, err := NewMetricsHandler(service)
			if err != nil {
				t.Fatalf("NewMetricsHandler() error = %v", err)
			}

			response := executeRequest(newTestRouter(handler), tt.method, tt.path)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if service.called != tt.wantCalled {
				t.Fatalf("service called = %v, want %v", service.called, tt.wantCalled)
			}
		})
	}
}

func TestMetricsHandlerGetMetricValueSuccess(t *testing.T) {
	service := &fakeService{getValue: "12.5"}
	handler, err := NewMetricsHandler(service)
	if err != nil {
		t.Fatalf("NewMetricsHandler() error = %v", err)
	}

	response := executeRequest(newTestRouter(handler), http.MethodGet, "/value/gauge/Alloc")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/plain; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", contentType, "text/plain; charset=utf-8")
	}
	if body := response.Body.String(); body != "12.5" {
		t.Fatalf("body = %q, want 12.5", body)
	}
	if service.getMetricType != "gauge" {
		t.Fatalf("GetMetricValue() metricType = %q, want gauge", service.getMetricType)
	}
	if service.getMetricName != "Alloc" {
		t.Fatalf("GetMetricValue() metricName = %q, want Alloc", service.getMetricName)
	}
}

func TestMetricsHandlerGetMetricValueReturnsNotFound(t *testing.T) {
	service := &fakeService{getErr: errors.New("not found")}
	handler, err := NewMetricsHandler(service)
	if err != nil {
		t.Fatalf("NewMetricsHandler() error = %v", err)
	}

	response := executeRequest(newTestRouter(handler), http.MethodGet, "/value/gauge/Unknown")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func TestMetricsHandlerListMetrics(t *testing.T) {
	gaugeValue := 12.5
	counterValue := int64(3)
	service := &fakeService{
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
	}
	handler, err := NewMetricsHandler(service)
	if err != nil {
		t.Fatalf("NewMetricsHandler() error = %v", err)
	}

	response := executeRequest(newTestRouter(handler), http.MethodGet, "/")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want %q", contentType, "text/html; charset=utf-8")
	}
	body := response.Body.String()
	for _, want := range []string{"Alloc", "12.5", "PollCount", "3"} {
		if !strings.Contains(body, want) {
			t.Fatalf("body does not contain %q: %s", want, body)
		}
	}
}

func newTestRouter(handler *MetricsHandler) http.Handler {
	router := chi.NewRouter()
	router.Post("/update/{metricType}/{metricName}/{rawValue}", handler.UpdateMetric)
	router.Get("/value/{metricType}/{metricName}", handler.GetMetricValue)
	router.Get("/", handler.ListMetrics)

	return router
}

func executeRequest(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, http.NoBody)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	return response
}

type fakeService struct {
	called        bool
	metricType    string
	metricName    string
	value         string
	err           error
	getMetricType string
	getMetricName string
	getValue      string
	getErr        error
	metrics       []models.Metric
}

func (s *fakeService) UpdateMetric(metricType, metricName, value string) error {
	s.called = true
	s.metricType = metricType
	s.metricName = metricName
	s.value = value

	return s.err
}

func (s *fakeService) GetMetricValue(metricType, metricName string) (string, error) {
	s.getMetricType = metricType
	s.getMetricName = metricName

	return s.getValue, s.getErr
}

func (s *fakeService) ListMetrics() []models.Metric {
	return s.metrics
}
