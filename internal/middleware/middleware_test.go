package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

	var buffer bytes.Buffer
	writer := gzip.NewWriter(&buffer)
	_, err := io.WriteString(writer, value)
	if err != nil {
		t.Fatalf("write gzip body: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return &buffer
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
