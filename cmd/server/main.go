package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/repository"
	"github.com/tomkqwe/metrics/internal/service"
)

func main() {
	handler, err := newServerHandler()
	if err != nil {
		panic(err)
	}
	if err := http.ListenAndServe(":8080", handler); err != nil {
		panic(err)
	}
}

func newServerHandler() (http.Handler, error) {
	router := chi.NewRouter()
	storage := repository.NewMemStorage()
	srv, err := service.NewMetricService(storage)
	if err != nil {
		return nil, err
	}
	metricsHandler, err := handler.NewMetricsHandler(srv)
	if err != nil {
		return nil, err
	}
	router.Post("/update/{metricType}/{metricName}/{rawValue}", metricsHandler.UpdateMetric)
	router.Get("/value/{metricType}/{metricName}", metricsHandler.GetMetricValue)
	router.Get("/", metricsHandler.ListMetrics)

	return router, nil
}
