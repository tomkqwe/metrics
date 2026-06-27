package handler

import (
	"errors"
	"fmt"
	"github.com/tomkqwe/metrics/internal/service"
	"net/http"
	"strings"
)

var (
	ErrServiceInvalid = errors.New("service is invalid")
)

type MetricsHandler struct {
	service service.Service
}

func NewMetricsHandler(srv service.Service) (*MetricsHandler, error) {
	if srv == nil || srv == service.Service(nil) {
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
	path := strings.TrimPrefix(r.URL.Path, "/update/")
	parts := strings.Split(path, "/")
	if len(parts) != 3 {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	metricType := parts[0]
	metricName := parts[1]
	rawValue := parts[2]

	if metricName == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	err := m.service.UpdateMetric(metricType, metricName, rawValue)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%s %s = %s", metricType, metricName, rawValue)
}
