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

package database_test

import (
	"context"
	"github.com/jc-lab/distworker/go/internal/testing/database_mock"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	"github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"testing"
	"time"
)

func TestMockDatabase(t *testing.T) {
	t.Run("DatabaseInterface", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(context.Background())

		// Test Health
		err := db.Health(context.Background())
		if err != nil {
			t.Errorf("Expected healthy database, got error: %v", err)
		}

		// Test repositories
		taskRepo := db.GetTaskRepository()
		if taskRepo == nil {
			t.Error("Expected non-nil task repository")
		}

		queueRepo := db.GetQueueRepository()
		if queueRepo == nil {
			t.Error("Expected non-nil queue repository")
		}

		workerRepo := db.GetWorkerSessionRepository()
		if workerRepo == nil {
			t.Error("Expected non-nil worker session repository")
		}

		// Test Close
		err = db.Close(context.Background())
		if err != nil {
			t.Errorf("Expected successful close, got error: %v", err)
		}

		// Health should fail after close
		err = db.Health(context.Background())
		if err == nil {
			t.Error("Expected health check to fail after close")
		}
	})

	t.Run("HealthError", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(context.Background())

		// Set health error
		testError := &mockError{"test health error"}
		db.SetHealthError(testError)

		err := db.Health(context.Background())
		if err != testError {
			t.Errorf("Expected specific health error, got: %v", err)
		}
	})
}

func TestTaskRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateTask", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		task := &models.Task{
			Id:        "task-123",
			Status:    types.TaskStatusPending,
			Queue:     "test-queue",
			Metadata:  map[string]interface{}{"priority": "high"},
			Input:     map[string]interface{}{"data": "test"},
			TimeoutMS: 60000,
		}

		err := repo.Create(ctx, task)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		// Verify task exists
		if !db.TaskExists("task-123") {
			t.Error("Expected task to exist after creation")
		}

		// Verify task count
		if db.CountTasks() != 1 {
			t.Errorf("Expected 1 task, got: %d", db.CountTasks())
		}
	})

	t.Run("CreateDuplicateTask", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		task := &models.Task{
			Id:     "task-duplicate",
			Status: types.TaskStatusPending,
			Queue:  "test-queue",
		}

		// Create first task
		err := repo.Create(ctx, task)
		if err != nil {
			t.Fatalf("Failed to create first task: %v", err)
		}

		// Try to create duplicate
		err = repo.Create(ctx, task)
		if err == nil {
			t.Error("Expected error when creating duplicate task")
		}
	})

	t.Run("CreateTaskWithoutId", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		task := &models.Task{
			Status: types.TaskStatusPending,
			Queue:  "test-queue",
			// Missing Id
		}

		err := repo.Create(ctx, task)
		if err == nil {
			t.Error("Expected error when creating task without ID")
		}
	})

	t.Run("GetTaskById", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		// Create a task first
		originalTask := &models.Task{
			Id:        "task-get-test",
			Status:    types.TaskStatusPending,
			Queue:     "test-queue",
			Metadata:  map[string]interface{}{"key": "value"},
			TimeoutMS: 30000,
		}

		err := repo.Create(ctx, originalTask)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		// Get the task
		retrievedTask, err := repo.GetById(ctx, "task-get-test")
		if err != nil {
			t.Fatalf("Failed to get task: %v", err)
		}

		// Verify task data
		if retrievedTask.Id != originalTask.Id {
			t.Errorf("Expected task ID %s, got: %s", originalTask.Id, retrievedTask.Id)
		}
		if retrievedTask.Status != originalTask.Status {
			t.Errorf("Expected status %s, got: %s", originalTask.Status, retrievedTask.Status)
		}
		if retrievedTask.Queue != originalTask.Queue {
			t.Errorf("Expected queue %s, got: %s", originalTask.Queue, retrievedTask.Queue)
		}
		if retrievedTask.TimeoutMS != originalTask.TimeoutMS {
			t.Errorf("Expected timeout %d, got: %d", originalTask.TimeoutMS, retrievedTask.TimeoutMS)
		}
	})

	t.Run("GetNonexistentTask", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		_, err := repo.GetById(ctx, "nonexistent-task")
		if err == nil {
			t.Error("Expected error when getting nonexistent task")
		}
	})

	t.Run("UpdateTask", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		// Create a task first
		task := &models.Task{
			Id:     "task-update-test",
			Status: types.TaskStatusPending,
			Queue:  "test-queue",
		}

		err := repo.Create(ctx, task)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		// Update the task
		task.Status = types.TaskStatusProcessing
		task.WorkerId = "worker-123"
		task.StartedAt = models.NowPtr()

		err = repo.Update(ctx, task)
		if err != nil {
			t.Fatalf("Failed to update task: %v", err)
		}

		// Verify update
		retrievedTask, err := repo.GetById(ctx, "task-update-test")
		if err != nil {
			t.Fatalf("Failed to retrieve updated task: %v", err)
		}

		if retrievedTask.Status != types.TaskStatusProcessing {
			t.Errorf("Expected status %s, got: %s", types.TaskStatusProcessing, retrievedTask.Status)
		}
		if retrievedTask.WorkerId != "worker-123" {
			t.Errorf("Expected worker ID 'worker-123', got: %s", retrievedTask.WorkerId)
		}
		if retrievedTask.StartedAt == nil {
			t.Error("Expected StartedAt to be set")
		}
	})

	t.Run("UpdateNonexistentTask", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		task := &models.Task{
			Id:     "nonexistent-task",
			Status: types.TaskStatusProcessing,
		}

		err := repo.Update(ctx, task)
		if err == nil {
			t.Error("Expected error when updating nonexistent task")
		}
	})

	t.Run("DeleteTask", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		// Create a task first
		task := &models.Task{
			Id:     "task-delete-test",
			Status: types.TaskStatusPending,
			Queue:  "test-queue",
		}

		err := repo.Create(ctx, task)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		// Verify task exists
		if !db.TaskExists("task-delete-test") {
			t.Error("Expected task to exist before deletion")
		}

		// Delete the task
		err = repo.Delete(ctx, "task-delete-test")
		if err != nil {
			t.Fatalf("Failed to delete task: %v", err)
		}

		// Verify task no longer exists
		if db.TaskExists("task-delete-test") {
			t.Error("Expected task to not exist after deletion")
		}
	})

	t.Run("DeleteNonexistentTask", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		err := repo.Delete(ctx, "nonexistent-task")
		if err == nil {
			t.Error("Expected error when deleting nonexistent task")
		}
	})

	t.Run("ListTasks", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		// Create multiple tasks
		tasks := []*models.Task{
			{Id: "task-1", Status: types.TaskStatusPending, Queue: "queue-a"},
			{Id: "task-2", Status: types.TaskStatusProcessing, Queue: "queue-a", WorkerId: "worker-1"},
			{Id: "task-3", Status: types.TaskStatusCompleted, Queue: "queue-b", WorkerId: "worker-2"},
			{Id: "task-4", Status: types.TaskStatusPending, Queue: "queue-b"},
		}

		for _, task := range tasks {
			err := repo.Create(ctx, task)
			if err != nil {
				t.Fatalf("Failed to create task %s: %v", task.Id, err)
			}
		}

		// Test list all tasks
		allTasks, total, err := repo.List(ctx, database.TaskFilter{}, 1, 10)
		if err != nil {
			t.Fatalf("Failed to list all tasks: %v", err)
		}
		if len(allTasks) != 4 {
			t.Errorf("Expected 4 tasks, got: %d", len(allTasks))
		}
		if total != 4 {
			t.Errorf("Expected total 4, got: %d", total)
		}

		// Test filter by queue
		queueATasks, total, err := repo.List(ctx, database.TaskFilter{Queue: "queue-a"}, 1, 10)
		if err != nil {
			t.Fatalf("Failed to list queue-a tasks: %v", err)
		}
		if len(queueATasks) != 2 {
			t.Errorf("Expected 2 queue-a tasks, got: %d", len(queueATasks))
		}
		if total != 2 {
			t.Errorf("Expected total 2 for queue-a, got: %d", total)
		}

		// Test filter by status
		pendingTasks, total, err := repo.List(ctx, database.TaskFilter{Status: string(types.TaskStatusPending)}, 1, 10)
		if err != nil {
			t.Fatalf("Failed to list pending tasks: %v", err)
		}
		if len(pendingTasks) != 2 {
			t.Errorf("Expected 2 pending tasks, got: %d", len(pendingTasks))
		}

		// Test filter by worker
		worker2Tasks, total, err := repo.List(ctx, database.TaskFilter{WorkerId: "worker-2"}, 1, 10)
		if err != nil {
			t.Fatalf("Failed to list worker-2 tasks: %v", err)
		}
		if len(worker2Tasks) != 1 {
			t.Errorf("Expected 1 worker-2 task, got: %d", len(worker2Tasks))
		}

		// Test pagination
		page1Tasks, total, err := repo.List(ctx, database.TaskFilter{}, 1, 2)
		if err != nil {
			t.Fatalf("Failed to list page 1: %v", err)
		}
		if len(page1Tasks) != 2 {
			t.Errorf("Expected 2 tasks in page 1, got: %d", len(page1Tasks))
		}
		if total != 4 {
			t.Errorf("Expected total 4 in pagination, got: %d", total)
		}

		page2Tasks, _, err := repo.List(ctx, database.TaskFilter{}, 2, 2)
		if err != nil {
			t.Fatalf("Failed to list page 2: %v", err)
		}
		if len(page2Tasks) != 2 {
			t.Errorf("Expected 2 tasks in page 2, got: %d", len(page2Tasks))
		}
	})

	t.Run("GetPendingTasks", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetTaskRepository()

		// Create tasks with different statuses
		tasks := []*models.Task{
			{Id: "pending-1", Status: types.TaskStatusPending, Queue: "queue-a"},
			{Id: "pending-2", Status: types.TaskStatusPending, Queue: "queue-a"},
			{Id: "pending-3", Status: types.TaskStatusPending, Queue: "queue-b"},
			{Id: "processing-1", Status: types.TaskStatusProcessing, Queue: "queue-a"},
			{Id: "completed-1", Status: types.TaskStatusCompleted, Queue: "queue-a"},
		}

		for _, task := range tasks {
			err := repo.Create(ctx, task)
			if err != nil {
				t.Fatalf("Failed to create task %s: %v", task.Id, err)
			}
		}

		// Get pending tasks for queue-a
		pendingTasks, err := repo.GetPendingTasks(ctx, "queue-a", 10)
		if err != nil {
			t.Fatalf("Failed to get pending tasks: %v", err)
		}
		if len(pendingTasks) != 2 {
			t.Errorf("Expected 2 pending tasks for queue-a, got: %d", len(pendingTasks))
		}

		// Get pending tasks for all queues
		allPendingTasks, err := repo.GetPendingTasks(ctx, "", 10)
		if err != nil {
			t.Fatalf("Failed to get all pending tasks: %v", err)
		}
		if len(allPendingTasks) != 3 {
			t.Errorf("Expected 3 pending tasks total, got: %d", len(allPendingTasks))
		}

		// Test limit
		limitedTasks, err := repo.GetPendingTasks(ctx, "", 2)
		if err != nil {
			t.Fatalf("Failed to get limited pending tasks: %v", err)
		}
		if len(limitedTasks) != 2 {
			t.Errorf("Expected 2 limited pending tasks, got: %d", len(limitedTasks))
		}
	})
}

func TestQueueRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateQueue", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetQueueRepository()

		queue := &models.Queue{
			Name:        "test-queue",
			Description: "Test queue description",
			Status:      types.QueueStatusActive,
		}

		err := repo.Create(ctx, queue)
		if err != nil {
			t.Fatalf("Failed to create queue: %v", err)
		}

		// Verify queue exists
		if !db.QueueExists("test-queue") {
			t.Error("Expected queue to exist after creation")
		}

		// Verify queue count
		if db.CountQueues() != 1 {
			t.Errorf("Expected 1 queue, got: %d", db.CountQueues())
		}
	})

	t.Run("CreateDuplicateQueue", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetQueueRepository()

		queue := &models.Queue{
			Name:   "duplicate-queue",
			Status: types.QueueStatusActive,
		}

		// Create first queue
		err := repo.Create(ctx, queue)
		if err != nil {
			t.Fatalf("Failed to create first queue: %v", err)
		}

		// Try to create duplicate
		err = repo.Create(ctx, queue)
		if err == nil {
			t.Error("Expected error when creating duplicate queue")
		}
	})

	t.Run("GetQueueByName", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetQueueRepository()

		// Create a queue first
		originalQueue := &models.Queue{
			Name:        "test-get-queue",
			Description: "Test description",
			Status:      types.QueueStatusActive,
		}

		err := repo.Create(ctx, originalQueue)
		if err != nil {
			t.Fatalf("Failed to create queue: %v", err)
		}

		// Get the queue
		retrievedQueue, err := repo.GetByName(ctx, "test-get-queue")
		if err != nil {
			t.Fatalf("Failed to get queue: %v", err)
		}

		// Verify queue data
		if retrievedQueue.Name != originalQueue.Name {
			t.Errorf("Expected queue name %s, got: %s", originalQueue.Name, retrievedQueue.Name)
		}
		if retrievedQueue.Description != originalQueue.Description {
			t.Errorf("Expected description %s, got: %s", originalQueue.Description, retrievedQueue.Description)
		}
		if retrievedQueue.Status != originalQueue.Status {
			t.Errorf("Expected status %s, got: %s", originalQueue.Status, retrievedQueue.Status)
		}
	})

	t.Run("GetNonexistentQueue", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetQueueRepository()

		_, err := repo.GetByName(ctx, "nonexistent-queue")
		if err == nil {
			t.Error("Expected error when getting nonexistent queue")
		}
	})

	t.Run("UpdateQueue", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetQueueRepository()

		// Create a queue first
		queue := &models.Queue{
			Name:        "test-update-queue",
			Description: "Original description",
			Status:      types.QueueStatusActive,
		}

		err := repo.Create(ctx, queue)
		if err != nil {
			t.Fatalf("Failed to create queue: %v", err)
		}

		// Update the queue
		queue.Description = "Updated description"
		queue.Status = types.QueueStatusInactive

		err = repo.Update(ctx, queue)
		if err != nil {
			t.Fatalf("Failed to update queue: %v", err)
		}

		// Verify update
		retrievedQueue, err := repo.GetByName(ctx, "test-update-queue")
		if err != nil {
			t.Fatalf("Failed to retrieve updated queue: %v", err)
		}

		if retrievedQueue.Description != "Updated description" {
			t.Errorf("Expected description 'Updated description', got: %s", retrievedQueue.Description)
		}
		if retrievedQueue.Status != types.QueueStatusInactive {
			t.Errorf("Expected status %s, got: %s", types.QueueStatusInactive, retrievedQueue.Status)
		}
	})

	t.Run("DeleteQueue", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetQueueRepository()

		// Create a queue first
		queue := &models.Queue{
			Name:   "test-delete-queue",
			Status: types.QueueStatusActive,
		}

		err := repo.Create(ctx, queue)
		if err != nil {
			t.Fatalf("Failed to create queue: %v", err)
		}

		// Verify queue exists
		if !db.QueueExists("test-delete-queue") {
			t.Error("Expected queue to exist before deletion")
		}

		// Delete the queue
		err = repo.Delete(ctx, "test-delete-queue")
		if err != nil {
			t.Fatalf("Failed to delete queue: %v", err)
		}

		// Verify queue no longer exists
		if db.QueueExists("test-delete-queue") {
			t.Error("Expected queue to not exist after deletion")
		}
	})

	t.Run("ListQueues", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetQueueRepository()

		// Create multiple queues
		queues := []*models.Queue{
			{Name: "queue-1", Status: types.QueueStatusActive},
			{Name: "queue-2", Status: types.QueueStatusInactive},
			{Name: "queue-3", Status: types.QueueStatusActive},
		}

		for _, queue := range queues {
			err := repo.Create(ctx, queue)
			if err != nil {
				t.Fatalf("Failed to create queue %s: %v", queue.Name, err)
			}
		}

		// List all queues
		allQueues, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("Failed to list queues: %v", err)
		}
		if len(allQueues) != 3 {
			t.Errorf("Expected 3 queues, got: %d", len(allQueues))
		}
	})
}

func TestWorkerSessionRepository(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateWorkerSession", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetWorkerSessionRepository()

		session := &models.WorkerSession{
			WorkerId:        "worker-123",
			Status:          types.WorkerStatusIdle,
			Queues:          []string{"queue-*", "test/#"},
			ProvisionerName: "test-provisioner",
			ResourceInfo:    map[string]interface{}{"cpu": 4, "memory": "8GB"},
			WorkerToken:     "secret-token",
		}

		err := repo.Create(ctx, session)
		if err != nil {
			t.Fatalf("Failed to create worker session: %v", err)
		}

		// Verify session exists
		if !db.WorkerSessionExists("worker-123") {
			t.Error("Expected worker session to exist after creation")
		}

		// Verify session count
		if db.CountWorkerSessions() != 1 {
			t.Errorf("Expected 1 worker session, got: %d", db.CountWorkerSessions())
		}
	})

	t.Run("GetWorkerSessionById", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetWorkerSessionRepository()

		// Create a session first
		originalSession := &models.WorkerSession{
			WorkerId:        "worker-get-test",
			Status:          types.WorkerStatusIdle,
			Queues:          []string{"test/*"},
			ProvisionerName: "test-provisioner",
			WorkerToken:     "secret-token",
		}

		err := repo.Create(ctx, originalSession)
		if err != nil {
			t.Fatalf("Failed to create worker session: %v", err)
		}

		// Get the session
		retrievedSession, err := repo.GetById(ctx, "worker-get-test")
		if err != nil {
			t.Fatalf("Failed to get worker session: %v", err)
		}

		// Verify session data
		if retrievedSession.WorkerId != originalSession.WorkerId {
			t.Errorf("Expected worker ID %s, got: %s", originalSession.WorkerId, retrievedSession.WorkerId)
		}
		if retrievedSession.Status != originalSession.Status {
			t.Errorf("Expected status %s, got: %s", originalSession.Status, retrievedSession.Status)
		}
		if len(retrievedSession.Queues) != len(originalSession.Queues) {
			t.Errorf("Expected %d queues, got: %d", len(originalSession.Queues), len(retrievedSession.Queues))
		}
	})

	t.Run("UpdateWorkerSession", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetWorkerSessionRepository()

		// Create a session first
		session := &models.WorkerSession{
			WorkerId: "worker-update-test",
			Status:   types.WorkerStatusIdle,
			Queues:   []string{"test/*"},
		}

		err := repo.Create(ctx, session)
		if err != nil {
			t.Fatalf("Failed to create worker session: %v", err)
		}

		// Update the session
		session.Status = types.WorkerStatusProcessing
		session.CurrentTask = "task-123"

		err = repo.Update(ctx, session)
		if err != nil {
			t.Fatalf("Failed to update worker session: %v", err)
		}

		// Verify update
		retrievedSession, err := repo.GetById(ctx, "worker-update-test")
		if err != nil {
			t.Fatalf("Failed to retrieve updated worker session: %v", err)
		}

		if retrievedSession.Status != types.WorkerStatusProcessing {
			t.Errorf("Expected status %s, got: %s", types.WorkerStatusProcessing, retrievedSession.Status)
		}
		if retrievedSession.CurrentTask != "task-123" {
			t.Errorf("Expected current task 'task-123', got: %s", retrievedSession.CurrentTask)
		}
	})

	t.Run("UpdateHeartbeat", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetWorkerSessionRepository()

		// Create a session first
		session := &models.WorkerSession{
			WorkerId: "worker-heartbeat-test",
			Status:   types.WorkerStatusIdle,
		}

		err := repo.Create(ctx, session)
		if err != nil {
			t.Fatalf("Failed to create worker session: %v", err)
		}

		// Get original heartbeat time
		originalSession, err := repo.GetById(ctx, "worker-heartbeat-test")
		if err != nil {
			t.Fatalf("Failed to get original session: %v", err)
		}

		// Wait a bit to ensure time difference
		time.Sleep(10 * time.Millisecond)

		// Update heartbeat
		err = repo.UpdateHeartbeat(ctx, "worker-heartbeat-test", types.WorkerStatusProcessing)
		if err != nil {
			t.Fatalf("Failed to update heartbeat: %v", err)
		}

		// Verify heartbeat update
		updatedSession, err := repo.GetById(ctx, "worker-heartbeat-test")
		if err != nil {
			t.Fatalf("Failed to get updated session: %v", err)
		}

		if updatedSession.Status != types.WorkerStatusProcessing {
			t.Errorf("Expected status %s, got: %s", types.WorkerStatusProcessing, updatedSession.Status)
		}
		if !updatedSession.LastHeartbeat.After(originalSession.LastHeartbeat.Time) {
			t.Error("Expected LastHeartbeat to be updated")
		}
	})

	t.Run("ListWorkerSessions", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetWorkerSessionRepository()

		// Create multiple sessions
		sessions := []*models.WorkerSession{
			{WorkerId: "worker-1", Status: types.WorkerStatusIdle},
			{WorkerId: "worker-2", Status: types.WorkerStatusProcessing},
			{WorkerId: "worker-3", Status: types.WorkerStatusError},
		}

		for _, session := range sessions {
			err := repo.Create(ctx, session)
			if err != nil {
				t.Fatalf("Failed to create worker session %s: %v", session.WorkerId, err)
			}
		}

		// List all sessions
		allSessions, err := repo.List(ctx)
		if err != nil {
			t.Fatalf("Failed to list worker sessions: %v", err)
		}
		if len(allSessions) != 3 {
			t.Errorf("Expected 3 worker sessions, got: %d", len(allSessions))
		}
	})

	t.Run("DeleteWorkerSession", func(t *testing.T) {
		db := database_mock.NewMockDatabase()
		defer db.Close(ctx)
		repo := db.GetWorkerSessionRepository()

		// Create a session first
		session := &models.WorkerSession{
			WorkerId: "worker-delete-test",
			Status:   types.WorkerStatusIdle,
		}

		err := repo.Create(ctx, session)
		if err != nil {
			t.Fatalf("Failed to create worker session: %v", err)
		}

		// Verify session exists
		if !db.WorkerSessionExists("worker-delete-test") {
			t.Error("Expected worker session to exist before deletion")
		}

		// Delete the session
		err = repo.Delete(ctx, "worker-delete-test")
		if err != nil {
			t.Fatalf("Failed to delete worker session: %v", err)
		}

		// Verify session no longer exists
		if db.WorkerSessionExists("worker-delete-test") {
			t.Error("Expected worker session to not exist after deletion")
		}
	})
}

func TestQueuePatternMatching(t *testing.T) {
	testCases := []struct {
		pattern  string
		queue    string
		expected bool
	}{
		// Exact matches
		{"test", "test", true},
		{"test/queue", "test/queue", true},
		{"test", "other", false},

		// Single segment wildcard (*)
		{"test/*", "test/queue", true},
		{"test/*", "test/another", true},
		{"test/*", "test", false},           // * requires at least one segment
		{"test/*", "test/queue/sub", false}, // * matches only one segment
		{"*/queue", "test/queue", true},
		{"*/queue", "prod/queue", true},
		{"*/queue", "test/other", false},

		// Multi-segment wildcard (#)
		{"test/#", "test", true},
		{"test/#", "test/queue", true},
		{"test/#", "test/queue/sub", true},
		{"test/#", "test/queue/sub/deep", true},
		{"test/#", "other/queue", false},
		{"#", "anything", true},
		{"#", "any/deep/path", true},

		// Complex patterns
		{"ml/*/image", "ml/training/image", true},
		{"ml/*/image", "ml/inference/image", true},
		{"ml/*/image", "ml/training/text", false},
		{"ml/training/*", "ml/training/image", true},
		{"ml/training/*", "ml/training/text", true},
		{"ml/training/*", "ml/inference/image", false},

		// Pattern with # in middle
		{"ml/#/final", "ml/training/final", true},
		{"ml/#/final", "ml/training/image/final", true},
		{"ml/#/final", "ml/final", true},
		{"ml/#/final", "ml/training/image", false},

		// Edge cases
		{"", "", true},
		{"test", "", false},
		{"", "test", false},
	}

	for _, tc := range testCases {
		t.Run(tc.pattern+"_vs_"+tc.queue, func(t *testing.T) {
			result := database_mock.MatchQueuePattern(tc.pattern, tc.queue)
			if result != tc.expected {
				t.Errorf("Pattern '%s' vs Queue '%s': expected %v, got %v",
					tc.pattern, tc.queue, tc.expected, result)
			}
		})
	}
}

func TestDatabaseClosedOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("OperationsAfterClose", func(t *testing.T) {
		db := database_mock.NewMockDatabase()

		// Close the database
		err := db.Close(ctx)
		if err != nil {
			t.Fatalf("Failed to close database: %v", err)
		}

		// Test task operations after close
		taskRepo := db.GetTaskRepository()
		task := &models.Task{Id: "test", Status: types.TaskStatusPending, Queue: "test"}

		err = taskRepo.Create(ctx, task)
		if err == nil {
			t.Error("Expected error for task create after close")
		}

		_, err = taskRepo.GetById(ctx, "test")
		if err == nil {
			t.Error("Expected error for task get after close")
		}

		// Test queue operations after close
		queueRepo := db.GetQueueRepository()
		queue := &models.Queue{Name: "test", Status: types.QueueStatusActive}

		err = queueRepo.Create(ctx, queue)
		if err == nil {
			t.Error("Expected error for queue create after close")
		}

		// Test worker session operations after close
		workerRepo := db.GetWorkerSessionRepository()
		session := &models.WorkerSession{WorkerId: "test", Status: types.WorkerStatusIdle}

		err = workerRepo.Create(ctx, session)
		if err == nil {
			t.Error("Expected error for worker session create after close")
		}
	})
}

// Helper type for testing
type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}
