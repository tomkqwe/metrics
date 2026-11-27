package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tomkqwe/metrics/internal/repository/memstorage"
	"github.com/tomkqwe/metrics/internal/service"
	"go.uber.org/zap"
)

func TestTrustedSubnetRoutes(t *testing.T) {
	_, subnet, err := net.ParseCIDR("192.0.2.0/24")
	if err != nil {
		t.Fatal(err)
	}
	svc, err := service.NewMetricService(memstorage.NewMemStorage())
	if err != nil {
		t.Fatal(err)
	}
	h, err := newServerHandlerWithCrypto(zap.NewNop(), svc, nil, "", nil, subnet)
	if err != nil {
		t.Fatal(err)
	}
	defer h.Close()

	for _, path := range []string{"/update", "/update/", "/updates", "/updates/", "/update/counter/count/1"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, nil))
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s: status = %d, want 403", path, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/update/counter/count/1", nil)
	req.Header.Set("X-Real-IP", "192.0.2.7")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("trusted update: status = %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/value/counter/count", nil))
	if rec.Code != http.StatusOK || rec.Body.String() != "1" {
		t.Errorf("read: status = %d, body = %q", rec.Code, rec.Body)
	}
}

func TestRunRejectsInvalidSubnet(t *testing.T) {
	if err := run(context.Background(), config{TrustedSubnet: "192.0.2.1"}, zap.NewNop()); err == nil {
		t.Fatal("expected an error for a missing CIDR prefix length")
	}
}
