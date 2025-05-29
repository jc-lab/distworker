package storage

import (
	"context"
	"fmt"
	storage2 "github.com/jc-lab/distworker/pkg/controller/storage"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const TypeLocal = "local"

type LocalStorage struct {
	basePath string
	maxSize  int64 // Maximum file size in bytes (0 = no limit)
}

// LocalConfig implements StorageConfig for local storage
type LocalConfig struct {
	Path    string `yaml:"path"`
	MaxSize int64  `yaml:"max_size,omitempty"` // in bytes
}

// Validate validates the local storage configuration
func (c *LocalConfig) Validate() error {
	if c.Path == "" {
		return fmt.Errorf("local storage base path is required")
	}

	// Ensure base path is absolute
	if !filepath.IsAbs(c.Path) {
		var err error
		c.Path, err = filepath.Abs(c.Path)
		if err != nil {
			return err
		}
	}

	return nil
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(cfg *LocalConfig) (*LocalStorage, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Create base directory if it doesn't exist
	if err := os.MkdirAll(cfg.Path, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base directory: %w", err)
	}

	storage := &LocalStorage{
		basePath: cfg.Path,
		maxSize:  cfg.MaxSize,
	}

	// Test write permissions
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := storage.Health(ctx); err != nil {
		return nil, fmt.Errorf("local storage health check failed: %w", err)
	}

	return storage, nil
}

// Upload uploads a file to local storage
func (l *LocalStorage) Upload(ctx context.Context, filename string, data io.Reader, contentType string) (*storage2.FileInfo, error) {
	// Generate unique file ID
	fileId := uuid.New().String()

	// Create directory structure: basePath/fileId/
	dirPath := filepath.Join(l.basePath, fileId)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Sanitize filename
	safeFilename := l.sanitizeFilename(filename)
	filePath := filepath.Join(dirPath, safeFilename)

	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data with size limit check
	var size int64
	if l.maxSize > 0 {
		limitedReader := io.LimitReader(data, l.maxSize+1) // +1 to detect if exceeded
		written, err := io.Copy(file, limitedReader)
		if err != nil {
			os.Remove(filePath) // Clean up on error
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
		if written > l.maxSize {
			os.Remove(filePath) // Clean up oversized file
			return nil, fmt.Errorf("file size exceeds maximum allowed size of %d bytes", l.maxSize)
		}
		size = written
	} else {
		written, err := io.Copy(file, data)
		if err != nil {
			os.Remove(filePath) // Clean up on error
			return nil, fmt.Errorf("failed to write file: %w", err)
		}
		size = written
	}

	// Create metadata file
	if err := l.writeMetadata(dirPath, filename, contentType, size); err != nil {
		os.RemoveAll(dirPath) // Clean up on error
		return nil, fmt.Errorf("failed to write metadata: %w", err)
	}

	// Build storage URL
	storageURL := fmt.Sprintf("files/%s/%s", fileId, safeFilename)

	fileInfo := &storage2.FileInfo{
		FileId:      fileId,
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		StorageURL:  storageURL,
		CreatedAt:   time.Now().Unix(),
	}

	return fileInfo, nil
}

// Download downloads a file from local storage
func (l *LocalStorage) Download(ctx context.Context, fileId string) (io.ReadCloser, *storage2.FileInfo, error) {
	fileInfo, err := l.GetFileInfo(ctx, fileId)
	if err != nil {
		return nil, nil, err
	}

	// Find the actual file
	dirPath := filepath.Join(l.basePath, fileId)
	safeFilename := l.sanitizeFilename(fileInfo.Filename)
	filePath := filepath.Join(dirPath, safeFilename)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, fileInfo, nil
}

// Delete deletes a file from local storage
func (l *LocalStorage) Delete(ctx context.Context, fileId string) error {
	dirPath := filepath.Join(l.basePath, fileId)

	// Check if directory exists
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", fileId)
	}

	// Remove entire directory
	if err := os.RemoveAll(dirPath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

// GetFileInfo gets file information
func (l *LocalStorage) GetFileInfo(ctx context.Context, fileId string) (*storage2.FileInfo, error) {
	dirPath := filepath.Join(l.basePath, fileId)
	metadataPath := filepath.Join(dirPath, ".metadata")

	// Check if metadata file exists
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("file not found: %s", fileId)
	}

	// Read metadata
	metadata, err := l.readMetadata(metadataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	// Get file stats for creation time and size verification
	safeFilename := l.sanitizeFilename(metadata.Filename)
	filePath := filepath.Join(dirPath, safeFilename)
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file stats: %w", err)
	}

	// Build storage URL
	storageURL := fmt.Sprintf("files/%s/%s", fileId, safeFilename)

	fileInfo := &storage2.FileInfo{
		FileId:      fileId,
		Filename:    metadata.Filename,
		ContentType: metadata.ContentType,
		Size:        stat.Size(),
		StorageURL:  storageURL,
		CreatedAt:   stat.ModTime().Unix(),
	}

	return fileInfo, nil
}

// GenerateURL generates a URL for file access (returns storage URL for local storage)
func (l *LocalStorage) GenerateURL(ctx context.Context, fileId string, expiration int64) (string, error) {
	fileInfo, err := l.GetFileInfo(ctx, fileId)
	if err != nil {
		return "", err
	}

	// For local storage, return the regular storage URL
	// In a real implementation, you might want to generate signed URLs with expiration
	return fileInfo.StorageURL, nil
}

// Health checks local storage health
func (l *LocalStorage) Health(ctx context.Context) error {
	// Test write permissions by creating a temporary file
	testFile := filepath.Join(l.basePath, ".health_check")
	file, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("cannot write to storage directory: %w", err)
	}
	file.Close()

	// Clean up test file
	os.Remove(testFile)

	return nil
}

// Close closes local storage (no-op for local storage)
func (l *LocalStorage) Close(ctx context.Context) error {
	return nil
}

// Helper methods

func (l *LocalStorage) sanitizeFilename(filename string) string {
	// Replace potentially problematic characters
	safe := strings.ReplaceAll(filename, "/", "_")
	safe = strings.ReplaceAll(safe, "\\", "_")
	safe = strings.ReplaceAll(safe, "..", "_")
	safe = strings.ReplaceAll(safe, ":", "_")
	safe = strings.ReplaceAll(safe, "*", "_")
	safe = strings.ReplaceAll(safe, "?", "_")
	safe = strings.ReplaceAll(safe, "\"", "_")
	safe = strings.ReplaceAll(safe, "<", "_")
	safe = strings.ReplaceAll(safe, ">", "_")
	safe = strings.ReplaceAll(safe, "|", "_")

	// Ensure filename is not empty
	if safe == "" {
		safe = "unnamed_file"
	}

	return safe
}

type metadata struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	CreatedAt   int64  `json:"created_at"`
}

func (l *LocalStorage) writeMetadata(dirPath, filename, contentType string, size int64) error {
	metadataPath := filepath.Join(dirPath, ".metadata")

	meta := metadata{
		Filename:    filename,
		ContentType: contentType,
		Size:        size,
		CreatedAt:   time.Now().Unix(),
	}

	// Write metadata as simple text format (you could use JSON if preferred)
	content := fmt.Sprintf("filename=%s\ncontent_type=%s\nsize=%d\ncreated_at=%d\n",
		meta.Filename, meta.ContentType, meta.Size, meta.CreatedAt)

	return os.WriteFile(metadataPath, []byte(content), 0644)
}

func (l *LocalStorage) readMetadata(metadataPath string) (*metadata, error) {
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	meta := &metadata{}
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key, value := parts[0], parts[1]
		switch key {
		case "filename":
			meta.Filename = value
		case "content_type":
			meta.ContentType = value
		case "size":
			fmt.Sscanf(value, "%d", &meta.Size)
		case "created_at":
			fmt.Sscanf(value, "%d", &meta.CreatedAt)
		}
	}

	if meta.Filename == "" {
		return nil, fmt.Errorf("invalid metadata: missing filename")
	}

	return meta, nil
}
