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

package models

import (
	"github.com/google/uuid"
	"github.com/jc-lab/distworker/go/pkg/types"
	"sync"
)

type ControllerSetting struct {
	WorkerAccessibleBaseUrl string `yaml:"worker_accessible_base_url"`
}

// Task represents a task in the system
type Task struct {
	Id          string                 `bson:"_id" json:"task_id"`
	Status      types.TaskStatus       `bson:"status" json:"status"`
	Queue       string                 `bson:"queue" json:"queue"`
	Metadata    map[string]interface{} `bson:"metadata" json:"metadata"`
	Input       map[string]interface{} `bson:"input" json:"input"`
	Result      map[string]interface{} `bson:"result,omitempty" json:"result,omitempty"`
	Error       *TaskError             `bson:"error,omitempty" json:"error,omitempty"`
	Files       []FileInfo             `bson:"files,omitempty" json:"files,omitempty"`
	ResultFiles []FileInfo             `bson:"result_files,omitempty" json:"result_files,omitempty"`
	WorkerId    string                 `bson:"worker_id,omitempty" json:"worker_id"`
	TimeoutMS   int64                  `bson:"timeout_ms" json:"timeout"`
	CreatedAt   UnixTime               `bson:"created_at" json:"created_at"`
	StartedAt   *UnixTime              `bson:"started_at" json:"started_at"`
	CompletedAt *UnixTime              `bson:"completed_at" json:"completed_at"`
	MaxRetry    int                    `bson:"max_retry" json:"max_retry"`
	Retried     int                    `bson:"retried" json:"retried"`
	WebhookUrl  string                 `bson:"webhook_url" json:"webhook_url"`
}

type TaskProgress struct {
	Message string
	Data    map[string]interface{}
}

// TaskError represents an error that occurred during task execution
type TaskError struct {
	Code    types.TaskErrorCode    `bson:"code" json:"code"`
	Message string                 `bson:"message" json:"message"`
	Details map[string]interface{} `bson:"details,omitempty" json:"details,omitempty"`
}

// FileInfo represents information about a file
type FileInfo struct {
	FileId      string `bson:"file_id" json:"file_id"`
	Filename    string `bson:"filename" json:"filename"`
	ContentType string `bson:"content_type" json:"content_type"`
	Size        int64  `bson:"size" json:"size"`
	StorageURL  string `bson:"storage_url" json:"storage_url"`
}

// Queue represents a task queue
type Queue struct {
	Name        string            `bson:"_id" json:"name"`
	Description string            `bson:"description" json:"description"`
	Status      types.QueueStatus `bson:"status" json:"status"`
	CreatedAt   UnixTime          `bson:"created_at" json:"created_at"`
	UpdatedAt   UnixTime          `bson:"updated_at" json:"updated_at"`
}

// Worker represents a connected worker
type Worker struct {
	Id            string                 `json:"worker_id"`
	Status        types.WorkerStatus     `json:"status"`
	Queues        []string               `json:"queues"`
	CurrentTask   string                 `json:"current_task,omitempty"`
	ConnectedAt   UnixTime               `json:"connected_at"`
	LastHeartbeat UnixTime               `json:"last_heartbeat"`
	Provisioner   string                 `json:"provisioner_name"`
	ResourceInfo  map[string]interface{} `json:"resource_info"`
}

// Provisioner represents a worker provisioner
type Provisioner struct {
	Name          string                  `json:"name"`
	URL           string                  `json:"url"`
	Status        types.ProvisionerStatus `json:"status"`
	Workers       int                     `json:"workers"`
	MaxWorkers    int                     `json:"max_workers"`
	LastHeartbeat UnixTime                `json:"last_heartbeat"`
}

// QueueStats represents statistics for a queue
type QueueStats struct {
	QueueName           string   `json:"queue_name"`
	TotalTasks          int64    `json:"total_tasks"`
	CompletedTasks      int64    `json:"completed_tasks"`
	FailedTasks         int64    `json:"failed_tasks"`
	AvgProcessingTimeMS int64    `json:"avg_processing_time"`
	PeakWorkers         int      `json:"peak_workers"`
	CurrentWorkers      int      `json:"current_workers"`
	Workers             []Worker `json:"workers"`
}

// WorkerSession represents a worker session in the database
type WorkerSession struct {
	mutex sync.RWMutex

	WorkerId string             `bson:"_id" json:"worker_id"`
	Health   types.WorkerHealth `bson:"health" json:"health"`
	// TODO: queues as set<string>
	Queues        []string `bson:"queues" json:"queues"`
	CurrentTask   *Task    `bson:"-"` // `bson:"current_task,omitempty" json:"current_task,omitempty"`
	ConnectedAt   UnixTime `bson:"connected_at" json:"connected_at"`
	LastHeartbeat UnixTime `bson:"last_heartbeat" json:"last_heartbeat"`
	// Rtt microseconds
	Rtt             int64                  `bson:"rtt" json:"rtt"`
	ProvisionerName string                 `bson:"provisioner_name" json:"provisioner_name"`
	ResourceInfo    map[string]interface{} `bson:"resource_info" json:"resource_info"`
	WorkerToken     string                 `bson:"worker_token" json:"-"` // Don't expose in JSON
}

func (s *WorkerSession) AssignTask(task *Task) bool {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.CurrentTask != nil {
		return false
	}
	s.CurrentTask = task
	return true
}

func (s *WorkerSession) ClearTask() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.CurrentTask = nil
}

func (s *WorkerSession) ToModel() *Worker {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	m := &Worker{
		Id:            s.WorkerId,
		Status:        s.getStatus(),
		Queues:        s.Queues,
		ConnectedAt:   s.ConnectedAt,
		LastHeartbeat: s.LastHeartbeat,
		Provisioner:   s.ProvisionerName,
		ResourceInfo:  s.ResourceInfo,
	}
	if s.CurrentTask != nil {
		m.CurrentTask = s.CurrentTask.Id
	}

	return m
}

func (s *WorkerSession) getStatus() types.WorkerStatus {
	switch {
	case s.Health == types.WorkerHealthDown:
		return types.WorkerStatusError
	case s.Health == types.WorkerHealthDisconnected:
		return types.WorkerStatusDisconnected
	case s.CurrentTask != nil:
		return types.WorkerStatusProcessing
	case s.Health == types.WorkerHealthUp:
		return types.WorkerStatusIdle
	default:
		return types.WorkerStatusError
	}
}

// NewTaskId generates a new UUIDv7 for task ID
func NewTaskId() string {
	return uuid.Must(uuid.NewV7()).String()
}

// NewWorkerId generates a new UUIDv7 for worker Id
func NewWorkerId() string {
	return uuid.Must(uuid.NewV7()).String()
}
