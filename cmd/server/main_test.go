package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewServerHandlerAcceptsMetricUpdate(t *testing.T) {
	handler, err := newServerHandler()
	if err != nil {
		t.Fatalf("newServerHandler() error = %v", err)
	}

	request := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/12.5", http.NoBody)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestNewServerHandlerReturnsMetricValue(t *testing.T) {
	handler, err := newServerHandler()
	if err != nil {
		t.Fatalf("newServerHandler() error = %v", err)
	}

	executeRequest(handler, http.MethodPost, "/update/gauge/Alloc/12.5")
	response := executeRequest(handler, http.MethodGet, "/value/gauge/Alloc")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); body != "12.5" {
		t.Fatalf("body = %q, want 12.5", body)
	}
}

func TestNewServerHandlerReturnsMetricsList(t *testing.T) {
	handler, err := newServerHandler()
	if err != nil {
		t.Fatalf("newServerHandler() error = %v", err)
	}

	executeRequest(handler, http.MethodPost, "/update/counter/PollCount/3")
	response := executeRequest(handler, http.MethodGet, "/")

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); body == "" {
		t.Fatal("body is empty, want HTML page")
	}
}

func TestNewServerHandlerReturnsNotFoundForUnknownMetric(t *testing.T) {
	handler, err := newServerHandler()
	if err != nil {
		t.Fatalf("newServerHandler() error = %v", err)
	}

	response := executeRequest(handler, http.MethodGet, "/value/gauge/Unknown")

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNotFound)
	}
}

func executeRequest(handler http.Handler, method, path string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, http.NoBody)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	return response
}
