package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tomkqwe/metrics/internal/agent/sender"
	"github.com/tomkqwe/metrics/internal/encryption"
	models "github.com/tomkqwe/metrics/internal/model"
	"github.com/tomkqwe/metrics/internal/repository/memstorage"
	"github.com/tomkqwe/metrics/internal/service"
	"go.uber.org/zap"
)

func TestEncryptedAgentServer(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	storage := memstorage.NewMemStorage()
	svc, err := service.NewMetricService(storage)
	if err != nil {
		t.Fatal(err)
	}
	h, err := newServerHandlerWithCrypto(zap.NewNop(), svc, nil, "shared", key)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()
	var wire []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wire, _ = io.ReadAll(r.Body)
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(wire))
		if r.Header.Get(encryption.Header) != encryption.Scheme {
			t.Error("missing encryption header")
		}
		h.ServeHTTP(w, r)
	}))
	defer server.Close()
	value := 42.5
	s := sender.NewHTTPSender(server.URL, sender.WithPublicKey(&key.PublicKey), sender.WithKey("shared"))
	if err := s.Send(context.Background(), []models.Metric{{ID: "secret-metric", MType: models.MetricTypeGauge, Value: &value}}); err != nil {
		t.Fatal(err)
	}
	metrics, err := storage.Snapshot(context.Background())
	if err != nil || len(metrics) != 1 || metrics[0].Value == nil || *metrics[0].Value != value {
		t.Fatalf("stored metrics: %v, %v", metrics, err)
	}
	if bytes.Contains(wire, []byte("secret-metric")) {
		t.Fatal("plaintext on wire")
	}
	for _, body := range [][]byte{[]byte("garbage"), wire[:len(wire)-1]} {
		req := httptest.NewRequest(http.MethodPost, "/updates/", bytes.NewReader(body))
		req.Header.Set(encryption.Header, encryption.Scheme)
		response := httptest.NewRecorder()
		h.ServeHTTP(response, req)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("status = %d", response.Code)
		}
	}
}

func TestRunRejectsMissingCryptoKey(t *testing.T) {
	if err := run(context.Background(), config{CryptoKey: t.TempDir() + "/missing.pem"}, zap.NewNop()); err == nil {
		t.Fatal("expected missing crypto key error")
	}
}
