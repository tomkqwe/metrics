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

func NewFileObserver(path string) (*FileObserver, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("open audit file: %w", err)
	}
	return &FileObserver{file: file}, nil
}

func (f *FileObserver) Notify(_ context.Context, event Event) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := json.NewEncoder(f.file).Encode(event); err != nil {
		return fmt.Errorf("write audit file: %w", err)
	}
	return nil
}

func (f *FileObserver) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.file.Close()
}
