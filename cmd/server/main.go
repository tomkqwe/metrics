package main

import (
	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/repository"
	"github.com/tomkqwe/metrics/internal/service"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	storage := repository.NewMemStorage()
	srv, err := service.NewMetricService(storage)
	if err != nil {
		panic(err)
	}
	metricsHandler, err := handler.NewMetricsHandler(srv)
	if err != nil {
		panic(err)
	}
	mux.HandleFunc("/update/", metricsHandler.UpdateMetric)
	if err := http.ListenAndServe(":8080", mux); err != nil {
		panic(err)
	}
}
