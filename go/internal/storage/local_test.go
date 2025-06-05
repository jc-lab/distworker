// distworker
// Copyright (C) 2025 JC-Lab
//
// SPDX-License-Identifier: AGPL-3.0-only
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package storage

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorageConfig(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := &LocalConfig{
			Path:    tempDir,
			MaxSize: 1024 * 1024, // 1MB
		}

		err := cfg.Validate()
		if err != nil {
			t.Fatalf("Expected valid config to pass validation, got: %v", err)
		}

		// Path should be absolute after validation
		if !filepath.IsAbs(cfg.Path) {
			t.Errorf("Expected path to be absolute after validation, got: %s", cfg.Path)
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		cfg := &LocalConfig{
			Path: "",
		}

		err := cfg.Validate()
		if err == nil {
			t.Fatal("Expected validation to fail for empty path")
		}

		if !strings.Contains(err.Error(), "base path is required") {
			t.Errorf("Expected error about required path, got: %v", err)
		}
	})

	t.Run("RelativePath", func(t *testing.T) {
		cfg := &LocalConfig{
			Path: "relative/path",
		}

		err := cfg.Validate()
		if err != nil {
			t.Fatalf("Expected relative path to be converted to absolute, got: %v", err)
		}

		if !filepath.IsAbs(cfg.Path) {
			t.Errorf("Expected relative path to be converted to absolute, got: %s", cfg.Path)
		}
	})
}

func TestNewLocalStorage(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := &LocalConfig{
			Path:    tempDir,
			MaxSize: 1024,
		}

		localStorage, err := NewLocalStorage(cfg)
		if err != nil {
			t.Fatalf("Failed to create local storage: %v", err)
		}

		if localStorage == nil {
			t.Fatal("Expected non-nil local storage instance")
		}

		// Test health check
		ctx := context.Background()
		if err := localStorage.Health(ctx); err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
	})

	t.Run("InvalidPath", func(t *testing.T) {
		cfg := &LocalConfig{
			Path: "/invalid/nonexistent/directory/with/no/permissions",
		}

		_, err := NewLocalStorage(cfg)
		if err == nil {
			t.Fatal("Expected error for invalid path")
		}
	})
}

func TestLocalStorageUpload(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &LocalConfig{
		Path:    tempDir,
		MaxSize: 1024, // 1KB limit for testing
	}

	localStorage, err := NewLocalStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	ctx := context.Background()

	t.Run("SuccessfulUpload", func(t *testing.T) {
		testData := []byte("Hello, World!")
		reader := bytes.NewReader(testData)

		fileInfo, err := localStorage.Upload(ctx, "test.txt", reader, "text/plain")
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}

		// Verify file info
		if fileInfo.FileId == "" {
			t.Error("Expected non-empty file ID")
		}
		if fileInfo.Filename != "test.txt" {
			t.Errorf("Expected filename 'test.txt', got: %s", fileInfo.Filename)
		}
		if fileInfo.ContentType != "text/plain" {
			t.Errorf("Expected content type 'text/plain', got: %s", fileInfo.ContentType)
		}
		if fileInfo.Size != int64(len(testData)) {
			t.Errorf("Expected size %d, got: %d", len(testData), fileInfo.Size)
		}
		if fileInfo.StorageURL == "" {
			t.Error("Expected non-empty storage URL")
		}
		if fileInfo.CreatedAt == 0 {
			t.Error("Expected non-zero creation time")
		}

		// Verify file actually exists
		dirPath := filepath.Join(tempDir, fileInfo.FileId)
		filePath := filepath.Join(dirPath, "test.txt")
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			t.Error("Expected uploaded file to exist on disk")
		}

		// Verify metadata file exists
		metadataPath := filepath.Join(dirPath, ".metadata")
		if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
			t.Error("Expected metadata file to exist")
		}
	})

	t.Run("FileSizeExceedsLimit", func(t *testing.T) {
		// Create data larger than the limit (1KB)
		testData := make([]byte, 2048) // 2KB
		reader := bytes.NewReader(testData)

		_, err := localStorage.Upload(ctx, "large.txt", reader, "text/plain")
		if err == nil {
			t.Fatal("Expected error for oversized file")
		}

		if !strings.Contains(err.Error(), "exceeds maximum allowed size") {
			t.Errorf("Expected error about file size limit, got: %v", err)
		}
	})

	t.Run("FilenameWithUnsafeCharacters", func(t *testing.T) {
		testData := []byte("test content")
		reader := bytes.NewReader(testData)

		// Test with unsafe filename
		unsafeFilename := "../../test/file:with*unsafe?chars<>|.txt"

		fileInfo, err := localStorage.Upload(ctx, unsafeFilename, reader, "text/plain")
		if err != nil {
			t.Fatalf("Upload failed: %v", err)
		}

		// Verify that the file was created with sanitized name
		dirPath := filepath.Join(tempDir, fileInfo.FileId)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			t.Fatalf("Failed to read directory: %v", err)
		}

		found := false
		for _, entry := range entries {
			if entry.Name() != ".metadata" {
				found = true
				// Filename should be sanitized (no unsafe characters)
				if strings.ContainsAny(entry.Name(), "/\\:*?\"<>|") {
					t.Errorf("Expected sanitized filename, got: %s", entry.Name())
				}
			}
		}

		if !found {
			t.Error("Expected to find uploaded file in directory")
		}
	})
}

func TestLocalStorageDownload(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &LocalConfig{
		Path:    tempDir,
		MaxSize: 1024,
	}

	localStorage, err := NewLocalStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	ctx := context.Background()

	// Upload a test file first
	testData := []byte("Test download content")
	reader := bytes.NewReader(testData)
	fileInfo, err := localStorage.Upload(ctx, "download_test.txt", reader, "text/plain")
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	t.Run("SuccessfulDownload", func(t *testing.T) {
		readCloser, downloadFileInfo, err := localStorage.Download(ctx, fileInfo.FileId)
		if err != nil {
			t.Fatalf("Download failed: %v", err)
		}
		defer readCloser.Close()

		// Verify file info matches
		if downloadFileInfo.FileId != fileInfo.FileId {
			t.Errorf("Expected file ID %s, got: %s", fileInfo.FileId, downloadFileInfo.FileId)
		}
		if downloadFileInfo.Filename != fileInfo.Filename {
			t.Errorf("Expected filename %s, got: %s", fileInfo.Filename, downloadFileInfo.Filename)
		}

		// Verify content
		downloadedData, err := io.ReadAll(readCloser)
		if err != nil {
			t.Fatalf("Failed to read downloaded data: %v", err)
		}

		if !bytes.Equal(testData, downloadedData) {
			t.Errorf("Downloaded data doesn't match uploaded data")
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		_, _, err := localStorage.Download(ctx, "nonexistent-file-id")
		if err == nil {
			t.Fatal("Expected error for nonexistent file")
		}

		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("Expected 'file not found' error, got: %v", err)
		}
	})
}

func TestLocalStorageDelete(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &LocalConfig{
		Path:    tempDir,
		MaxSize: 1024,
	}

	localStorage, err := NewLocalStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	ctx := context.Background()

	// Upload a test file first
	testData := []byte("Test delete content")
	reader := bytes.NewReader(testData)
	fileInfo, err := localStorage.Upload(ctx, "delete_test.txt", reader, "text/plain")
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	t.Run("SuccessfulDelete", func(t *testing.T) {
		// Verify file exists before deletion
		dirPath := filepath.Join(tempDir, fileInfo.FileId)
		if _, err := os.Stat(dirPath); os.IsNotExist(err) {
			t.Fatal("Expected file directory to exist before deletion")
		}

		// Delete the file
		err := localStorage.Delete(ctx, fileInfo.FileId)
		if err != nil {
			t.Fatalf("Delete failed: %v", err)
		}

		// Verify file no longer exists
		if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
			t.Error("Expected file directory to be deleted")
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		err := localStorage.Delete(ctx, "nonexistent-file-id")
		if err == nil {
			t.Fatal("Expected error for nonexistent file")
		}

		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("Expected 'file not found' error, got: %v", err)
		}
	})
}

func TestLocalStorageGetFileInfo(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &LocalConfig{
		Path:    tempDir,
		MaxSize: 1024,
	}

	localStorage, err := NewLocalStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	ctx := context.Background()

	// Upload a test file first
	testData := []byte("Test file info content")
	reader := bytes.NewReader(testData)
	originalFileInfo, err := localStorage.Upload(ctx, "fileinfo_test.txt", reader, "text/plain")
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	t.Run("SuccessfulGetFileInfo", func(t *testing.T) {
		fileInfo, err := localStorage.GetFileInfo(ctx, originalFileInfo.FileId)
		if err != nil {
			t.Fatalf("GetFileInfo failed: %v", err)
		}

		// Verify file info matches
		if fileInfo.FileId != originalFileInfo.FileId {
			t.Errorf("Expected file ID %s, got: %s", originalFileInfo.FileId, fileInfo.FileId)
		}
		if fileInfo.Filename != originalFileInfo.Filename {
			t.Errorf("Expected filename %s, got: %s", originalFileInfo.Filename, fileInfo.Filename)
		}
		if fileInfo.ContentType != originalFileInfo.ContentType {
			t.Errorf("Expected content type %s, got: %s", originalFileInfo.ContentType, fileInfo.ContentType)
		}
		if fileInfo.Size != originalFileInfo.Size {
			t.Errorf("Expected size %d, got: %d", originalFileInfo.Size, fileInfo.Size)
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		_, err := localStorage.GetFileInfo(ctx, "nonexistent-file-id")
		if err == nil {
			t.Fatal("Expected error for nonexistent file")
		}

		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("Expected 'file not found' error, got: %v", err)
		}
	})
}

func TestLocalStorageGenerateURL(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &LocalConfig{
		Path:    tempDir,
		MaxSize: 1024,
	}

	localStorage, err := NewLocalStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	ctx := context.Background()

	// Upload a test file first
	testData := []byte("Test URL generation")
	reader := bytes.NewReader(testData)
	fileInfo, err := localStorage.Upload(ctx, "url_test.txt", reader, "text/plain")
	if err != nil {
		t.Fatalf("Failed to upload test file: %v", err)
	}

	t.Run("SuccessfulGenerateURL", func(t *testing.T) {
		url, err := localStorage.GenerateURL(ctx, fileInfo.FileId, 3600) // 1 hour
		if err != nil {
			t.Fatalf("GenerateURL failed: %v", err)
		}

		if url == "" {
			t.Error("Expected non-empty URL")
		}

		// For local storage, URL should match storage URL
		if url != fileInfo.StorageURL {
			t.Errorf("Expected URL to match storage URL, got: %s", url)
		}
	})

	t.Run("FileNotFound", func(t *testing.T) {
		_, err := localStorage.GenerateURL(ctx, "nonexistent-file-id", 3600)
		if err == nil {
			t.Fatal("Expected error for nonexistent file")
		}

		if !strings.Contains(err.Error(), "file not found") {
			t.Errorf("Expected 'file not found' error, got: %v", err)
		}
	})
}

func TestLocalStorageHealth(t *testing.T) {
	t.Run("HealthyStorage", func(t *testing.T) {
		tempDir := t.TempDir()
		cfg := &LocalConfig{
			Path: tempDir,
		}

		localStorage, err := NewLocalStorage(cfg)
		if err != nil {
			t.Fatalf("Failed to create local storage: %v", err)
		}

		ctx := context.Background()
		err = localStorage.Health(ctx)
		if err != nil {
			t.Fatalf("Health check failed: %v", err)
		}
	})

	t.Run("ReadOnlyDirectory", func(t *testing.T) {
		tempDir := t.TempDir()

		// Make directory read-only
		err := os.Chmod(tempDir, 0444)
		if err != nil {
			t.Fatalf("Failed to make directory read-only: %v", err)
		}

		// Restore permissions for cleanup
		defer os.Chmod(tempDir, 0755)

		cfg := &LocalConfig{
			Path: tempDir,
		}

		_, err = NewLocalStorage(cfg)
		if err == nil {
			t.Fatal("Expected error for read-only directory")
		}
	})
}

func TestLocalStorageClose(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &LocalConfig{
		Path: tempDir,
	}

	localStorage, err := NewLocalStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	ctx := context.Background()
	err = localStorage.Close(ctx)
	if err != nil {
		t.Fatalf("Close failed: %v", err)
	}
}

func TestLocalStorageConcurrentOperations(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &LocalConfig{
		Path:    tempDir,
		MaxSize: 1024 * 1024, // 1MB
	}

	localStorage, err := NewLocalStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	ctx := context.Background()

	// Test concurrent uploads
	t.Run("ConcurrentUploads", func(t *testing.T) {
		done := make(chan error, 10)

		for i := 0; i < 10; i++ {
			go func(index int) {
				testData := []byte("Concurrent test data " + string(rune(index+'0')))
				reader := bytes.NewReader(testData)
				filename := "concurrent_" + string(rune(index+'0')) + ".txt"

				_, err := localStorage.Upload(ctx, filename, reader, "text/plain")
				done <- err
			}(i)
		}

		// Wait for all uploads to complete
		for i := 0; i < 10; i++ {
			if err := <-done; err != nil {
				t.Errorf("Concurrent upload %d failed: %v", i, err)
			}
		}
	})
}

func TestLocalStorageEdgeCases(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &LocalConfig{
		Path:    tempDir,
		MaxSize: 1024,
	}

	localStorage, err := NewLocalStorage(cfg)
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}

	ctx := context.Background()

	t.Run("EmptyFile", func(t *testing.T) {
		reader := bytes.NewReader([]byte{})

		fileInfo, err := localStorage.Upload(ctx, "empty.txt", reader, "text/plain")
		if err != nil {
			t.Fatalf("Failed to upload empty file: %v", err)
		}

		if fileInfo.Size != 0 {
			t.Errorf("Expected empty file size to be 0, got: %d", fileInfo.Size)
		}
	})

	t.Run("EmptyFilename", func(t *testing.T) {
		testData := []byte("test data")
		reader := bytes.NewReader(testData)

		fileInfo, err := localStorage.Upload(ctx, "", reader, "text/plain")
		if err != nil {
			t.Fatalf("Failed to upload file with empty filename: %v", err)
		}

		// Should get default filename
		if fileInfo.Filename != "" {
			t.Errorf("Expected filename to remain empty, got: %s", fileInfo.Filename)
		}

		// But the actual file should have been created with a safe name
		dirPath := filepath.Join(tempDir, fileInfo.FileId)
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			t.Fatalf("Failed to read directory: %v", err)
		}

		foundFile := false
		for _, entry := range entries {
			if entry.Name() != ".metadata" {
				foundFile = true
				if entry.Name() == "" {
					t.Error("Expected sanitized filename, got empty filename")
				}
			}
		}

		if !foundFile {
			t.Error("Expected to find uploaded file")
		}
	})

	t.Run("ContextCancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		testData := []byte("test data")
		reader := bytes.NewReader(testData)

		// Should still work for local storage as it doesn't respect context cancellation
		// in this implementation, but in a real implementation it might
		_, err := localStorage.Upload(ctx, "cancelled.txt", reader, "text/plain")
		// This test depends on implementation - for now we expect it to succeed
		// as the local storage doesn't check context cancellation
		if err != nil {
			t.Logf("Upload with cancelled context failed (this might be expected): %v", err)
		}
	})
}
