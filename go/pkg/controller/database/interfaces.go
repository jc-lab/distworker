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

	Stat(ctx context.Context) (*TaskCollectionStat, error)

	// ListAll retrieves tasks with pagination and filters
	ListAll(ctx context.Context, filter TaskFilter) (chan *models.Task, error)
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
	UpdateHeartbeat(ctx context.Context, workerId string, health types.WorkerHealth) error
}

// TaskFilter represents filters for task queries
type TaskFilter struct {
	Queue     string
	StatusGte types.TaskStatus
	StatusLt  types.TaskStatus
	WorkerId  string
}
