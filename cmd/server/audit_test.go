package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/tomkqwe/metrics/internal/audit"
	"github.com/tomkqwe/metrics/internal/repository/memstorage"
	"github.com/tomkqwe/metrics/internal/service"
	"go.uber.org/zap"
)

type auditRecorder struct{ events []audit.Event }

func (a *auditRecorder) Notify(_ context.Context, e audit.Event) error {
	a.events = append(a.events, e)
	return errors.New("receiver failure")
}

func TestAuditConfiguration(t *testing.T) {
	unsetServerEnv(t)
	t.Setenv("AUDIT_FILE", "")
	t.Setenv("AUDIT_URL", "")
	cfg, err := parseConfig(nil)
	if err != nil || cfg.AuditFile != "" || cfg.AuditURL != "" {
		t.Fatalf("defaults=%+v, %v", cfg, err)
	}
	args := []string{"--audit-file", "events.jsonl", "--audit-url", "http://localhost/audit"}
	cfg, err = parseConfig(args)
	if err != nil || cfg.AuditFile != "events.jsonl" || cfg.AuditURL != "http://localhost/audit" {
		t.Fatalf("flags=%+v, %v", cfg, err)
	}
	t.Setenv("AUDIT_FILE", "env.jsonl")
	t.Setenv("AUDIT_URL", "http://example.org/audit")
	cfg, err = parseConfig(args)
	if err != nil || cfg.AuditFile != "env.jsonl" || cfg.AuditURL != "http://example.org/audit" {
		t.Fatalf("env=%+v, %v", cfg, err)
	}
}

func TestServerAuditsSuccessfulUpdates(t *testing.T) {
	tests := []struct {
		path, body, ip string
		status         int
		names          []string
	}{
		{"/update/gauge/Alloc/12.5", "", "192.168.0.42:5678", 200, []string{"Alloc"}},
		{"/update/", `{"id":"Alloc","type":"gauge","value":12.5}`, "[::1]:5678", 200, []string{"Alloc"}},
		{"/updates/", `[{"id":"Alloc","type":"gauge","value":12.5},{"id":"Frees","type":"counter","delta":1}]`, "192.168.0.42", 200, []string{"Alloc", "Frees"}},
		{"/update/gauge/Alloc/bad", "", "127.0.0.1:5678", 400, nil},
		{"/update/", `{"id":"Alloc","type":"gauge"}`, "127.0.0.1:5678", 400, nil},
		{"/updates/", `[{"id":"Alloc","type":"gauge","value":12.5},{"id":"Frees","type":"bad"}]`, "127.0.0.1:5678", 400, nil},
		{"/updates/", `invalid`, "127.0.0.1:5678", 400, nil},
		{"/value/", `{"id":"Alloc","type":"gauge"}`, "127.0.0.1:5678", 404, nil},
	}
	for _, tt := range tests {
		t.Run(tt.path+tt.body, func(t *testing.T) {
			srv, err := service.NewMetricService(memstorage.NewMemStorage())
			if err != nil {
				t.Fatal(err)
			}
			recorder := &auditRecorder{}
			delivered := make(chan audit.Event, 1)
			receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var e audit.Event
				if err := json.NewDecoder(r.Body).Decode(&e); err != nil {
					t.Error(err)
				}
				delivered <- e
				w.WriteHeader(http.StatusNoContent)
			}))
			defer receiver.Close()
			remote, err := audit.NewHTTPObserver(receiver.URL)
			if err != nil {
				t.Fatal(err)
			}
			router, err := newServerHandler(zap.NewNop(), srv, nil, "", recorder, remote)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, tt.path, strings.NewReader(tt.body))
			req.RemoteAddr = tt.ip
			req.Header.Set("X-Forwarded-For", "203.0.113.10")
			res := httptest.NewRecorder()
			before := time.Now().Unix()
			router.ServeHTTP(res, req)
			if res.Code != tt.status {
				t.Fatalf("status=%d want=%d", res.Code, tt.status)
			}
			if tt.names == nil {
				if len(recorder.events) != 0 || len(delivered) != 0 {
					t.Fatal("unexpected audit")
				}
				return
			}
			if len(recorder.events) != 1 || len(delivered) != 1 {
				t.Fatalf("events=%v delivered=%v", recorder.events, delivered)
			}
			e := recorder.events[0]
			if !reflect.DeepEqual(e.Metrics, tt.names) || e.TS < before || e.TS > time.Now().Unix() {
				t.Fatalf("event=%+v", e)
			}
			expectedIP := "192.168.0.42"
			if strings.HasPrefix(tt.ip, "[") {
				expectedIP = "::1"
			}
			if e.IPAddress != expectedIP {
				t.Fatalf("IP=%q want=%q", e.IPAddress, expectedIP)
			}
		})
	}
}
