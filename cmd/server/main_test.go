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
