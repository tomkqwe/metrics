package main

import (
	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/repository"
	"github.com/tomkqwe/metrics/internal/service"
	"net/http"
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
	mux := http.NewServeMux()
	storage := repository.NewMemStorage()
	srv, err := service.NewMetricService(storage)
	if err != nil {
		return nil, err
	}
	metricsHandler, err := handler.NewMetricsHandler(srv)
	if err != nil {
		return nil, err
	}
	mux.HandleFunc("/update/", metricsHandler.UpdateMetric)

	return mux, nil
}
