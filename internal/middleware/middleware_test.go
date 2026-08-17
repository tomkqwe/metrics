package middleware

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tomkqwe/metrics/internal/signature"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestWithLoggingLogsRequestAndResponseData(t *testing.T) {
	core, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	handler := WithLogging(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("stored"))
	}))

	request := httptest.NewRequest(http.MethodPost, "/update/gauge/Alloc/12.5", http.NoBody)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	logs := observedLogs.All()
	if len(logs) != 1 {
		t.Fatalf("logs count = %d, want %d", len(logs), 1)
	}

	entry := logs[0]
	if entry.Level != zap.InfoLevel {
		t.Fatalf("log level = %s, want %s", entry.Level, zap.InfoLevel)
	}

	assertStringField(t, entry.Context, "uri", "/update/gauge/Alloc/12.5")
	assertStringField(t, entry.Context, "method", http.MethodPost)
	assertIntField(t, entry.Context, "status", http.StatusCreated)
	assertIntField(t, entry.Context, "size", len("stored"))
	assertFieldExists(t, entry.Context, "duration")
}

func TestWithHashSHA256ValidatesRequestAndRestoresBody(t *testing.T) {
	const key = "secret"
	requestBody := []byte(`{"id":"Alloc"}`)

	handler := WithHashSHA256(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !bytes.Equal(body, requestBody) {
			t.Fatalf("body = %q, want %q", body, requestBody)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(requestBody))
	request.Header.Set(signature.Header, signature.Calculate(requestBody, key))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
}

func TestWithHashSHA256AllowsRequestWithoutHashHeader(t *testing.T) {
	const key = "secret"
	requestBody := []byte(`{"id":"Alloc"}`)
	called := false

	handler := WithHashSHA256(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if !bytes.Equal(body, requestBody) {
			t.Fatalf("body = %q, want %q", body, requestBody)
		}

		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(requestBody))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if !called {
		t.Fatal("handler was not called")
	}
}

func TestWithHashSHA256RejectsInvalidRequestHash(t *testing.T) {
	const key = "secret"
	called := false
	requestBody := []byte(`{"id":"Alloc"}`)

	handler := WithHashSHA256(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	request := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(requestBody))
	request.Header.Set(signature.Header, "bad")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if called {
		t.Fatal("handler was called for invalid hash")
	}
	if got, want := response.Header().Get(signature.Header), signature.Calculate(nil, key); got != want {
		t.Fatalf("%s = %q, want %q", signature.Header, got, want)
	}
}

func TestWithHashSHA256SignsResponseBody(t *testing.T) {
	const key = "secret"

	handler := WithHashSHA256(key)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok")
	}))

	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	request.Header.Set(signature.Header, signature.Calculate(nil, key))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if body := response.Body.String(); body != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
	if got, want := response.Header().Get(signature.Header), signature.Calculate(response.Body.Bytes(), key); got != want {
		t.Fatalf("%s = %q, want %q", signature.Header, got, want)
	}
}

func TestWithHashSHA256UsesWireBytesWithGzip(t *testing.T) {
	const key = "secret"
	requestBody := `{"id":"Alloc"}`
	responseBody := `{"status":"ok"}`
	compressedRequest := compressedBytes(t, requestBody)

	handler := WithHashSHA256(key)(WithGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(body) != requestBody {
			t.Fatalf("body = %q, want %q", body, requestBody)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, responseBody)
	})))

	request := httptest.NewRequest(http.MethodPost, "/update", bytes.NewReader(compressedRequest))
	request.Header.Set("Content-Encoding", "gzip")
	request.Header.Set("Accept-Encoding", "gzip")
	request.Header.Set(signature.Header, signature.Calculate(compressedRequest, key))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentEncoding := response.Header().Get("Content-Encoding"); contentEncoding != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", contentEncoding)
	}
	if got, want := response.Header().Get(signature.Header), signature.Calculate(response.Body.Bytes(), key); got != want {
		t.Fatalf("%s = %q, want %q", signature.Header, got, want)
	}
	if body := decompressedString(t, response.Body); body != responseBody {
		t.Fatalf("body = %q, want %q", body, responseBody)
	}
}

func TestWithGzipDecompressesRequestBody(t *testing.T) {
	handler := WithGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		if string(body) != `{"id":"Alloc"}` {
			t.Fatalf("body = %q, want %q", body, `{"id":"Alloc"}`)
		}

		w.WriteHeader(http.StatusOK)
	}))

	request := httptest.NewRequest(http.MethodPost, "/update", compressedString(t, `{"id":"Alloc"}`))
	request.Header.Set("Content-Encoding", "gzip")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
}

func TestWithGzipRejectsInvalidCompressedRequestBody(t *testing.T) {
	handler := WithGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler was called for invalid gzip body")
	}))

	request := httptest.NewRequest(http.MethodPost, "/update", strings.NewReader("not gzip"))
	request.Header.Set("Content-Encoding", "gzip")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestWithGzipCompressesSupportedResponseContentTypes(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
	}{
		{
			name:        "json",
			contentType: "application/json",
			body:        `{"status":"ok"}`,
		},
		{
			name:        "html",
			contentType: "text/html; charset=utf-8",
			body:        "<html><body>ok</body></html>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := WithGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", tt.contentType)
				_, _ = io.WriteString(w, tt.body)
			}))

			request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			request.Header.Set("Accept-Encoding", "gzip")
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
			}
			if contentEncoding := response.Header().Get("Content-Encoding"); contentEncoding != "gzip" {
				t.Fatalf("Content-Encoding = %q, want %q", contentEncoding, "gzip")
			}
			if body := decompressedString(t, response.Body); body != tt.body {
				t.Fatalf("body = %q, want %q", body, tt.body)
			}
		})
	}
}

func TestWithGzipDoesNotCompressUnsupportedResponseContentType(t *testing.T) {
	handler := WithGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = io.WriteString(w, "ok")
	}))

	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if contentEncoding := response.Header().Get("Content-Encoding"); contentEncoding != "" {
		t.Fatalf("Content-Encoding = %q, want empty", contentEncoding)
	}
	if body := response.Body.String(); body != "ok" {
		t.Fatalf("body = %q, want ok", body)
	}
}

func TestWithGzipFlushesCompressedResponse(t *testing.T) {
	handler := WithGzip(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ResponseWriter does not implement http.Flusher")
		}

		_, _ = io.WriteString(w, "first")
		flusher.Flush()
		_, _ = io.WriteString(w, "second")
	}))

	request := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	request.Header.Set("Accept-Encoding", "gzip")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if !response.Flushed {
		t.Fatal("underlying ResponseWriter was not flushed")
	}
	if body := decompressedString(t, response.Body); body != "firstsecond" {
		t.Fatalf("body = %q, want firstsecond", body)
	}
}

func TestGzipResponseWriterHijackDelegatesToWrappedWriter(t *testing.T) {
	response := &hijackableResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
	}
	writer := &gzipResponseWriter{
		ResponseWriter: response,
	}

	_, _, err := writer.Hijack()
	if err != nil {
		t.Fatalf("Hijack() error = %v", err)
	}
	if !response.hijacked {
		t.Fatal("wrapped ResponseWriter was not hijacked")
	}
}

func assertStringField(t *testing.T, fields []zapcore.Field, key, want string) {
	t.Helper()

	field, ok := fieldByKey(fields, key)
	if !ok {
		t.Fatalf("field %q not found", key)
	}
	if field.String != want {
		t.Fatalf("field %q = %q, want %q", key, field.String, want)
	}
}

func assertIntField(t *testing.T, fields []zapcore.Field, key string, want int) {
	t.Helper()

	field, ok := fieldByKey(fields, key)
	if !ok {
		t.Fatalf("field %q not found", key)
	}
	if field.Integer != int64(want) {
		t.Fatalf("field %q = %d, want %d", key, field.Integer, want)
	}
}

func assertFieldExists(t *testing.T, fields []zapcore.Field, key string) {
	t.Helper()

	if _, ok := fieldByKey(fields, key); !ok {
		t.Fatalf("field %q not found", key)
	}
}

func fieldByKey(fields []zapcore.Field, key string) (zapcore.Field, bool) {
	for _, field := range fields {
		if field.Key == key {
			return field, true
		}
	}

	return zapcore.Field{}, false
}

func compressedString(t *testing.T, value string) io.Reader {
	t.Helper()

	return bytes.NewReader(compressedBytes(t, value))
}

func compressedBytes(t *testing.T, value string) []byte {
	t.Helper()

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := io.WriteString(writer, value)
	if err != nil {
		t.Fatalf("write gzip body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buffer.Bytes()
}

func decompressedString(t *testing.T, body io.Reader) string {
	t.Helper()

	reader, err := gzip.NewReader(body)
	if err != nil {
		t.Fatalf("create gzip reader: %v", err)
	}
	defer func() {
		_ = reader.Close()
	}()

	value, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read gzip body: %v", err)
	}

	return string(value)
}

type hijackableResponseWriter struct {
	http.ResponseWriter
	hijacked bool
}

func (w *hijackableResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	w.hijacked = true
	return nil, nil, nil
}
