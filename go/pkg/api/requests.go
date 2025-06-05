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

package api

import (
	"github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
)

// Task API Request/Response structures

// CreateTaskRequest represents the request body for creating a task
type CreateTaskRequest struct {
	Queue    string                 `json:"queue"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	Input    map[string]interface{} `json:"input,omitempty"`
	Timeout  string                 `json:"timeout,omitempty"`
	Retry    int                    `json:"retry,omitempty"`
}

// ListTasksResponse represents the response for listing tasks
type ListTasksResponse struct {
	Tasks      []*models.Task `json:"tasks"`
	Pagination PaginationInfo `json:"pagination"`
}

// PaginationInfo contains pagination metadata
type PaginationInfo struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"total_pages"`
}

// DeleteTaskResponse represents the response for deleting/cancelling a task
type DeleteTaskResponse struct {
	TaskId string           `json:"task_id"`
	Status types.TaskStatus `json:"status"`
}

// Queue API Request/Response structures

// CreateQueueRequest represents the request body for creating a queue
type CreateQueueRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// UpdateQueueRequest represents the request body for updating a queue
type UpdateQueueRequest struct {
	Description string `json:"description"`
}

// ListQueuesResponse represents the response for listing queues
type ListQueuesResponse struct {
	Queues []*models.Queue `json:"queues"`
}

// DeleteQueueResponse represents the response for deleting a queue
type DeleteQueueResponse struct {
	Status string `json:"status"`
}

// Worker API Response structures

// ListWorkersResponse represents the response for listing workers
type ListWorkersResponse struct {
	Workers []*models.Worker `json:"workers"`
}

// DeleteWorkerResponse represents the response for deleting a worker
type DeleteWorkerResponse struct {
	Status      string `json:"status"`
	CurrentTask string `json:"current_task,omitempty"`
}

// Health API Response structures

type HealthDetail struct {
	Status       types.HealthStatus `json:"status"`
	ResponseTime int64              `json:"response_time"`
	Message      *string            `json:"message,omitempty"`
}

// HealthResponse represents the response for health check
type HealthResponse struct {
	Status    types.HealthStatus       `json:"status"`
	Timestamp int64                    `json:"timestamp"`
	Version   string                   `json:"version"`
	Details   map[string]*HealthDetail `json:"details"`
	Error     *string                  `json:"error,omitempty"`
}

// ErrorResponse represents a generic error response
type ErrorResponse struct {
	Status string `json:"status"`
	Error  string `json:"error"`
}
