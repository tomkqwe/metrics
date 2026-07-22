package middleware

import (
	"net/http"
	"net/http/httptest"
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
