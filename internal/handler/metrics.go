package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/service"
)

var (
	ErrServiceInvalid = errors.New("service is invalid")
)

type MetricsHandler struct {
	service service.Service
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

func NewMetricsHandler(srv service.Service) (*MetricsHandler, error) {
	if srv == nil {
		return nil, ErrServiceInvalid
	}
	return &MetricsHandler{
		service: srv,
	}, nil
}

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
	err := m.service.UpdateMetric(metricType, metricName, rawValue)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("metric updated: type=%s name=%s value=%s", metricType, metricName, rawValue)

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "%s %s = %s", metricType, metricName, rawValue)
}

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

	value, err := m.service.GetMetricValue(metricType, metricName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprint(w, value)
}

func (m *MetricsHandler) ListMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	err := metricsListTemplate.Execute(w, metricsForView(m.service.ListMetrics()))
	if err != nil {
		log.Printf("render metrics list: %v", err)
	}
}

func (m *MetricsHandler) UpdateMetricJSON(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	w.Header().Set("Content-Type", "application/json")
	var reqBody models.Metric
	if err := decoder.Decode(&reqBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := m.service.UpdateMetricJSON(&reqBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	metric, err := m.service.GetMetricJSON(&reqBody)
	if err != nil {
		if errors.Is(err, service.ErrMetricNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err = json.NewEncoder(w).Encode(metric); err != nil {
		log.Printf("encode response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (m *MetricsHandler) GetMetricJSON(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var reqBody models.Metric
	if err := decoder.Decode(&reqBody); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	metric, err := m.service.GetMetricJSON(&reqBody)
	if err != nil {
		if errors.Is(err, service.ErrMetricNotFound) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err = json.NewEncoder(w).Encode(metric); err != nil {
		log.Printf("encode response: %v", err)
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
