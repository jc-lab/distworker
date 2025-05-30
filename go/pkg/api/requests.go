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
	TaskId string `json:"task_id"`
	Status string `json:"status"`
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
