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

package types

import "encoding/json"

// TaskStatus represents the status of a task
type TaskStatus int

const (
	TaskStatusPending    TaskStatus = 0
	TaskStatusProcessing TaskStatus = 2
	TaskStatusCompleted  TaskStatus = 4
	TaskStatusFinished   TaskStatus = 8
)

func (v TaskStatus) String() string {
	switch v {
	case TaskStatusPending:
		return "pending"
	case TaskStatusProcessing:
		return "processing"
	case TaskStatusCompleted:
		return "completed"
	case TaskStatusFinished:
		return "finished"
	default:
		return "unknown"
	}
}

func (v TaskStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.String())
}

type WorkerHealth string

const (
	WorkerHealthUnknown      WorkerHealth = "unknown"
	WorkerHealthDisconnected WorkerHealth = "disconnected"
	WorkerHealthUp           WorkerHealth = "up"
	WorkerHealthDown         WorkerHealth = "down"
	WorkerHealthWarning      WorkerHealth = "warning"
)

// WorkerStatus represents the status of a worker
type WorkerStatus string

const (
	WorkerStatusIdle         WorkerStatus = "idle"
	WorkerStatusProcessing   WorkerStatus = "processing"
	WorkerStatusDisconnected WorkerStatus = "disconnected"
	WorkerStatusError        WorkerStatus = "error"
)

// QueueStatus represents the status of a queue
type QueueStatus string

const (
	QueueStatusActive   QueueStatus = "active"
	QueueStatusInactive QueueStatus = "inactive"
)

// HealthStatus represents the health status of a service component
type HealthStatus string

const (
	HealthStatusUp       HealthStatus = "up"
	HealthStatusDown     HealthStatus = "down"
	HealthStatusDegraded HealthStatus = "degraded"
)

// ProvisionerStatus represents the status of a provisioner
type ProvisionerStatus string

const (
	ProvisionerStatusUp   ProvisionerStatus = "up"
	ProvisionerStatusDown ProvisionerStatus = "down"
)

// TaskErrorCode represents predefined error codes for task failures
type TaskErrorCode string

const (
	TaskErrorCodeCancelled    TaskErrorCode = "CANCELLED"
	TaskErrorCodeTimeout      TaskErrorCode = "TIMEOUT"
	TaskErrorCodeWorkerError  TaskErrorCode = "WORKER_ERROR"
	TaskErrorCodeNetworkError TaskErrorCode = "NETWORK_ERROR"
	TaskErrorCodeUnknown      TaskErrorCode = "UNKNOWN"
)
