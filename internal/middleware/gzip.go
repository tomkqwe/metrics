package middleware

import (
	"bufio"
	"compress/gzip"
	"io"
	"mime"
	"net"
	"net/http"
	"strings"
)

var _ io.Closer = (*gzipResponseWriter)(nil)
var _ http.Flusher = (*gzipResponseWriter)(nil)
var _ http.Hijacker = (*gzipResponseWriter)(nil)

const gzipEncoding = "gzip"

var gzipContentTypes = map[string]struct{}{
	"application/json": {},
	"text/html":        {},
}

type gzipResponseWriter struct {
	http.ResponseWriter
	writer      *gzip.Writer
	compressing bool
	wroteHeader bool
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}

	w.wroteHeader = true
	if w.shouldCompress() {
		w.compressing = true
		w.writer = gzip.NewWriter(w.ResponseWriter)
		w.Header().Set("Content-Encoding", gzipEncoding)
		w.Header().Add("Vary", "Accept-Encoding")
		w.Header().Del("Content-Length")
	}

	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	if w.compressing {
		if _, err := w.writer.Write(data); err != nil {
			return 0, err
		}
		return len(data), nil
	}

	return w.ResponseWriter.Write(data)
}

func (w *gzipResponseWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	if w.compressing && w.writer != nil {
		_ = w.writer.Flush()
	}

	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *gzipResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, http.ErrNotSupported
	}

	return hijacker.Hijack()
}

func (w *gzipResponseWriter) Close() error {
	if w.writer == nil {
		return nil
	}

	return w.writer.Close()
}

func (w *gzipResponseWriter) shouldCompress() bool {
	if w.Header().Get("Content-Encoding") != "" {
		return false
	}

	contentType := w.Header().Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = contentType
	}
	_, ok := gzipContentTypes[mediaType]
	return ok
}

func WithGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hasEncoding(r.Header.Get("Content-Encoding"), gzipEncoding) {
			reader, err := gzip.NewReader(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			defer func() {
				_ = reader.Close()
			}()
			r.Body = reader
		}

		if !hasEncoding(r.Header.Get("Accept-Encoding"), gzipEncoding) {
			next.ServeHTTP(w, r)
			return
		}

		responseData := &gzipResponseWriter{
			ResponseWriter: w,
		}
		defer func() {
			_ = responseData.Close()
		}()

		next.ServeHTTP(responseData, r)
	})
}

func hasEncoding(headerValue, encoding string) bool {
	for _, value := range strings.Split(headerValue, ",") {
		value = strings.TrimSpace(value)
		value = strings.ToLower(strings.Split(value, ";")[0])
		if value == encoding {
			return true
		}
	}

	return false
}
