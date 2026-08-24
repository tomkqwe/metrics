package middleware

import (
	"bytes"
	"io"
	"net/http"

	"github.com/tomkqwe/metrics/internal/signature"
)

type hashResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newHashResponseWriter() *hashResponseWriter {
	return &hashResponseWriter{
		header: make(http.Header),
	}
}

func (w *hashResponseWriter) Header() http.Header {
	return w.header
}

func (w *hashResponseWriter) WriteHeader(status int) {
	if w.status != 0 {
		return
	}

	w.status = status
}

func (w *hashResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}

	return w.body.Write(data)
}

func WithHashSHA256(key string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if key == "" {
			return next
		}

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestBody, err := readRequestBody(r)
			requestHash := r.Header.Get(signature.Header)
			if err != nil || (requestHash != "" && !signature.Verify(requestBody, key, requestHash)) {
				writeSignedResponse(w, http.StatusBadRequest, nil, key)
				return
			}

			responseData := newHashResponseWriter()
			next.ServeHTTP(responseData, r)

			writeSignedCapturedResponse(w, responseData, key)
		})
	}
}

func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}

	bodyReader := r.Body
	defer func() {
		_ = bodyReader.Close()
	}()

	body, err := io.ReadAll(bodyReader)
	if err != nil {
		return nil, err
	}

	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))

	return body, nil
}

func writeSignedCapturedResponse(w http.ResponseWriter, responseData *hashResponseWriter, key string) {
	copyHeader(w.Header(), responseData.Header())
	writeSignedResponse(w, responseData.status, responseData.body.Bytes(), key)
}

func writeSignedResponse(w http.ResponseWriter, status int, body []byte, key string) {
	if status == 0 {
		status = http.StatusOK
	}

	w.Header().Set(signature.Header, signature.Calculate(body, key))
	w.WriteHeader(status)
	if len(body) > 0 {
		_, _ = w.Write(body)
	}
}

func copyHeader(dst, src http.Header) {
	for key, values := range src {
		dst.Del(key)
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
