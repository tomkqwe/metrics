package gziputil

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestIndependentStreams(t *testing.T) {
	// Concurrent requests must never share active compressors or destinations.
	for i := 0; i < 16; i++ {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			t.Parallel()
			for j := 0; j < 8; j++ {
				want := strings.Repeat(fmt.Sprintf("stream-%d-%d;", i, j), 100)
				var dst bytes.Buffer
				writer := AcquireWriter(&dst)
				if _, err := io.WriteString(writer, want); err != nil {
					t.Fatal(err)
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				ReleaseWriter(writer)
				reader, err := gzip.NewReader(bytes.NewReader(dst.Bytes()))
				if err != nil {
					t.Fatal(err)
				}
				got, err := io.ReadAll(reader)
				if err != nil {
					t.Fatal(err)
				}
				if err := reader.Close(); err != nil {
					t.Fatal(err)
				}
				if string(got) != want {
					t.Fatal("stream corrupted after writer reuse")
				}
			}
		})
	}
}

type failingWriter struct{}

var errWrite = errors.New("write failed")

func (failingWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestWriterResetAfterFailure(t *testing.T) {
	writer := AcquireWriter(failingWriter{})
	if _, err := writer.Write([]byte("first")); !errors.Is(err, errWrite) {
		t.Fatalf("error=%v", err)
	}
	if err := writer.Close(); !errors.Is(err, errWrite) {
		t.Fatalf("close error=%v", err)
	}
	ReleaseWriter(writer)
	var dst bytes.Buffer
	writer = AcquireWriter(&dst)
	if _, err := writer.Write([]byte("second")); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	ReleaseWriter(writer)
	reader, err := gzip.NewReader(&dst)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	body, err := io.ReadAll(reader)
	if err != nil || string(body) != "second" {
		t.Fatalf("body=%q error=%v", body, err)
	}
}
