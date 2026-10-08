// Package gziputil reuses gzip compression state across independent streams.
package gziputil

import (
	"compress/gzip"
	"io"
	"sync"
)

var writers = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

// AcquireWriter returns a writer exclusively owned by the caller until ReleaseWriter.
func AcquireWriter(dst io.Writer) *gzip.Writer {
	writer := writers.Get().(*gzip.Writer)
	writer.Reset(dst)
	return writer
}

// ReleaseWriter returns compression buffers to the pool. Call Close first to
// finish the stream. Reset detaches the previous destination, so the pool does
// not retain an HTTP response or an encoded request body. The writer must not
// be used after release.
func ReleaseWriter(writer *gzip.Writer) {
	writer.Reset(io.Discard)
	writers.Put(writer)
}
