package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/service"
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
			path:       "/update/gauge/12.5",
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

func TestMetricsHandlerUpdateMetricJSONSuccess(t *testing.T) {
	value := 1744184459.0
	service := &fakeService{
		getJSONResult: models.Metric{
			ID:    "LastGC",
			MType: models.MetricTypeGauge,
			Value: &value,
		},
	}
	handler, err := NewMetricsHandler(service)
	if err != nil {
		t.Fatalf("NewMetricsHandler() error = %v", err)
	}

	response := executeRequestWithBody(newTestRouter(handler), http.MethodPost, "/update/", `{"id":"LastGC","type":"gauge","value":1744184459}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", contentType, "application/json")
	}
	if !service.updateJSONCalled {
		t.Fatal("UpdateMetricJSON() was not called")
	}
	if service.updateJSONMetric.ID != "LastGC" {
		t.Fatalf("UpdateMetricJSON() ID = %q, want LastGC", service.updateJSONMetric.ID)
	}
	if service.updateJSONMetric.MType != models.MetricTypeGauge {
		t.Fatalf("UpdateMetricJSON() MType = %q, want %q", service.updateJSONMetric.MType, models.MetricTypeGauge)
	}
	if service.updateJSONMetric.Value == nil {
		t.Fatal("UpdateMetricJSON() Value = nil, want 1744184459")
	}
	if *service.updateJSONMetric.Value != 1744184459 {
		t.Fatalf("UpdateMetricJSON() Value = %v, want 1744184459", *service.updateJSONMetric.Value)
	}
	if !service.getJSONCalled {
		t.Fatal("GetMetricJSON() was not called")
	}
	if service.getJSONMetric.ID != "LastGC" {
		t.Fatalf("GetMetricJSON() ID = %q, want LastGC", service.getJSONMetric.ID)
	}
	if service.getJSONMetric.MType != models.MetricTypeGauge {
		t.Fatalf("GetMetricJSON() MType = %q, want %q", service.getJSONMetric.MType, models.MetricTypeGauge)
	}

	var metric models.Metric
	if err := json.Unmarshal(response.Body.Bytes(), &metric); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if metric.ID != "LastGC" {
		t.Fatalf("response ID = %q, want LastGC", metric.ID)
	}
	if metric.MType != models.MetricTypeGauge {
		t.Fatalf("response MType = %q, want %q", metric.MType, models.MetricTypeGauge)
	}
	if metric.Value == nil {
		t.Fatal("response Value = nil, want 1744184459")
	}
	if *metric.Value != 1744184459 {
		t.Fatalf("response Value = %v, want 1744184459", *metric.Value)
	}
}

func TestMetricsHandlerUpdateMetricJSONRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		serviceErr error
		wantCalled bool
	}{
		{
			name: "invalid json",
			body: "{",
		},
		{
			name:       "service error",
			body:       `{"id":"LastGC","type":"gauge"}`,
			serviceErr: service.ErrInvalidMetricValue,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeService{updateJSONErr: tt.serviceErr}
			handler, err := NewMetricsHandler(service)
			if err != nil {
				t.Fatalf("NewMetricsHandler() error = %v", err)
			}

			response := executeRequestWithBody(newTestRouter(handler), http.MethodPost, "/update/", tt.body)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if service.updateJSONCalled != tt.wantCalled {
				t.Fatalf("UpdateMetricJSON() called = %v, want %v", service.updateJSONCalled, tt.wantCalled)
			}
		})
	}
}

func TestMetricsHandlerUpdateMetricsJSONSuccess(t *testing.T) {
	service := &fakeService{}
	handler, err := NewMetricsHandler(service)
	if err != nil {
		t.Fatalf("NewMetricsHandler() error = %v", err)
	}

	response := executeRequestWithBody(
		newTestRouter(handler),
		http.MethodPost,
		"/updates/",
		`[{"id":"Alloc","type":"gauge","value":12.5},{"id":"PollCount","type":"counter","delta":3}]`,
	)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", contentType, "application/json")
	}
	if !service.updateJSONBatchCalled {
		t.Fatal("UpdateMetricsJSON() was not called")
	}
	if len(service.updateJSONBatch) != 2 {
		t.Fatalf("UpdateMetricsJSON() len = %d, want 2", len(service.updateJSONBatch))
	}
	if service.updateJSONBatch[0].ID != "Alloc" {
		t.Fatalf("UpdateMetricsJSON()[0].ID = %q, want Alloc", service.updateJSONBatch[0].ID)
	}
	if service.updateJSONBatch[1].ID != "PollCount" {
		t.Fatalf("UpdateMetricsJSON()[1].ID = %q, want PollCount", service.updateJSONBatch[1].ID)
	}
}

func TestMetricsHandlerUpdateMetricsJSONRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		serviceErr error
		wantCalled bool
	}{
		{
			name: "invalid json",
			body: "{",
		},
		{
			name:       "service error",
			body:       `[{"id":"Alloc","type":"gauge"}]`,
			serviceErr: service.ErrInvalidMetricValue,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeService{updateJSONBatchErr: tt.serviceErr}
			handler, err := NewMetricsHandler(service)
			if err != nil {
				t.Fatalf("NewMetricsHandler() error = %v", err)
			}

			response := executeRequestWithBody(newTestRouter(handler), http.MethodPost, "/updates/", tt.body)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			if service.updateJSONBatchCalled != tt.wantCalled {
				t.Fatalf("UpdateMetricsJSON() called = %v, want %v", service.updateJSONBatchCalled, tt.wantCalled)
			}
		})
	}
}

func TestMetricsHandlerGetMetricJSONSuccess(t *testing.T) {
	value := 1744184459.0
	service := &fakeService{
		getJSONResult: models.Metric{
			ID:    "LastGC",
			MType: models.MetricTypeGauge,
			Value: &value,
		},
	}
	handler, err := NewMetricsHandler(service)
	if err != nil {
		t.Fatalf("NewMetricsHandler() error = %v", err)
	}

	response := executeRequestWithBody(newTestRouter(handler), http.MethodPost, "/value/", `{"id":"LastGC","type":"gauge"}`)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want %q", contentType, "application/json")
	}
	if !service.getJSONCalled {
		t.Fatal("GetMetricJSON() was not called")
	}
	if service.getJSONMetric.ID != "LastGC" {
		t.Fatalf("GetMetricJSON() ID = %q, want LastGC", service.getJSONMetric.ID)
	}
	if service.getJSONMetric.MType != models.MetricTypeGauge {
		t.Fatalf("GetMetricJSON() MType = %q, want %q", service.getJSONMetric.MType, models.MetricTypeGauge)
	}

	var metric models.Metric
	if err := json.Unmarshal(response.Body.Bytes(), &metric); err != nil {
		t.Fatalf("decode response body: %v", err)
	}
	if metric.ID != "LastGC" {
		t.Fatalf("response ID = %q, want LastGC", metric.ID)
	}
	if metric.MType != models.MetricTypeGauge {
		t.Fatalf("response MType = %q, want %q", metric.MType, models.MetricTypeGauge)
	}
	if metric.Value == nil {
		t.Fatal("response Value = nil, want 1744184459")
	}
	if *metric.Value != 1744184459 {
		t.Fatalf("response Value = %v, want 1744184459", *metric.Value)
	}
}

func TestMetricsHandlerGetMetricJSONRejectsInvalidRequests(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		serviceErr error
		wantStatus int
		wantCalled bool
	}{
		{
			name:       "invalid json",
			body:       "{",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "not found",
			body:       `{"id":"Unknown","type":"gauge"}`,
			serviceErr: service.ErrMetricNotFound,
			wantStatus: http.StatusNotFound,
			wantCalled: true,
		},
		{
			name:       "invalid metric",
			body:       `{"id":"LastGC","type":"unknown"}`,
			serviceErr: service.ErrUnknownMetricType,
			wantStatus: http.StatusBadRequest,
			wantCalled: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &fakeService{getJSONErr: tt.serviceErr}
			handler, err := NewMetricsHandler(service)
			if err != nil {
				t.Fatalf("NewMetricsHandler() error = %v", err)
			}

			response := executeRequestWithBody(newTestRouter(handler), http.MethodPost, "/value/", tt.body)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, tt.wantStatus)
			}
			if service.getJSONCalled != tt.wantCalled {
				t.Fatalf("GetMetricJSON() called = %v, want %v", service.getJSONCalled, tt.wantCalled)
			}
		})
	}
}

func newTestRouter(handler *MetricsHandler) http.Handler {
	router := chi.NewRouter()
	router.Post("/update/{metricType}/{metricName}/{rawValue}", handler.UpdateMetric)
	router.Get("/value/{metricType}/{metricName}", handler.GetMetricValue)
	router.Get("/", handler.ListMetrics)
	router.Post("/update/", handler.UpdateMetricJSON)
	router.Post("/updates/", handler.UpdateMetricsJSON)
	router.Post("/value/", handler.GetMetricJSON)

	return router
}

func executeRequest(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, http.NoBody)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	return response
}

func executeRequestWithBody(handler http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	return response
}

type fakeService struct {
	called                bool
	metricType            string
	metricName            string
	value                 string
	err                   error
	getMetricType         string
	getMetricName         string
	getValue              string
	getErr                error
	metrics               []models.Metric
	updateJSONCalled      bool
	updateJSONMetric      models.Metric
	updateJSONErr         error
	updateJSONBatchCalled bool
	updateJSONBatch       []models.Metric
	updateJSONBatchErr    error
	getJSONCalled         bool
	getJSONMetric         models.Metric
	getJSONResult         models.Metric
	getJSONErr            error
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

func (s *fakeService) UpdateMetricJSON(metric *models.Metric) error {
	s.updateJSONCalled = true
	if metric != nil {
		s.updateJSONMetric = *metric
	}

	return s.updateJSONErr
}

func (s *fakeService) UpdateMetricsJSON(metrics []models.Metric) error {
	s.updateJSONBatchCalled = true
	s.updateJSONBatch = append([]models.Metric(nil), metrics...)

	return s.updateJSONBatchErr
}

func (s *fakeService) GetMetricJSON(metric *models.Metric) (models.Metric, error) {
	s.getJSONCalled = true
	if metric != nil {
		s.getJSONMetric = *metric
	}

	return s.getJSONResult, s.getJSONErr
}
