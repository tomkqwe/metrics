package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tomkqwe/metrics/internal/encryption"
)

func TestDecryptionWithoutKey(t *testing.T) {
	for _, scheme := range []string{"", encryption.Scheme, "unknown"} {
		t.Run(scheme, func(t *testing.T) {
			called := false
			next := http.NewServeMux()
			next.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) })
			h := WithDecryption(nil)(next)
			if h != next {
				t.Fatal("middleware without key must return the original handler")
			}
			req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader("body"))
			if scheme != "" {
				req.Header.Set(encryption.Header, scheme)
			}
			response := httptest.NewRecorder()
			h.ServeHTTP(response, req)
			if response.Code != http.StatusNoContent || !called {
				t.Fatalf("status %d, called %v", response.Code, called)
			}
		})
	}
}
