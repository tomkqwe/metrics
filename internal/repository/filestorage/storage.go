// Package filestorage persists metric snapshots as JSON files.
package filestorage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	models "github.com/tomkqwe/metrics/internal/model"
)

// ErrInvalidFileStoragePath indicates an empty snapshot file path.
var ErrInvalidFileStoragePath = errors.New("file storage path is empty")

// FileStorage saves and loads JSON snapshots, serializing access within one instance.
type FileStorage struct {
	mu   sync.Mutex
	path string
}

// NewFileStorage creates a snapshot store for path without opening the file.
func NewFileStorage(path string) *FileStorage {
	return &FileStorage{
		path: path,
	}
}

// Save writes a JSON snapshot to a temporary file and renames it over the destination.
// Parent directories are created as needed; a nil snapshot is encoded as an empty array.
func (s *FileStorage) Save(metrics []models.Metric) error {
	if s.path == "" {
		return ErrInvalidFileStoragePath
	}
	if metrics == nil {
		metrics = make([]models.Metric, 0)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create metrics storage dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, filepath.Base(s.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create metrics storage temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	encoder := json.NewEncoder(tmpFile)
	if err = encoder.Encode(metrics); err != nil {
		_ = tmpFile.Close()
		return fmt.Errorf("encode metrics storage file: %w", err)
	}
	if err = tmpFile.Close(); err != nil {
		return fmt.Errorf("close metrics storage file: %w", err)
	}
	if err = os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("replace metrics storage file: %w", err)
	}

	removeTemp = false
	return nil
}

// Load reads a JSON snapshot. A missing or empty file returns a nil slice without an error.
func (s *FileStorage) Load() ([]models.Metric, error) {
	if s.path == "" {
		return nil, ErrInvalidFileStoragePath
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.Open(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open metrics storage file: %w", err)
	}
	defer func() {
		_ = file.Close()
	}()

	var metrics []models.Metric
	if err = json.NewDecoder(file).Decode(&metrics); errors.Is(err, io.EOF) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("decode metrics storage file: %w", err)
	}

	return metrics, nil
}
