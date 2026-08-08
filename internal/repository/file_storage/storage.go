package file_storage

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

var ErrInvalidFileStoragePath = errors.New("file storage path is empty")

type FileStorage struct {
	mu   sync.Mutex
	path string
}

func NewFileStorage(path string) *FileStorage {
	return &FileStorage{
		path: path,
	}
}

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
