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
	"encoding/json"
	"github.com/jc-lab/distworker/go/pkg/types"
	"strings"
	"testing"
	"time"
)

func TestTaskJSONMarshaling(t *testing.T) {
	t.Run("BasicTaskWithRequiredFields", func(t *testing.T) {
		task := &Task{
			Id:        "01234567-89ab-cdef-0123-456789abcdef",
			Status:    types.TaskStatusPending,
			Queue:     "test-queue",
			Metadata:  map[string]interface{}{"priority": "high"},
			Input:     map[string]interface{}{"data": "test"},
			TimeoutMS: 30000,
			CreatedAt: NewUnixTime(time.Unix(1640995200, 0)), // 2022-01-01 00:00:00 UTC
		}

		jsonData, err := json.Marshal(task)
		if err != nil {
			t.Fatalf("Failed to marshal task: %v", err)
		}

		// Check if time is in milliseconds
		jsonStr := string(jsonData)
		if !strings.Contains(jsonStr, `"created_at":1640995200`) {
			t.Errorf("Expected created_at to be in milliseconds (1640995200), got: %s", jsonStr)
		}

		// Unmarshal and verify
		var unmarshaled Task
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal task: %v", err)
		}

		if unmarshaled.Id != task.Id {
			t.Errorf("Expected Id %s, got %s", task.Id, unmarshaled.Id)
		}
		if unmarshaled.Status != task.Status {
			t.Errorf("Expected Status %s, got %s", task.Status, unmarshaled.Status)
		}
		if unmarshaled.Queue != task.Queue {
			t.Errorf("Expected Queue %s, got %s", task.Queue, unmarshaled.Queue)
		}
		if unmarshaled.CreatedAt.Unix() != task.CreatedAt.Unix() {
			t.Errorf("Expected CreatedAt %v, got %v", task.CreatedAt.Time, unmarshaled.CreatedAt.Time)
		}
	})

	t.Run("TaskWithOptionalTimeFields", func(t *testing.T) {
		startTime := time.Unix(1640995260, 0) // 2022-01-01 00:01:00 UTC
		endTime := time.Unix(1640995320, 0)   // 2022-01-01 00:02:00 UTC

		task := &Task{
			Id:          "01234567-89ab-cdef-0123-456789abcdef",
			Status:      types.TaskStatusCompleted,
			Queue:       "test-queue",
			Metadata:    map[string]interface{}{"priority": "high"},
			Input:       map[string]interface{}{"data": "test"},
			Result:      map[string]interface{}{"output": "success"},
			TimeoutMS:   30000,
			CreatedAt:   NewUnixTime(time.Unix(1640995200, 0)),
			StartedAt:   NewUnixTimePtr(startTime),
			CompletedAt: NewUnixTimePtr(endTime),
		}

		jsonData, err := json.Marshal(task)
		if err != nil {
			t.Fatalf("Failed to marshal task: %v", err)
		}

		jsonStr := string(jsonData)

		// Check all time fields are in milliseconds
		if !strings.Contains(jsonStr, `"created_at":1640995200`) {
			t.Errorf("Expected created_at in milliseconds, got: %s", jsonStr)
		}
		if !strings.Contains(jsonStr, `"started_at":1640995260`) {
			t.Errorf("Expected started_at in milliseconds, got: %s", jsonStr)
		}
		if !strings.Contains(jsonStr, `"completed_at":1640995320`) {
			t.Errorf("Expected completed_at in milliseconds, got: %s", jsonStr)
		}

		// Unmarshal and verify
		var unmarshaled Task
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal task: %v", err)
		}

		if unmarshaled.StartedAt == nil {
			t.Error("Expected StartedAt to be set")
		} else if unmarshaled.StartedAt.Time.Unix() != startTime.Unix() {
			t.Errorf("Expected StartedAt %v, got %v", startTime, unmarshaled.StartedAt.Time)
		}

		if unmarshaled.CompletedAt == nil {
			t.Error("Expected CompletedAt to be set")
		} else if unmarshaled.CompletedAt.Time.Unix() != endTime.Unix() {
			t.Errorf("Expected CompletedAt %v, got %v", endTime, unmarshaled.CompletedAt.Time)
		}
	})

	t.Run("TaskWithNullOptionalFields", func(t *testing.T) {
		task := &Task{
			Id:        "01234567-89ab-cdef-0123-456789abcdef",
			Status:    types.TaskStatusPending,
			Queue:     "test-queue",
			Metadata:  map[string]interface{}{"priority": "low"},
			Input:     map[string]interface{}{"data": "test"},
			TimeoutMS: 30000,
			CreatedAt: NewUnixTime(time.Unix(1640995200, 0)),
			// StartedAt and CompletedAt are nil
		}

		jsonData, err := json.Marshal(task)
		if err != nil {
			t.Fatalf("Failed to marshal task: %v", err)
		}

		jsonStr := string(jsonData)
		if !strings.Contains(jsonStr, `"started_at":null`) {
			t.Errorf("Expected started_at to be null, got: %s", jsonStr)
		}
		if !strings.Contains(jsonStr, `"completed_at":null`) {
			t.Errorf("Expected completed_at to be null, got: %s", jsonStr)
		}

		// Unmarshal and verify
		var unmarshaled Task
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal task: %v", err)
		}

		if unmarshaled.StartedAt != nil {
			t.Error("Expected StartedAt to be nil")
		}
		if unmarshaled.CompletedAt != nil {
			t.Error("Expected CompletedAt to be nil")
		}
	})

	t.Run("TaskWithError", func(t *testing.T) {
		task := &Task{
			Id:       "01234567-89ab-cdef-0123-456789abcdef",
			Status:   types.TaskStatusFailed,
			Queue:    "test-queue",
			Metadata: map[string]interface{}{"priority": "high"},
			Input:    map[string]interface{}{"data": "test"},
			Error: &TaskError{
				Code:    types.TaskErrorCodeTimeout,
				Message: "Task timed out",
				Details: map[string]interface{}{"timeout_ms": 30000},
			},
			TimeoutMS: 30000,
			CreatedAt: NewUnixTime(time.Unix(1640995200, 0)),
		}

		jsonData, err := json.Marshal(task)
		if err != nil {
			t.Fatalf("Failed to marshal task: %v", err)
		}

		// Unmarshal and verify
		var unmarshaled Task
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal task: %v", err)
		}

		if unmarshaled.Error == nil {
			t.Error("Expected Error to be set")
		} else {
			if unmarshaled.Error.Code != types.TaskErrorCodeTimeout {
				t.Errorf("Expected Error.Code %s, got %s", types.TaskErrorCodeTimeout, unmarshaled.Error.Code)
			}
			if unmarshaled.Error.Message != "Task timed out" {
				t.Errorf("Expected Error.Message 'Task timed out', got %s", unmarshaled.Error.Message)
			}
		}
	})

	t.Run("TaskWithFiles", func(t *testing.T) {
		task := &Task{
			Id:       "01234567-89ab-cdef-0123-456789abcdef",
			Status:   types.TaskStatusCompleted,
			Queue:    "test-queue",
			Metadata: map[string]interface{}{"priority": "high"},
			Input:    map[string]interface{}{"data": "test"},
			Files: []FileInfo{
				{
					FileId:      "file-001",
					Filename:    "input.txt",
					ContentType: "text/plain",
					Size:        1024,
					StorageURL:  "s3://bucket/file-001",
				},
			},
			ResultFiles: []FileInfo{
				{
					FileId:      "file-002",
					Filename:    "output.txt",
					ContentType: "text/plain",
					Size:        2048,
					StorageURL:  "s3://bucket/file-002",
				},
			},
			TimeoutMS: 30000,
			CreatedAt: NewUnixTime(time.Unix(1640995200, 0)),
		}

		jsonData, err := json.Marshal(task)
		if err != nil {
			t.Fatalf("Failed to marshal task: %v", err)
		}

		// Unmarshal and verify
		var unmarshaled Task
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal task: %v", err)
		}

		if len(unmarshaled.Files) != 1 {
			t.Errorf("Expected 1 file, got %d", len(unmarshaled.Files))
		} else {
			file := unmarshaled.Files[0]
			if file.FileId != "file-001" {
				t.Errorf("Expected FileId 'file-001', got %s", file.FileId)
			}
			if file.Size != 1024 {
				t.Errorf("Expected Size 1024, got %d", file.Size)
			}
		}

		if len(unmarshaled.ResultFiles) != 1 {
			t.Errorf("Expected 1 result file, got %d", len(unmarshaled.ResultFiles))
		}
	})
}

func TestQueueJSONMarshaling(t *testing.T) {
	t.Run("BasicQueue", func(t *testing.T) {
		queue := &Queue{
			Name:        "test-queue",
			Description: "Test queue for processing",
			Status:      types.QueueStatusActive,
			CreatedAt:   NewUnixTime(time.Unix(1640995200, 0)),
			UpdatedAt:   NewUnixTime(time.Unix(1640995260, 0)),
		}

		jsonData, err := json.Marshal(queue)
		if err != nil {
			t.Fatalf("Failed to marshal queue: %v", err)
		}

		jsonStr := string(jsonData)
		if !strings.Contains(jsonStr, `"created_at":1640995200`) {
			t.Errorf("Expected created_at in milliseconds, got: %s", jsonStr)
		}
		if !strings.Contains(jsonStr, `"updated_at":1640995260`) {
			t.Errorf("Expected updated_at in milliseconds, got: %s", jsonStr)
		}

		// Unmarshal and verify
		var unmarshaled Queue
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal queue: %v", err)
		}

		if unmarshaled.Name != queue.Name {
			t.Errorf("Expected Name %s, got %s", queue.Name, unmarshaled.Name)
		}
		if unmarshaled.Status != queue.Status {
			t.Errorf("Expected Status %s, got %s", queue.Status, unmarshaled.Status)
		}
		if unmarshaled.CreatedAt.Unix() != queue.CreatedAt.Unix() {
			t.Errorf("Expected CreatedAt %v, got %v", queue.CreatedAt.Time, unmarshaled.CreatedAt.Time)
		}
	})
}

func TestWorkerJSONMarshaling(t *testing.T) {
	t.Run("BasicWorker", func(t *testing.T) {
		worker := &Worker{
			Id:            "worker-001",
			Status:        types.WorkerStatusIdle,
			Queues:        []string{"test-*", "prod-#"},
			CurrentTask:   "",
			ConnectedAt:   NewUnixTime(time.Unix(1640995200, 0)),
			LastHeartbeat: NewUnixTime(time.Unix(1640995260, 0)),
			Provisioner:   "test-provisioner",
			ResourceInfo:  map[string]interface{}{"cpu": 4, "memory": "8GB"},
		}

		jsonData, err := json.Marshal(worker)
		if err != nil {
			t.Fatalf("Failed to marshal worker: %v", err)
		}

		jsonStr := string(jsonData)
		if !strings.Contains(jsonStr, `"connected_at":1640995200`) {
			t.Errorf("Expected connected_at in milliseconds, got: %s", jsonStr)
		}
		if !strings.Contains(jsonStr, `"last_heartbeat":1640995260`) {
			t.Errorf("Expected last_heartbeat in milliseconds, got: %s", jsonStr)
		}

		// Unmarshal and verify
		var unmarshaled Worker
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal worker: %v", err)
		}

		if unmarshaled.Id != worker.Id {
			t.Errorf("Expected Id %s, got %s", worker.Id, unmarshaled.Id)
		}
		if len(unmarshaled.Queues) != 2 {
			t.Errorf("Expected 2 queues, got %d", len(unmarshaled.Queues))
		}
	})
}

func TestWorkerSessionJSONMarshaling(t *testing.T) {
	t.Run("BasicWorkerSession", func(t *testing.T) {
		session := &WorkerSession{
			WorkerId:        "worker-001",
			Status:          types.WorkerStatusProcessing,
			Queues:          []string{"test-*"},
			CurrentTask:     "task-123",
			ConnectedAt:     NewUnixTime(time.Unix(1640995200, 0)),
			LastHeartbeat:   NewUnixTime(time.Unix(1640995260, 0)),
			ProvisionerName: "test-provisioner",
			ResourceInfo:    map[string]interface{}{"cpu": 4},
			WorkerToken:     "secret-token", // Should not appear in JSON
		}

		jsonData, err := json.Marshal(session)
		if err != nil {
			t.Fatalf("Failed to marshal worker session: %v", err)
		}

		jsonStr := string(jsonData)

		// Check that WorkerToken is not exposed
		if strings.Contains(jsonStr, "secret-token") {
			t.Error("WorkerToken should not be exposed in JSON")
		}

		// Check time fields
		if !strings.Contains(jsonStr, `"connected_at":1640995200`) {
			t.Errorf("Expected connected_at in milliseconds, got: %s", jsonStr)
		}

		// Unmarshal and verify
		var unmarshaled WorkerSession
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal worker session: %v", err)
		}

		if unmarshaled.WorkerId != session.WorkerId {
			t.Errorf("Expected WorkerId %s, got %s", session.WorkerId, unmarshaled.WorkerId)
		}
		if unmarshaled.WorkerToken != "" {
			t.Error("WorkerToken should not be unmarshaled from JSON")
		}
	})
}

func TestProvisionerJSONMarshaling(t *testing.T) {
	t.Run("BasicProvisioner", func(t *testing.T) {
		provisioner := &Provisioner{
			Name:          "test-provisioner",
			URL:           "https://provisioner.example.com",
			Status:        types.ProvisionerStatusUp,
			Workers:       5,
			MaxWorkers:    10,
			LastHeartbeat: NewUnixTime(time.Unix(1640995200, 0)),
		}

		jsonData, err := json.Marshal(provisioner)
		if err != nil {
			t.Fatalf("Failed to marshal provisioner: %v", err)
		}

		jsonStr := string(jsonData)
		if !strings.Contains(jsonStr, `"last_heartbeat":1640995200`) {
			t.Errorf("Expected last_heartbeat in milliseconds, got: %s", jsonStr)
		}

		// Unmarshal and verify
		var unmarshaled Provisioner
		err = json.Unmarshal(jsonData, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal provisioner: %v", err)
		}

		if unmarshaled.Name != provisioner.Name {
			t.Errorf("Expected Name %s, got %s", provisioner.Name, unmarshaled.Name)
		}
		if unmarshaled.Workers != provisioner.Workers {
			t.Errorf("Expected Workers %d, got %d", provisioner.Workers, unmarshaled.Workers)
		}
	})
}

func TestUnixTimeEdgeCases(t *testing.T) {
	t.Run("ZeroTime", func(t *testing.T) {
		zeroTime := UnixTime{}

		jsonData, err := json.Marshal(zeroTime)
		if err != nil {
			t.Fatalf("Failed to marshal zero time: %v", err)
		}

		if string(jsonData) != "null" {
			t.Errorf("Expected zero time to marshal as null, got: %s", string(jsonData))
		}

		// Test unmarshaling null
		var unmarshaled UnixTime
		err = json.Unmarshal([]byte("null"), &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal null time: %v", err)
		}
	})

	t.Run("NilPointerTime", func(t *testing.T) {
		var nilTime *UnixTime = nil

		jsonData, err := json.Marshal(nilTime)
		if err != nil {
			t.Fatalf("Failed to marshal nil time pointer: %v", err)
		}

		if string(jsonData) != "null" {
			t.Errorf("Expected nil pointer to marshal as null, got: %s", string(jsonData))
		}
	})
}

func TestComplexTaskJSONRoundTrip(t *testing.T) {
	// Create a complex task with all fields populated
	startTime := time.Unix(1640995260, 500000000) // with nanoseconds
	endTime := time.Unix(1640995320, 750000000)   // with nanoseconds

	originalTask := &Task{
		Id:     "01HN2Z8K6M7QJ9P5R3S6V8W2X4",
		Status: types.TaskStatusCompleted,
		Queue:  "ml-training-queue",
		Metadata: map[string]interface{}{
			"priority":   "high",
			"retries":    3,
			"tags":       []string{"ml", "training", "gpu"},
			"user_id":    12345,
			"model_type": "transformer",
		},
		Input: map[string]interface{}{
			"dataset_path":  "/data/training/dataset.csv",
			"epochs":        100,
			"batch_size":    32,
			"learning_rate": 0.001,
		},
		Result: map[string]interface{}{
			"accuracy":      0.95,
			"loss":          0.05,
			"model_path":    "/models/trained_model.pkl",
			"training_time": "2h15m30s",
		},
		Files: []FileInfo{
			{
				FileId:      "file-input-001",
				Filename:    "training_data.csv",
				ContentType: "text/csv",
				Size:        1048576, // 1MB
				StorageURL:  "s3://ml-bucket/datasets/training_data.csv",
			},
			{
				FileId:      "file-input-002",
				Filename:    "config.json",
				ContentType: "application/json",
				Size:        2048, // 2KB
				StorageURL:  "s3://ml-bucket/configs/config.json",
			},
		},
		ResultFiles: []FileInfo{
			{
				FileId:      "file-output-001",
				Filename:    "trained_model.pkl",
				ContentType: "application/octet-stream",
				Size:        10485760, // 10MB
				StorageURL:  "s3://ml-bucket/models/trained_model.pkl",
			},
			{
				FileId:      "file-output-002",
				Filename:    "metrics.json",
				ContentType: "application/json",
				Size:        4096, // 4KB
				StorageURL:  "s3://ml-bucket/metrics/metrics.json",
			},
		},
		WorkerId:    "worker-gpu-001",
		TimeoutMS:   7200000, // 2 hours
		CreatedAt:   NewUnixTime(time.Unix(1640995200, 123000000)),
		StartedAt:   NewUnixTimePtr(startTime),
		CompletedAt: NewUnixTimePtr(endTime),
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(originalTask)
	if err != nil {
		t.Fatalf("Failed to marshal complex task: %v", err)
	}

	// Verify JSON structure
	jsonStr := string(jsonData)
	t.Logf("Marshaled JSON: %s", jsonStr)

	// Check that all time fields are in milliseconds
	if !strings.Contains(jsonStr, `"created_at":1640995200`) {
		t.Error("created_at should be in seconds")
	}
	if !strings.Contains(jsonStr, `"started_at":1640995260`) {
		t.Error("started_at should be in seconds")
	}
	if !strings.Contains(jsonStr, `"completed_at":1640995320`) {
		t.Error("completed_at should be in seconds")
	}

	// Unmarshal back to Task
	var unmarshaledTask Task
	err = json.Unmarshal(jsonData, &unmarshaledTask)
	if err != nil {
		t.Fatalf("Failed to unmarshal complex task: %v", err)
	}

	// Verify all fields
	if unmarshaledTask.Id != originalTask.Id {
		t.Errorf("Id mismatch: expected %s, got %s", originalTask.Id, unmarshaledTask.Id)
	}

	if unmarshaledTask.Status != originalTask.Status {
		t.Errorf("Status mismatch: expected %s, got %s", originalTask.Status, unmarshaledTask.Status)
	}

	// Check metadata
	if len(unmarshaledTask.Metadata) != len(originalTask.Metadata) {
		t.Errorf("Metadata length mismatch: expected %d, got %d", len(originalTask.Metadata), len(unmarshaledTask.Metadata))
	}

	// Check files
	if len(unmarshaledTask.Files) != len(originalTask.Files) {
		t.Errorf("Files length mismatch: expected %d, got %d", len(originalTask.Files), len(unmarshaledTask.Files))
	}

	if len(unmarshaledTask.ResultFiles) != len(originalTask.ResultFiles) {
		t.Errorf("ResultFiles length mismatch: expected %d, got %d", len(originalTask.ResultFiles), len(unmarshaledTask.ResultFiles))
	}

	// Check time precision (should be accurate to millisecond)
	if unmarshaledTask.CreatedAt.Unix() != originalTask.CreatedAt.Unix() {
		t.Errorf("CreatedAt mismatch: expected %d, got %d",
			originalTask.CreatedAt.Unix(), unmarshaledTask.CreatedAt.Unix())
	}

	if unmarshaledTask.StartedAt.Time.Unix() != originalTask.StartedAt.Time.Unix() {
		t.Errorf("StartedAt mismatch: expected %d, got %d",
			originalTask.StartedAt.Time.Unix(), unmarshaledTask.StartedAt.Time.Unix())
	}

	if unmarshaledTask.CompletedAt.Time.Unix() != originalTask.CompletedAt.Time.Unix() {
		t.Errorf("CompletedAt mismatch: expected %d, got %d",
			originalTask.CompletedAt.Time.Unix(), unmarshaledTask.CompletedAt.Time.Unix())
	}
}
