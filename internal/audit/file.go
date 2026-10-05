package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// FileObserver appends one JSON event per line without interleaving writes.
type FileObserver struct {
	mu   sync.Mutex
	file *os.File
}

// NewFileObserver opens or creates an audit file for appending JSON Lines.
// The caller must close the observer when it is no longer needed.
func NewFileObserver(path string) (*FileObserver, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open audit file: %w", err)
	}
	return &FileObserver{file: file}, nil
}

// Notify appends one event as a JSON line under a write lock.
func (f *FileObserver) Notify(_ context.Context, event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := json.NewEncoder(f.file).Encode(event); err != nil {
		return fmt.Errorf("write audit file: %w", err)
	}
	return nil
}

// Close closes the audit file, waiting for any current write to finish.
func (f *FileObserver) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Close()
}
