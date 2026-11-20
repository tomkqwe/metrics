package middleware

import (
	"bytes"
	"crypto/rsa"
	"io"
	"net/http"

	"github.com/tomkqwe/metrics/internal/encryption"
)

// WithDecryption opens encrypted requests before gzip decoding. Unmarked requests
// remain supported. A nil key disables the middleware entirely.
func WithDecryption(key *rsa.PrivateKey) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			scheme := r.Header.Get(encryption.Header)
			if scheme == "" {
				next.ServeHTTP(w, r)
				return
			}
			if scheme != encryption.Scheme {
				http.Error(w, "unsupported encryption", http.StatusBadRequest)
				return
			}
			body, err := readRequestBody(r)
			if err == nil {
				body, err = encryption.Decrypt(key, body)
			}
			if err != nil {
				http.Error(w, "invalid encrypted request", http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
			r.Header.Del("Content-Length")
			r.Header.Del(encryption.Header)
			next.ServeHTTP(w, r)
		})
	}
}
