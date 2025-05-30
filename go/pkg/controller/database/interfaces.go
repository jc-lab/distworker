package database

import (
	"context"
	"github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
)

// Database represents the main database interface
type Database interface {
	// Close closes the database connection
	Close(ctx context.Context) error

	// Health checks the database health
	Health(ctx context.Context) error

	// GetTaskRepository returns the task repository
	GetTaskRepository() TaskRepositoryInterface

	// GetQueueRepository returns the queue repository
	GetQueueRepository() QueueRepositoryInterface

	// GetWorkerSessionRepository returns the worker session repository
	GetWorkerSessionRepository() WorkerSessionRepositoryInterface
}

// TaskRepositoryInterface defines the interface for task database operations
type TaskRepositoryInterface interface {
	// Create creates a new task
	Create(ctx context.Context, task *models.Task) error

	// GetById retrieves a task by ID
	GetById(ctx context.Context, taskId string) (*models.Task, error)

	// Update updates a task
	Update(ctx context.Context, task *models.Task) error

	// Delete deletes a task
	Delete(ctx context.Context, taskId string) error

	// List retrieves tasks with pagination and filters
	List(ctx context.Context, filter TaskFilter, page, limit int) ([]*models.Task, int64, error)

	// GetPendingTasks retrieves pending tasks for a queue
	GetPendingTasks(ctx context.Context, queue string, limit int) ([]*models.Task, error)
}

// QueueRepositoryInterface defines the interface for queue database operations
type QueueRepositoryInterface interface {
	// Create creates a new queue
	Create(ctx context.Context, queue *models.Queue) error

	// GetByName retrieves a queue by name
	GetByName(ctx context.Context, name string) (*models.Queue, error)

	// Update updates a queue
	Update(ctx context.Context, queue *models.Queue) error

	// Delete deletes a queue
	Delete(ctx context.Context, name string) error

	// List retrieves all queues
	List(ctx context.Context) ([]*models.Queue, error)
}

// WorkerSessionRepositoryInterface defines the interface for worker session database operations
type WorkerSessionRepositoryInterface interface {
	// Create creates a new worker session
	Create(ctx context.Context, session *models.WorkerSession) error

	// GetById retrieves a worker session by ID
	GetById(ctx context.Context, workerId string) (*models.WorkerSession, error)

	// Update updates a worker session
	Update(ctx context.Context, session *models.WorkerSession) error

	// Delete deletes a worker session
	Delete(ctx context.Context, workerId string) error

	// List retrieves all active worker sessions
	List(ctx context.Context) ([]*models.WorkerSession, error)

	// UpdateHeartbeat updates the last heartbeat time for a worker
	UpdateHeartbeat(ctx context.Context, workerId string, status types.WorkerStatus) error
}

// TaskFilter represents filters for task queries
type TaskFilter struct {
	Queue    string
	Status   string
	WorkerId string
}
