package handler_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/tomkqwe/metrics/internal/handler"
	"github.com/tomkqwe/metrics/internal/repository/memstorage"
	"github.com/tomkqwe/metrics/internal/service"
)

// exampleRouter wires the public handlers to the same endpoint paths as the server.
// Each example has independent in-memory storage and needs no network listener.
func exampleRouter() http.Handler {
	srv, err := service.NewMetricService(memstorage.NewMemStorage())
	if err != nil {
		panic(err)
	}
	h, err := handler.NewMetricsHandler(srv)
	if err != nil {
		panic(err)
	}
	router := chi.NewRouter()
	router.Post("/update/{metricType}/{metricName}/{rawValue}", h.UpdateMetric)
	router.Get("/value/{metricType}/{metricName}", h.GetMetricValue)
	router.Post("/update/", h.UpdateMetricJSON)
	router.Post("/updates/", h.UpdateMetricsJSON)
	router.Post("/value/", h.GetMetricJSON)
	router.Get("/", h.ListMetrics)
	return router
}

func exampleRequest(router http.Handler, method, path, body string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response
}

func ExampleMetricsHandler_UpdateMetric() {
	router := exampleRouter()
	response := exampleRequest(router, http.MethodPost, "/update/gauge/Alloc/12.5", "")
	fmt.Println(response.Code)
	fmt.Println(response.Body.String())
	// Output:
	// 200
	// gauge Alloc = 12.5
}

func ExampleMetricsHandler_GetMetricValue() {
	router := exampleRouter()
	exampleRequest(router, http.MethodPost, "/update/counter/PollCount/3", "")
	exampleRequest(router, http.MethodPost, "/update/counter/PollCount/2", "")
	response := exampleRequest(router, http.MethodGet, "/value/counter/PollCount", "")
	fmt.Println(response.Code)
	fmt.Println(response.Body.String())
	// Output:
	// 200
	// 5
}

func ExampleMetricsHandler_UpdateMetricJSON() {
	router := exampleRouter()
	response := exampleRequest(router, http.MethodPost, "/update/", `{"id":"Alloc","type":"gauge","value":12.5}`)
	fmt.Println(response.Code)
	fmt.Print(response.Body.String())
	// Output:
	// 200
	// {"id":"Alloc","type":"gauge","value":12.5}
}

func ExampleMetricsHandler_UpdateMetricsJSON() {
	router := exampleRouter()
	response := exampleRequest(router, http.MethodPost, "/updates/", `[
  {"id":"Alloc","type":"gauge","value":12.5},
  {"id":"PollCount","type":"counter","delta":3}
 ]`)
	fmt.Println(response.Code)
	fmt.Println("Response bytes:", response.Body.Len())
	gauge := exampleRequest(router, http.MethodGet, "/value/gauge/Alloc", "")
	counter := exampleRequest(router, http.MethodGet, "/value/counter/PollCount", "")
	fmt.Println("Alloc:", gauge.Body.String())
	fmt.Println("PollCount:", counter.Body.String())
	// Output:
	// 200
	// Response bytes: 0
	// Alloc: 12.5
	// PollCount: 3
}

func ExampleMetricsHandler_GetMetricJSON() {
	router := exampleRouter()
	exampleRequest(router, http.MethodPost, "/update/", `{"id":"PollCount","type":"counter","delta":3}`)
	response := exampleRequest(router, http.MethodPost, "/value/", `{"id":"PollCount","type":"counter"}`)
	fmt.Println(response.Code)
	fmt.Print(response.Body.String())
	// Output:
	// 200
	// {"id":"PollCount","type":"counter","delta":3}
}

func ExampleMetricsHandler_GetMetricJSON_notFound() {
	response := exampleRequest(exampleRouter(), http.MethodPost, "/value/", `{"id":"Unknown","type":"gauge"}`)
	fmt.Println(response.Code)
	// Output: 404
}

func ExampleMetricsHandler_UpdateMetricJSON_invalidValue() {
	response := exampleRequest(exampleRouter(), http.MethodPost, "/update/", `{"id":"Alloc","type":"gauge"}`)
	fmt.Println(response.Code)
	// Output: 400
}

func ExampleMetricsHandler_ListMetrics() {
	router := exampleRouter()
	exampleRequest(router, http.MethodPost, "/update/", `{"id":"Alloc","type":"gauge","value":12.5}`)
	response := exampleRequest(router, http.MethodGet, "/", "")
	fmt.Println(response.Code)
	fmt.Println(response.Header().Get("Content-Type"))
	fmt.Println(strings.Contains(response.Body.String(), "<tr><td>gauge</td><td>Alloc</td><td>12.5</td></tr>"))
	// Output:
	// 200
	// text/html; charset=utf-8
	// true
}

// healthyDatabase substitutes only the database connectivity check.
type healthyDatabase struct{}

func (healthyDatabase) PingContext(context.Context) error { return nil }

func ExamplePingHandler_Ping() {
	h := handler.NewPingHandler(healthyDatabase{})
	router := chi.NewRouter()
	router.Get("/ping", h.Ping)
	response := exampleRequest(router, http.MethodGet, "/ping", "")
	fmt.Println(response.Code)
	// Output: 200
}
