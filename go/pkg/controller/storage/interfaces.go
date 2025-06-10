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
	"io"
)

// Storage represents the main storage interface
type Storage interface {
	// Upload uploads a file and returns the file ID
	Upload(ctx context.Context, filename string, data io.Reader, contentType string) (*FileInfo, error)

	// Download downloads a file by file ID
	Download(ctx context.Context, fileId string) (io.ReadCloser, *FileInfo, error)

	// Delete deletes a file by file ID
	Delete(ctx context.Context, fileId string) error

	// GetFileInfo gets file information by file ID
	GetFileInfo(ctx context.Context, fileId string) (*FileInfo, error)

	// GenerateURL generates a signed URL for file access (if supported)
	GenerateURL(ctx context.Context, fileId string, expiration int64) (string, error)

	// Health checks storage health
	Health(ctx context.Context) error

	// Close closes storage connections
	Close(ctx context.Context) error
}

// FileInfo represents file metadata
type FileInfo struct {
	FileId      string `json:"file_id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	StorageURL  string `json:"storage_url,omitempty"` // For S3, this could be the S3 URL
	CreatedAt   int64  `json:"created_at"`            // Unix timestamp
}
