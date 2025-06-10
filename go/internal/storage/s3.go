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
	"context"
	"fmt"
	storage2 "github.com/jc-lab/distworker/go/pkg/controller/storage"
	"io"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const TypeS3 = "s3"

// S3Storage implements Storage interface for AWS S3
type S3Storage struct {
	client    *s3.Client
	bucket    string
	region    string
	endpoint  string // for S3-compatible services
	keyPrefix string // optional prefix for all keys
}

// S3Config implements StorageConfig for S3
type S3Config struct {
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
	Endpoint  string `yaml:"endpoint,omitempty"`   // for S3-compatible services
	KeyPrefix string `yaml:"key_prefix,omitempty"` // optional prefix
}

// Validate validates the S3 configuration
func (c *S3Config) Validate() error {
	if c.Bucket == "" {
		return fmt.Errorf("S3 bucket is required")
	}
	if c.Region == "" {
		return fmt.Errorf("S3 region is required")
	}
	return nil
}

// NewS3Storage creates a new S3 storage instance
func NewS3Storage(cfg *S3Config) (*S3Storage, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	// Load AWS config
	awsConfig, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(cfg.Region),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create S3 client
	var client *s3.Client
	if cfg.Endpoint != "" {
		// Use custom endpoint (for S3-compatible services)
		client = s3.NewFromConfig(awsConfig, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
			o.UsePathStyle = true // Required for most S3-compatible services
		})
	} else {
		client = s3.NewFromConfig(awsConfig)
	}

	storage := &S3Storage{
		client:    client,
		bucket:    cfg.Bucket,
		region:    cfg.Region,
		endpoint:  cfg.Endpoint,
		keyPrefix: cfg.KeyPrefix,
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := storage.Health(ctx); err != nil {
		return nil, fmt.Errorf("S3 health check failed: %w", err)
	}

	return storage, nil
}

// Upload uploads a file to S3
func (s *S3Storage) Upload(ctx context.Context, filename string, data io.Reader, contentType string) (*storage2.FileInfo, error) {
	// Generate unique file ID
	fileId := uuid.New().String()

	// Create S3 key
	key := s.buildKey(fileId, filename)

	// Prepare upload input
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        data,
		ContentType: aws.String(contentType),
		Metadata: map[string]string{
			"file-id":       fileId,
			"original-name": filename,
		},
	}

	// Upload to S3
	result, err := s.client.PutObject(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Get object size
	headResult, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Build storage URL
	storageURL := s.buildStorageURL(key)

	fileInfo := &storage2.FileInfo{
		FileId:      fileId,
		Filename:    filename,
		ContentType: contentType,
		Size:        *headResult.ContentLength,
		StorageURL:  storageURL,
		CreatedAt:   time.Now().Unix(),
	}

	// Add ETag if available
	if result.ETag != nil {
		fileInfo.StorageURL = fmt.Sprintf("%s?etag=%s", storageURL, strings.Trim(*result.ETag, "\""))
	}

	return fileInfo, nil
}

// Download downloads a file from S3
func (s *S3Storage) Download(ctx context.Context, fileId string) (io.ReadCloser, *storage2.FileInfo, error) {
	// Find the object by file ID
	key, fileInfo, err := s.findObjectByFileID(ctx, fileId)
	if err != nil {
		return nil, nil, err
	}

	// Download object
	result, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to download from S3: %w", err)
	}

	return result.Body, fileInfo, nil
}

// Delete deletes a file from S3
func (s *S3Storage) Delete(ctx context.Context, fileId string) error {
	// Find the object by file ID
	key, _, err := s.findObjectByFileID(ctx, fileId)
	if err != nil {
		return err
	}

	// Delete object
	_, err = s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}

	return nil
}

// GetFileInfo gets file information
func (s *S3Storage) GetFileInfo(ctx context.Context, fileId string) (*storage2.FileInfo, error) {
	_, fileInfo, err := s.findObjectByFileID(ctx, fileId)
	return fileInfo, err
}

// GenerateURL generates a presigned URL
func (s *S3Storage) GenerateURL(ctx context.Context, fileId string, expiration int64) (string, error) {
	// Find the object by file ID
	key, _, err := s.findObjectByFileID(ctx, fileId)
	if err != nil {
		return "", err
	}

	// Create presign client
	presignClient := s3.NewPresignClient(s.client)

	// Generate presigned URL
	request, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}, func(opts *s3.PresignOptions) {
		opts.Expires = time.Duration(expiration) * time.Second
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	return request.URL, nil
}

// Health checks S3 connectivity
func (s *S3Storage) Health(ctx context.Context) error {
	// Try to head the bucket
	_, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	if err != nil {
		return fmt.Errorf("S3 health check failed: %w", err)
	}
	return nil
}

// Close closes S3 connections (no-op for S3)
func (s *S3Storage) Close(ctx context.Context) error {
	return nil
}

// Helper methods

func (s *S3Storage) buildKey(fileId, filename string) string {
	// Use file Id as primary key with original filename for reference
	key := fmt.Sprintf("%s/%s", fileId, filename)
	if s.keyPrefix != "" {
		key = fmt.Sprintf("%s/%s", s.keyPrefix, key)
	}
	return key
}

func (s *S3Storage) buildStorageURL(key string) string {
	if s.endpoint != "" {
		return fmt.Sprintf("%s/%s/%s", s.endpoint, s.bucket, key)
	}
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key)
}

func (s *S3Storage) findObjectByFileID(ctx context.Context, fileId string) (string, *storage2.FileInfo, error) {
	// List objects with the file Id prefix
	prefix := fileId
	if s.keyPrefix != "" {
		prefix = fmt.Sprintf("%s/%s", s.keyPrefix, fileId)
	}

	result, err := s.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(s.bucket),
		Prefix:  aws.String(prefix),
		MaxKeys: aws.Int32(1),
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to list objects: %w", err)
	}

	if len(result.Contents) == 0 {
		return "", nil, fmt.Errorf("file not found: %s", fileId)
	}

	obj := result.Contents[0]
	key := *obj.Key

	// Get object metadata
	headResult, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return "", nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	// Extract original filename from key or metadata
	filename := filepath.Base(key)
	if originalName, exists := headResult.Metadata["original-name"]; exists {
		filename = originalName
	}

	// Get content type
	contentType := ""
	if headResult.ContentType != nil {
		contentType = *headResult.ContentType
	}

	fileInfo := &storage2.FileInfo{
		FileId:      fileId,
		Filename:    filename,
		ContentType: contentType,
		Size:        *headResult.ContentLength,
		StorageURL:  s.buildStorageURL(key),
		CreatedAt:   headResult.LastModified.Unix(),
	}

	return key, fileInfo, nil
}
