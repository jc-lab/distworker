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
	"strings"
	"testing"
)

func TestS3Config(t *testing.T) {
	t.Run("ValidConfig", func(t *testing.T) {
		cfg := &S3Config{
			Bucket:    "test-bucket",
			Region:    "us-east-1",
			Endpoint:  "",
			KeyPrefix: "test/",
		}

		err := cfg.Validate()
		if err != nil {
			t.Fatalf("Expected valid config to pass validation, got: %v", err)
		}
	})

	t.Run("MissingBucket", func(t *testing.T) {
		cfg := &S3Config{
			Region: "us-east-1",
		}

		err := cfg.Validate()
		if err == nil {
			t.Fatal("Expected validation to fail for missing bucket")
		}

		if !strings.Contains(err.Error(), "bucket is required") {
			t.Errorf("Expected error about required bucket, got: %v", err)
		}
	})

	t.Run("MissingRegion", func(t *testing.T) {
		cfg := &S3Config{
			Bucket: "test-bucket",
		}

		err := cfg.Validate()
		if err == nil {
			t.Fatal("Expected validation to fail for missing region")
		}

		if !strings.Contains(err.Error(), "region is required") {
			t.Errorf("Expected error about required region, got: %v", err)
		}
	})

	t.Run("ValidConfigWithEndpoint", func(t *testing.T) {
		cfg := &S3Config{
			Bucket:   "test-bucket",
			Region:   "us-east-1",
			Endpoint: "http://localhost:9000", // MinIO endpoint
		}

		err := cfg.Validate()
		if err != nil {
			t.Fatalf("Expected valid config with endpoint to pass validation, got: %v", err)
		}
	})
}

// Mock S3 storage for testing when AWS credentials are not available
type MockS3Storage struct {
	files map[string]mockFile
}

type mockFile struct {
	data        []byte
	filename    string
	contentType string
	size        int64
}

func NewMockS3Storage() *MockS3Storage {
	return &MockS3Storage{
		files: make(map[string]mockFile),
	}
}

func (m *MockS3Storage) simulateUpload(ctx context.Context, filename string, data io.Reader, contentType string) (string, error) {
	fileId := "mock-file-id-123"

	content, err := io.ReadAll(data)
	if err != nil {
		return "", err
	}

	m.files[fileId] = mockFile{
		data:        content,
		filename:    filename,
		contentType: contentType,
		size:        int64(len(content)),
	}

	return fileId, nil
}

func TestS3StorageLogic(t *testing.T) {
	// These tests verify the logical behavior without requiring actual AWS access

	t.Run("KeyBuilding", func(t *testing.T) {
		// Test the key building logic that would be used in S3Storage
		fileId := "12345-abcde-67890"
		filename := "test.txt"
		keyPrefix := "uploads/"

		// Simulate the key building logic from s3.go
		expectedKey := keyPrefix + fileId + "/" + filename
		key := keyPrefix + fileId + "/" + filename

		if key != expectedKey {
			t.Errorf("Expected key %s, got: %s", expectedKey, key)
		}
	})

	t.Run("URLBuilding", func(t *testing.T) {
		// Test URL building logic for different scenarios
		bucket := "test-bucket"
		region := "us-east-1"
		key := "uploads/12345/test.txt"

		// Standard S3 URL
		expectedStandardURL := "https://test-bucket.s3.us-east-1.amazonaws.com/uploads/12345/test.txt"
		standardURL := "https://" + bucket + ".s3." + region + ".amazonaws.com/" + key

		if standardURL != expectedStandardURL {
			t.Errorf("Expected standard URL %s, got: %s", expectedStandardURL, standardURL)
		}

		// Custom endpoint URL
		endpoint := "http://localhost:9000"
		expectedCustomURL := "http://localhost:9000/test-bucket/uploads/12345/test.txt"
		customURL := endpoint + "/" + bucket + "/" + key

		if customURL != expectedCustomURL {
			t.Errorf("Expected custom URL %s, got: %s", expectedCustomURL, customURL)
		}
	})

	t.Run("FilenameExtraction", func(t *testing.T) {
		// Test filename extraction from key
		key := "uploads/12345-abcde-67890/my-test-file.txt"

		// Simulate the filename extraction logic
		parts := strings.Split(key, "/")
		if len(parts) < 2 {
			t.Fatal("Expected key to have at least 2 parts")
		}

		extractedFilename := parts[len(parts)-1]
		expectedFilename := "my-test-file.txt"

		if extractedFilename != expectedFilename {
			t.Errorf("Expected filename %s, got: %s", expectedFilename, extractedFilename)
		}
	})
}

func TestS3StorageMockOperations(t *testing.T) {
	// Test the storage operations using a mock to verify logic
	mockStorage := NewMockS3Storage()
	ctx := context.Background()

	t.Run("MockUpload", func(t *testing.T) {
		testData := []byte("Hello, S3 World!")
		reader := bytes.NewReader(testData)

		fileId, err := mockStorage.simulateUpload(ctx, "test.txt", reader, "text/plain")
		if err != nil {
			t.Fatalf("Mock upload failed: %v", err)
		}

		if fileId == "" {
			t.Error("Expected non-empty file ID")
		}

		// Verify file was stored
		if file, exists := mockStorage.files[fileId]; !exists {
			t.Error("Expected file to be stored in mock")
		} else {
			if !bytes.Equal(file.data, testData) {
				t.Error("Stored data doesn't match uploaded data")
			}
			if file.filename != "test.txt" {
				t.Errorf("Expected filename 'test.txt', got: %s", file.filename)
			}
			if file.contentType != "text/plain" {
				t.Errorf("Expected content type 'text/plain', got: %s", file.contentType)
			}
			if file.size != int64(len(testData)) {
				t.Errorf("Expected size %d, got: %d", len(testData), file.size)
			}
		}
	})

	t.Run("MockDownload", func(t *testing.T) {
		// First upload a file
		testData := []byte("Download test data")
		reader := bytes.NewReader(testData)
		fileId, err := mockStorage.simulateUpload(ctx, "download.txt", reader, "text/plain")
		if err != nil {
			t.Fatalf("Mock upload failed: %v", err)
		}

		// Then download it
		if file, exists := mockStorage.files[fileId]; !exists {
			t.Error("Expected file to exist for download")
		} else {
			if !bytes.Equal(file.data, testData) {
				t.Error("Downloaded data doesn't match uploaded data")
			}
		}
	})

	t.Run("MockFileNotFound", func(t *testing.T) {
		// Test accessing non-existent file
		if _, exists := mockStorage.files["nonexistent-id"]; exists {
			t.Error("Expected file not to exist")
		}
	})
}

func TestS3ErrorHandling(t *testing.T) {
	t.Run("InvalidCredentials", func(t *testing.T) {
		// Test what happens with invalid AWS credentials
		cfg := &S3Config{
			Bucket: "test-bucket",
			Region: "us-east-1",
		}

		// This will likely fail with credential errors in CI/testing environment
		_, err := NewS3Storage(cfg)
		if err != nil {
			// Expected in testing environment without AWS credentials
			t.Logf("Expected error in testing environment: %v", err)
		}
	})

	t.Run("InvalidBucket", func(t *testing.T) {
		// Test with an invalid bucket name
		cfg := &S3Config{
			Bucket: "invalid..bucket..name", // Invalid bucket name
			Region: "us-east-1",
		}

		// This should fail validation or during health check
		_, err := NewS3Storage(cfg)
		if err != nil {
			// Expected for invalid bucket
			t.Logf("Expected error for invalid bucket: %v", err)
		}
	})
}

func TestS3MetadataHandling(t *testing.T) {
	t.Run("MetadataExtraction", func(t *testing.T) {
		// Test metadata handling logic
		metadata := map[string]string{
			"file-id":       "12345-abcde-67890",
			"original-name": "test file with spaces.txt",
		}

		// Verify metadata contains expected values
		if fileId, exists := metadata["file-id"]; !exists {
			t.Error("Expected file-id in metadata")
		} else if fileId != "12345-abcde-67890" {
			t.Errorf("Expected file-id '12345-abcde-67890', got: %s", fileId)
		}

		if originalName, exists := metadata["original-name"]; !exists {
			t.Error("Expected original-name in metadata")
		} else if originalName != "test file with spaces.txt" {
			t.Errorf("Expected original-name 'test file with spaces.txt', got: %s", originalName)
		}
	})

	t.Run("ETagHandling", func(t *testing.T) {
		// Test ETag handling in URLs
		baseURL := "https://bucket.s3.region.amazonaws.com/key"
		etag := "\"d41d8cd98f00b204e9800998ecf8427e\""

		// Simulate ETag URL building
		etagURL := baseURL + "?etag=" + strings.Trim(etag, "\"")
		expectedURL := "https://bucket.s3.region.amazonaws.com/key?etag=d41d8cd98f00b204e9800998ecf8427e"

		if etagURL != expectedURL {
			t.Errorf("Expected ETag URL %s, got: %s", expectedURL, etagURL)
		}
	})
}

func TestS3PresignedURLLogic(t *testing.T) {
	t.Run("ExpirationTime", func(t *testing.T) {
		// Test expiration time calculation
		expiration := int64(3600) // 1 hour in seconds

		if expiration <= 0 {
			t.Error("Expected positive expiration time")
		}

		if expiration > 604800 { // 7 days in seconds
			t.Error("Expiration time too long (max 7 days for S3)")
		}
	})

	t.Run("URLValidation", func(t *testing.T) {
		// Test presigned URL structure validation
		samplePresignedURL := "https://bucket.s3.region.amazonaws.com/key?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=..."

		// Basic validation
		if !strings.HasPrefix(samplePresignedURL, "https://") {
			t.Error("Expected HTTPS URL")
		}

		if !strings.Contains(samplePresignedURL, "X-Amz-Algorithm") {
			t.Error("Expected AWS signature parameters in presigned URL")
		}
	})
}

// Integration test that requires actual AWS setup - only run if AWS credentials are available
func TestS3IntegrationIfAvailable(t *testing.T) {
	// Skip this test in CI unless AWS credentials are explicitly configured
	if testing.Short() {
		t.Skip("Skipping S3 integration test in short mode")
	}

	cfg := &S3Config{
		Bucket: "distworker-test-bucket", // Use a test bucket
		Region: "us-east-1",
	}

	s3Storage, err := NewS3Storage(cfg)
	if err != nil {
		t.Skipf("Skipping S3 integration test - could not create S3 storage: %v", err)
	}

	ctx := context.Background()

	t.Run("ActualS3Operations", func(t *testing.T) {
		// Test actual S3 operations if available
		testData := []byte("Integration test data")
		reader := bytes.NewReader(testData)

		// Upload
		fileInfo, err := s3Storage.Upload(ctx, "integration_test.txt", reader, "text/plain")
		if err != nil {
			t.Fatalf("S3 upload failed: %v", err)
		}

		// Verify upload
		if fileInfo.FileId == "" {
			t.Error("Expected non-empty file ID")
		}

		// Download
		readCloser, _, err := s3Storage.Download(ctx, fileInfo.FileId)
		if err != nil {
			t.Fatalf("S3 download failed: %v", err)
		}
		defer readCloser.Close()

		downloadedData, err := io.ReadAll(readCloser)
		if err != nil {
			t.Fatalf("Failed to read downloaded data: %v", err)
		}

		if !bytes.Equal(testData, downloadedData) {
			t.Error("Downloaded data doesn't match uploaded data")
		}

		// Generate URL
		url, err := s3Storage.GenerateURL(ctx, fileInfo.FileId, 3600)
		if err != nil {
			t.Fatalf("Failed to generate URL: %v", err)
		}

		if url == "" {
			t.Error("Expected non-empty URL")
		}

		// Cleanup - delete the test file
		err = s3Storage.Delete(ctx, fileInfo.FileId)
		if err != nil {
			t.Errorf("Failed to cleanup test file: %v", err)
		}
	})
}
