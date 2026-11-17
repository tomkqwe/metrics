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
			h := WithDecryption(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) }))
			req := httptest.NewRequest(http.MethodPost, "/updates/", strings.NewReader("body"))
			if scheme != "" {
				req.Header.Set(encryption.Header, scheme)
			}
			response := httptest.NewRecorder()
			h.ServeHTTP(response, req)
			want := http.StatusBadRequest
			if scheme == "" {
				want = http.StatusNoContent
			}
			if response.Code != want || called != (scheme == "") {
				t.Fatalf("status %d, called %v", response.Code, called)
			}
		})
	}
}
