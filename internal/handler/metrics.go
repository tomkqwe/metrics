// Package handler provides HTTP endpoints for metric updates, queries and database health.
package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/tomkqwe/metrics/internal/audit"
	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/service"
	"go.uber.org/zap"
)

var (
	// ErrServiceInvalid indicates that no metric service was provided.
	ErrServiceInvalid = errors.New("service is invalid")
)

// MetricsHandler serves metric update, lookup and listing endpoints using a Service.
type MetricsHandler struct {
	service service.Service
	audit   *audit.Publisher
	logger  *zap.Logger
}

type metricView struct {
	Type  string
	Name  string
	Value string
}

var metricsListTemplate = template.Must(template.New("metrics").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Metrics</title>
</head>
<body>
<h1>Metrics</h1>
<table>
<thead>
<tr><th>Type</th><th>Name</th><th>Value</th></tr>
</thead>
<tbody>
{{range .}}
<tr><td>{{.Type}}</td><td>{{.Name}}</td><td>{{.Value}}</td></tr>
{{else}}
<tr><td colspan="3">No metrics</td></tr>
{{end}}
</tbody>
</table>
</body>
</html>`))

// NewMetricsHandler creates handlers with structured logging and asynchronous audit.
// A nil service returns ErrServiceInvalid; a nil logger disables logging.
// The caller must call Close after all requests finish to drain audit deliveries.
func NewMetricsHandler(srv service.Service, logger *zap.Logger, observers ...audit.Observer) (*MetricsHandler, error) {
	if srv == nil {
		return nil, ErrServiceInvalid
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &MetricsHandler{
		service: srv,
		logger:  logger,
		audit:   audit.NewPublisher(logger, observers...),
	}, nil
}

// UpdateMetric handles POST /update/{metricType}/{metricName}/{rawValue}.
// It replaces gauges, increments counters and audits successful updates.
// Success returns 200; invalid values or types return 400; missing path parameters return 404.
func (m *MetricsHandler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	metricType := chi.URLParam(r, "metricType")
	metricName := chi.URLParam(r, "metricName")
	rawValue := chi.URLParam(r, "rawValue")

	if metricType == "" || metricName == "" || rawValue == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if err := m.service.UpdateMetric(r.Context(), metricType, metricName, rawValue); err != nil {
		writeServiceError(w, err)
		return
	}

	m.auditUpdate(r, []string{metricName})

	m.logger.Info("metric updated", zap.String("type", metricType), zap.String("name", metricName), zap.String("value", rawValue))

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "%s %s = %s", metricType, metricName, rawValue)
}

// GetMetricValue handles GET /value/{metricType}/{metricName}, returning a plain-text value.
// It returns 404 for a missing metric and 400 for an unsupported type.
func (m *MetricsHandler) GetMetricValue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	metricType := chi.URLParam(r, "metricType")
	metricName := chi.URLParam(r, "metricName")

	if metricType == "" || metricName == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	value, err := m.service.GetMetricValue(r.Context(), metricType, metricName)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, value)
}

// ListMetrics handles GET / and renders stored metrics as an HTML table.
func (m *MetricsHandler) ListMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	metrics, err := m.service.ListMetrics(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = metricsListTemplate.Execute(w, metricsForView(metrics)); err != nil {
		m.logger.Error("render metrics list", zap.Error(err))
	}
}

// UpdateMetricJSON handles POST /update/ with a JSON metric and returns its current stored value.
// Invalid input returns 400; a successful update emits an audit event.
func (m *MetricsHandler) UpdateMetricJSON(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	w.Header().Set("Content-Type", "application/json")
	var reqBody models.Metric
	if err := decoder.Decode(&reqBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := m.service.UpdateMetricJSON(r.Context(), &reqBody); err != nil {
		writeServiceError(w, err)
		return
	}

	m.auditUpdate(r, []string{reqBody.ID})

	metric, err := m.service.GetMetricJSON(r.Context(), &reqBody)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if err = json.NewEncoder(w).Encode(metric); err != nil {
		m.logger.Error("encode response", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// UpdateMetricsJSON handles POST /updates/ with a JSON array of metrics.
// It returns 200 with an empty body on success and emits one audit event for the batch.
// Invalid input returns 400; storage failures return 500.
func (m *MetricsHandler) UpdateMetricsJSON(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	w.Header().Set("Content-Type", "application/json")

	var reqBody []models.Metric
	if err := decoder.Decode(&reqBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := m.service.UpdateMetricsJSON(r.Context(), reqBody); err != nil {
		writeServiceError(w, err)
		return
	}

	names := make([]string, 0, len(reqBody))
	for _, metric := range reqBody {
		names = append(names, metric.ID)
	}
	m.auditUpdate(r, names)

	w.WriteHeader(http.StatusOK)
}

// GetMetricJSON handles POST /value/ with a metric ID and type, returning the stored metric as JSON.
// It returns 404 when the metric does not exist and 400 for invalid input.
func (m *MetricsHandler) GetMetricJSON(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var reqBody models.Metric
	if err := decoder.Decode(&reqBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	metric, err := m.service.GetMetricJSON(r.Context(), &reqBody)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(metric); err != nil {
		m.logger.Error("encode response", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func metricsForView(metrics []models.Metric) []metricView {
	view := make([]metricView, 0, len(metrics))
	for _, metric := range metrics {
		view = append(view, metricView{
			Type:  metric.MType,
			Name:  metric.ID,
			Value: metricValue(metric),
		})
	}

	return view
}

func metricValue(metric models.Metric) string {
	switch metric.MType {
	case models.MetricTypeGauge:
		if metric.Value == nil {
			return ""
		}
		return strconv.FormatFloat(*metric.Value, 'f', -1, 64)
	case models.MetricTypeCounter:
		if metric.Delta == nil {
			return ""
		}
		return strconv.FormatInt(*metric.Delta, 10)
	default:
		return ""
	}
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrMetricNotFound):
		w.WriteHeader(http.StatusNotFound)
	case isBadRequestError(err):
		w.WriteHeader(http.StatusBadRequest)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func isBadRequestError(err error) bool {
	if errors.Is(err, service.ErrNilMetric) ||
		errors.Is(err, service.ErrInvalidMetricName) ||
		errors.Is(err, service.ErrInvalidMetricValue) ||
		errors.Is(err, service.ErrUnknownMetricType) {
		return true
	}

	var numErr *strconv.NumError
	return errors.As(err, &numErr)
}

func (m *MetricsHandler) auditUpdate(r *http.Request, names []string) {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}
	event := audit.Event{TS: time.Now().Unix(), Metrics: names, IPAddress: ip}
	if err := m.audit.Notify(r.Context(), event); err != nil {
		m.logger.Error("queue audit event", zap.Error(err))
	}
}

// Close waits for pending audit deliveries. Call it after all HTTP handlers finish.
func (m *MetricsHandler) Close() { m.audit.Close() }
