package audit

import (
	"context"
	"encoding/json"
	"errors"
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
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode audit event: %w", err)
	}
	data = append(data, '\n')
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, err := f.file.Write(data); err != nil {
		return fmt.Errorf("write audit file: %w", err)
	}
	return nil
}

// Close syncs and closes the audit file after any current write finishes.
// Drain the publisher first so queued events are written before closing.
func (f *FileObserver) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return errors.Join(f.file.Sync(), f.file.Close())
}
