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

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gorilla/mux"
	"github.com/jc-lab/distworker/go/internal/testing/database_mock"
	"github.com/jc-lab/distworker/go/pkg/api"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	"github.com/jc-lab/distworker/go/pkg/controller/storage"
	"github.com/jc-lab/distworker/go/pkg/controller/websocket"
	"github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// MockStorage implements the Storage interface for testing
type MockStorage struct {
	files       map[string]*storage.FileInfo
	filesData   map[string][]byte
	shouldError bool
}

func NewMockStorage() *MockStorage {
	return &MockStorage{
		files:     make(map[string]*storage.FileInfo),
		filesData: make(map[string][]byte),
	}
}

func (m *MockStorage) Upload(ctx context.Context, filename string, data io.Reader, contentType string) (*storage.FileInfo, error) {
	if m.shouldError {
		return nil, &mockError{"mock upload error"}
	}

	fileId := "mock-file-" + filename

	// Read data
	content, err := io.ReadAll(data)
	if err != nil {
		return nil, err
	}

	fileInfo := &storage.FileInfo{
		FileId:      fileId,
		Filename:    filename,
		ContentType: contentType,
		Size:        int64(len(content)),
		StorageURL:  "mock://storage/" + fileId,
		CreatedAt:   models.Now().Unix(),
	}

	m.files[fileId] = fileInfo
	m.filesData[fileId] = content

	return fileInfo, nil
}

func (m *MockStorage) Download(ctx context.Context, fileId string) (io.ReadCloser, *storage.FileInfo, error) {
	if m.shouldError {
		return nil, nil, &mockError{"mock download error"}
	}

	fileInfo, exists := m.files[fileId]
	if !exists {
		return nil, nil, &mockError{"file not found"}
	}

	data, exists := m.filesData[fileId]
	if !exists {
		return nil, nil, &mockError{"file data not found"}
	}

	reader := io.NopCloser(bytes.NewReader(data))
	return reader, fileInfo, nil
}

func (m *MockStorage) Delete(ctx context.Context, fileId string) error {
	if m.shouldError {
		return &mockError{"mock delete error"}
	}

	delete(m.files, fileId)
	delete(m.filesData, fileId)
	return nil
}

func (m *MockStorage) GetFileInfo(ctx context.Context, fileId string) (*storage.FileInfo, error) {
	if m.shouldError {
		return nil, &mockError{"mock get file info error"}
	}

	fileInfo, exists := m.files[fileId]
	if !exists {
		return nil, &mockError{"file not found"}
	}

	return fileInfo, nil
}

func (m *MockStorage) GenerateURL(ctx context.Context, fileId string, expiration int64) (string, error) {
	if m.shouldError {
		return "", &mockError{"mock generate URL error"}
	}

	fileInfo, exists := m.files[fileId]
	if !exists {
		return "", &mockError{"file not found"}
	}

	return fileInfo.StorageURL + "?expires=" + string(rune(expiration)), nil
}

func (m *MockStorage) Health(ctx context.Context) error {
	if m.shouldError {
		return &mockError{"mock health error"}
	}
	return nil
}

func (m *MockStorage) Close(ctx context.Context) error {
	return nil
}

func (m *MockStorage) SetShouldError(shouldError bool) {
	m.shouldError = shouldError
}

// Test utilities
func createTestServer() (*Server, *database_mock.MockDatabase, *MockStorage) {
	mockDB := database_mock.NewMockDatabase()
	mockStorage := NewMockStorage()

	// Create server with minimal config
	server := &Server{}
	server.SetDatabase(mockDB)
	server.SetStorage(mockStorage)
	server.workerManager = &websocket.WorkerManager{}

	return server, mockDB, mockStorage
}

func TestCreateTask(t *testing.T) {
	t.Run("CreateTaskJSON", func(t *testing.T) {
		server, _, _ := createTestServer()

		requestBody := map[string]interface{}{
			"queue":    "test-queue",
			"metadata": map[string]interface{}{"priority": "high"},
			"input":    map[string]interface{}{"data": "test"},
			"timeout":  "1h",
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		server.HandleCreateTask(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response models.Task
		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Queue != "test-queue" {
			t.Errorf("Expected queue 'test-queue', got: %s", response.Queue)
		}
		if response.Status != types.TaskStatusPending {
			t.Errorf("Expected status pending, got: %s", response.Status)
		}
		if response.TimeoutMS != 3600000 { // 1 hour in milliseconds
			t.Errorf("Expected timeout 3600000ms, got: %d", response.TimeoutMS)
		}
	})

	t.Run("CreateTaskMissingQueue", func(t *testing.T) {
		server, _, _ := createTestServer()

		requestBody := map[string]interface{}{
			"metadata": map[string]interface{}{"priority": "high"},
			"input":    map[string]interface{}{"data": "test"},
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		server.HandleCreateTask(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got: %d", rr.Code)
		}
	})

	t.Run("CreateTaskInvalidJSON", func(t *testing.T) {
		server, _, _ := createTestServer()

		req := httptest.NewRequest("POST", "/api/v1/tasks", strings.NewReader("invalid json"))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		server.HandleCreateTask(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got: %d", rr.Code)
		}
	})

	t.Run("CreateTaskInvalidTimeout", func(t *testing.T) {
		server, _, _ := createTestServer()

		requestBody := map[string]interface{}{
			"queue":   "test-queue",
			"timeout": "invalid-timeout",
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		server.HandleCreateTask(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got: %d", rr.Code)
		}
	})
}

func TestCreateTaskWithFiles(t *testing.T) {
	t.Run("CreateTaskMultipart", func(t *testing.T) {
		server, _, _ := createTestServer()

		// Create multipart form
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		// Add task data
		taskData := map[string]interface{}{
			"queue":   "test-queue",
			"timeout": "30m",
		}
		taskJSON, _ := json.Marshal(taskData)
		writer.WriteField("task", string(taskJSON))

		// Add file
		fileWriter, _ := writer.CreateFormFile("file", "test.txt")
		fileWriter.Write([]byte("test file content"))

		writer.Close()

		req := httptest.NewRequest("POST", "/api/v1/tasks", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		server.HandleCreateTask(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response models.Task
		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(response.Files) != 1 {
			t.Errorf("Expected 1 file, got: %d", len(response.Files))
		}
		if response.Files[0].Filename != "test.txt" {
			t.Errorf("Expected filename 'test.txt', got: %s", response.Files[0].Filename)
		}
	})

	t.Run("CreateTaskMultipartMissingTask", func(t *testing.T) {
		server, _, _ := createTestServer()

		// Create multipart form without task field
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)

		fileWriter, _ := writer.CreateFormFile("file", "test.txt")
		fileWriter.Write([]byte("test file content"))

		writer.Close()

		req := httptest.NewRequest("POST", "/api/v1/tasks", &buf)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		rr := httptest.NewRecorder()

		server.HandleCreateTask(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got: %d", rr.Code)
		}
	})
}

func TestGetTask(t *testing.T) {
	t.Run("GetExistingTask", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create a task in the database first
		task := &models.Task{
			Id:        "test-task-123",
			Status:    types.TaskStatusPending,
			Queue:     "test-queue",
			CreatedAt: models.Now(),
		}
		mockDB.GetTaskRepository().Create(context.Background(), task)

		req := httptest.NewRequest("GET", "/api/v1/tasks/test-task-123", nil)
		req = mux.SetURLVars(req, map[string]string{"task_id": "test-task-123"})
		rr := httptest.NewRecorder()

		server.HandleGetTask(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response models.Task
		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Id != "test-task-123" {
			t.Errorf("Expected task ID 'test-task-123', got: %s", response.Id)
		}
	})

	t.Run("GetNonexistentTask", func(t *testing.T) {
		server, _, _ := createTestServer()

		req := httptest.NewRequest("GET", "/api/v1/tasks/nonexistent", nil)
		req = mux.SetURLVars(req, map[string]string{"task_id": "nonexistent"})
		rr := httptest.NewRecorder()

		server.HandleGetTask(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got: %d", rr.Code)
		}
	})
}

func TestListTasks(t *testing.T) {
	t.Run("ListAllTasks", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create some tasks
		tasks := []*models.Task{
			{Id: "task-1", Status: types.TaskStatusPending, Queue: "queue-a"},
			{Id: "task-2", Status: types.TaskStatusProcessing, Queue: "queue-b"},
			{Id: "task-3", Status: types.TaskStatusCompleted, Queue: "queue-a"},
		}

		for _, task := range tasks {
			if err := mockDB.GetTaskRepository().Create(context.Background(), task); err != nil {
				t.Error(err)
			}
		}

		req := httptest.NewRequest("GET", "/api/v1/tasks", nil)
		rr := httptest.NewRecorder()

		server.HandleListTasks(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response struct {
			Tasks      []*models.Task `json:"tasks"`
			Pagination struct {
				Page       int   `json:"page"`
				Limit      int   `json:"limit"`
				Total      int64 `json:"total"`
				TotalPages int64 `json:"total_pages"`
			} `json:"pagination"`
		}

		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(response.Tasks) != 3 {
			t.Errorf("Expected 3 tasks, got: %d", len(response.Tasks))
		}
		if response.Pagination.Total != 3 {
			t.Errorf("Expected total 3, got: %d", response.Pagination.Total)
		}
	})

	t.Run("ListTasksWithFilter", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create tasks with different queues
		tasks := []*models.Task{
			{Id: "task-1", Status: types.TaskStatusPending, Queue: "queue-a"},
			{Id: "task-2", Status: types.TaskStatusPending, Queue: "queue-b"},
			{Id: "task-3", Status: types.TaskStatusCompleted, Queue: "queue-a"},
		}

		for _, task := range tasks {
			mockDB.GetTaskRepository().Create(context.Background(), task)
		}

		req := httptest.NewRequest("GET", "/api/v1/tasks?queue=queue-a&status=pending", nil)
		rr := httptest.NewRecorder()

		server.HandleListTasks(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response struct {
			Tasks []*models.Task `json:"tasks"`
		}

		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(response.Tasks) != 1 {
			t.Errorf("Expected 1 filtered task, got: %d", len(response.Tasks))
		}
	})

	t.Run("ListTasksWithPagination", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create multiple tasks
		for i := 0; i < 5; i++ {
			task := &models.Task{
				Id:     "task-" + string(rune(i+'1')),
				Status: types.TaskStatusPending,
				Queue:  "test-queue",
			}
			mockDB.GetTaskRepository().Create(context.Background(), task)
		}

		req := httptest.NewRequest("GET", "/api/v1/tasks?page=1&limit=2", nil)
		rr := httptest.NewRecorder()

		server.HandleListTasks(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response struct {
			Tasks      []*models.Task `json:"tasks"`
			Pagination struct {
				Page       int   `json:"page"`
				Limit      int   `json:"limit"`
				Total      int64 `json:"total"`
				TotalPages int64 `json:"total_pages"`
			} `json:"pagination"`
		}

		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(response.Tasks) != 2 {
			t.Errorf("Expected 2 tasks per page, got: %d", len(response.Tasks))
		}
		if response.Pagination.Page != 1 {
			t.Errorf("Expected page 1, got: %d", response.Pagination.Page)
		}
		if response.Pagination.Limit != 2 {
			t.Errorf("Expected limit 2, got: %d", response.Pagination.Limit)
		}
	})
}

func TestDeleteTask(t *testing.T) {
	t.Run("DeletePendingTask", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create a pending task
		task := &models.Task{
			Id:        "task-to-delete",
			Status:    types.TaskStatusPending,
			Queue:     "test-queue",
			CreatedAt: models.Now(),
		}
		mockDB.GetTaskRepository().Create(context.Background(), task)

		req := httptest.NewRequest("DELETE", "/api/v1/tasks/task-to-delete", nil)
		req = mux.SetURLVars(req, map[string]string{"task_id": "task-to-delete"})
		rr := httptest.NewRecorder()

		server.HandleDeleteTask(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response struct {
			TaskId string `json:"task_id"`
			Status string `json:"status"`
		}

		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Status != string(types.TaskStatusCancelled) {
			t.Errorf("Expected status cancelled, got: %s", response.Status)
		}

		// Verify task was updated in database
		updatedTask, _ := mockDB.GetTaskRepository().GetById(context.Background(), "task-to-delete")
		if updatedTask.Status != types.TaskStatusCancelled {
			t.Errorf("Expected task status to be cancelled in database, got: %s", updatedTask.Status)
		}
	})

	t.Run("DeleteCompletedTask", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create a completed task
		task := &models.Task{
			Id:        "completed-task",
			Status:    types.TaskStatusCompleted,
			Queue:     "test-queue",
			CreatedAt: models.Now(),
		}
		mockDB.GetTaskRepository().Create(context.Background(), task)

		req := httptest.NewRequest("DELETE", "/api/v1/tasks/completed-task", nil)
		req = mux.SetURLVars(req, map[string]string{"task_id": "completed-task"})
		rr := httptest.NewRecorder()

		server.HandleDeleteTask(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got: %d", rr.Code)
		}
	})

	t.Run("DeleteNonexistentTask", func(t *testing.T) {
		server, _, _ := createTestServer()

		req := httptest.NewRequest("DELETE", "/api/v1/tasks/nonexistent", nil)
		req = mux.SetURLVars(req, map[string]string{"task_id": "nonexistent"})
		rr := httptest.NewRecorder()

		server.HandleDeleteTask(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got: %d", rr.Code)
		}
	})
}

func TestCreateQueue(t *testing.T) {
	t.Run("CreateValidQueue", func(t *testing.T) {
		server, _, _ := createTestServer()

		requestBody := map[string]interface{}{
			"name":        "test-queue",
			"description": "Test queue description",
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/queues", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		server.HandleCreateQueue(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response models.Queue
		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Name != "test-queue" {
			t.Errorf("Expected name 'test-queue', got: %s", response.Name)
		}
		if response.Status != types.QueueStatusInactive {
			t.Errorf("Expected status inactive, got: %s", response.Status)
		}
	})

	t.Run("CreateQueueMissingName", func(t *testing.T) {
		server, _, _ := createTestServer()

		requestBody := map[string]interface{}{
			"description": "Test queue description",
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("POST", "/api/v1/queues", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		server.HandleCreateQueue(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got: %d", rr.Code)
		}
	})
}

func TestListQueues(t *testing.T) {
	t.Run("ListAllQueues", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create some queues
		queues := []*models.Queue{
			{Name: "queue-1", Status: types.QueueStatusActive, CreatedAt: models.Now()},
			{Name: "queue-2", Status: types.QueueStatusInactive, CreatedAt: models.Now()},
		}

		for _, queue := range queues {
			mockDB.GetQueueRepository().Create(context.Background(), queue)
		}

		req := httptest.NewRequest("GET", "/api/v1/queues", nil)
		rr := httptest.NewRecorder()

		server.HandleListQueues(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response struct {
			Queues []*models.Queue `json:"queues"`
		}

		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(response.Queues) != 2 {
			t.Errorf("Expected 2 queues, got: %d", len(response.Queues))
		}
	})
}

func TestGetQueue(t *testing.T) {
	t.Run("GetExistingQueue", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create a queue
		queue := &models.Queue{
			Name:        "test-queue",
			Description: "Test description",
			Status:      types.QueueStatusActive,
			CreatedAt:   models.Now(),
		}
		mockDB.GetQueueRepository().Create(context.Background(), queue)

		req := httptest.NewRequest("GET", "/api/v1/queues/test-queue", nil)
		req = mux.SetURLVars(req, map[string]string{"queue_name": "test-queue"})
		rr := httptest.NewRecorder()

		server.HandleGetQueue(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response models.Queue
		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Name != "test-queue" {
			t.Errorf("Expected name 'test-queue', got: %s", response.Name)
		}
	})

	t.Run("GetNonexistentQueue", func(t *testing.T) {
		server, _, _ := createTestServer()

		req := httptest.NewRequest("GET", "/api/v1/queues/nonexistent", nil)
		req = mux.SetURLVars(req, map[string]string{"queue_name": "nonexistent"})
		rr := httptest.NewRecorder()

		server.HandleGetQueue(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got: %d", rr.Code)
		}
	})
}

func TestUpdateQueue(t *testing.T) {
	t.Run("UpdateExistingQueue", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create a queue first
		queue := &models.Queue{
			Name:        "update-queue",
			Description: "Original description",
			Status:      types.QueueStatusActive,
			CreatedAt:   models.Now(),
		}
		mockDB.GetQueueRepository().Create(context.Background(), queue)

		requestBody := map[string]interface{}{
			"description": "Updated description",
		}

		jsonBody, _ := json.Marshal(requestBody)
		req := httptest.NewRequest("PUT", "/api/v1/queues/update-queue", bytes.NewReader(jsonBody))
		req = mux.SetURLVars(req, map[string]string{"queue_name": "update-queue"})
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		server.HandleUpdateQueue(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response models.Queue
		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Description != "Updated description" {
			t.Errorf("Expected description 'Updated description', got: %s", response.Description)
		}
	})
}

func TestDeleteQueue(t *testing.T) {
	t.Run("DeleteExistingQueue", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create a queue first
		queue := &models.Queue{
			Name:      "delete-queue",
			Status:    types.QueueStatusActive,
			CreatedAt: models.Now(),
		}
		mockDB.GetQueueRepository().Create(context.Background(), queue)

		req := httptest.NewRequest("DELETE", "/api/v1/queues/delete-queue", nil)
		req = mux.SetURLVars(req, map[string]string{"queue_name": "delete-queue"})
		rr := httptest.NewRecorder()

		server.HandleDeleteQueue(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response struct {
			Status string `json:"status"`
		}

		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Status != "deleted" {
			t.Errorf("Expected status 'deleted', got: %s", response.Status)
		}

		// Verify queue was deleted
		if mockDB.QueueExists("delete-queue") {
			t.Error("Expected queue to be deleted from database")
		}
	})
}

func TestListWorkers(t *testing.T) {
	t.Run("ListAllWorkers", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Create some worker sessions
		sessions := []*models.WorkerSession{
			{
				WorkerId:        "worker-1",
				Status:          types.WorkerStatusIdle,
				Queues:          []string{"test/*"},
				ConnectedAt:     models.Now(),
				LastHeartbeat:   models.Now(),
				ProvisionerName: "test-provisioner",
			},
			{
				WorkerId:        "worker-2",
				Status:          types.WorkerStatusProcessing,
				Queues:          []string{"prod/*"},
				ConnectedAt:     models.Now(),
				LastHeartbeat:   models.Now(),
				ProvisionerName: "prod-provisioner",
			},
		}

		for _, session := range sessions {
			mockDB.GetWorkerSessionRepository().Create(context.Background(), session)
		}

		req := httptest.NewRequest("GET", "/api/v1/workers", nil)
		rr := httptest.NewRecorder()

		server.HandleListWorkers(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response struct {
			Workers []models.Worker `json:"workers"`
		}

		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(response.Workers) != 2 {
			t.Errorf("Expected 2 workers, got: %d", len(response.Workers))
		}
	})
}

func TestHealth(t *testing.T) {
	t.Run("HealthyService", func(t *testing.T) {
		server, _, _ := createTestServer()

		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()

		server.HandleHealth(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		var response struct {
			Status    string `json:"status"`
			Timestamp int64  `json:"timestamp"`
			Version   string `json:"version"`
		}

		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Status != "up" {
			t.Errorf("Expected status 'up', got: %s", response.Status)
		}
	})

	t.Run("UnhealthyService", func(t *testing.T) {
		server, mockDB, _ := createTestServer()

		// Set database to return error
		mockDB.SetHealthError(&mockError{"database error"})

		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()

		server.HandleHealth(rr, req)

		if rr.Code != http.StatusServiceUnavailable {
			t.Errorf("Expected status 503, got: %d", rr.Code)
		}

		var response *api.HealthResponse
		err := json.NewDecoder(rr.Body).Decode(&response)
		if err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if response.Status != "down" {
			t.Errorf("Expected status 'down', got: %s", response.Status)
		}
	})
}

func TestFileDownload(t *testing.T) {
	t.Run("DownloadExistingFile", func(t *testing.T) {
		server, _, mockStorage := createTestServer()

		// Upload a file first
		fileContent := []byte("test file content")
		fileInfo, _ := mockStorage.Upload(context.Background(), "test.txt", bytes.NewReader(fileContent), "text/plain")

		req := httptest.NewRequest("GET", "/worker/v1/file/"+fileInfo.FileId, nil)
		req = mux.SetURLVars(req, map[string]string{"file_id": fileInfo.FileId})
		rr := httptest.NewRecorder()

		server.HandleFileDownload(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected status 200, got: %d", rr.Code)
		}

		// Check content type
		expectedContentType := "text/plain"
		if rr.Header().Get("Content-Type") != expectedContentType {
			t.Errorf("Expected Content-Type %s, got: %s", expectedContentType, rr.Header().Get("Content-Type"))
		}

		// Check content
		if rr.Body.String() != string(fileContent) {
			t.Errorf("Expected file content %s, got: %s", string(fileContent), rr.Body.String())
		}
	})

	t.Run("DownloadNonexistentFile", func(t *testing.T) {
		server, _, _ := createTestServer()

		req := httptest.NewRequest("GET", "/worker/v1/file/nonexistent", nil)
		req = mux.SetURLVars(req, map[string]string{"file_id": "nonexistent"})
		rr := httptest.NewRecorder()

		server.HandleFileDownload(rr, req)

		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got: %d", rr.Code)
		}
	})

	t.Run("DownloadMissingFileId", func(t *testing.T) {
		server, _, _ := createTestServer()

		req := httptest.NewRequest("GET", "/worker/v1/file/", nil)
		req = mux.SetURLVars(req, map[string]string{"file_id": ""})
		rr := httptest.NewRecorder()

		server.HandleFileDownload(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400, got: %d", rr.Code)
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

// Add these methods to Server for testing access
func (s *Server) SetDatabase(db database.Database) {
	s.db = db
}

func (s *Server) SetStorage(stor storage.Storage) {
	s.storage = stor
}

// Wrapper methods to access private handler methods for testing
func (s *Server) HandleCreateTask(w http.ResponseWriter, r *http.Request) {
	s.handleCreateTask(w, r)
}

func (s *Server) HandleGetTask(w http.ResponseWriter, r *http.Request) {
	s.handleGetTask(w, r)
}

func (s *Server) HandleListTasks(w http.ResponseWriter, r *http.Request) {
	s.handleListTasks(w, r)
}

func (s *Server) HandleDeleteTask(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteTask(w, r)
}

func (s *Server) HandleCreateQueue(w http.ResponseWriter, r *http.Request) {
	s.handleCreateQueue(w, r)
}

func (s *Server) HandleListQueues(w http.ResponseWriter, r *http.Request) {
	s.handleListQueues(w, r)
}

func (s *Server) HandleGetQueue(w http.ResponseWriter, r *http.Request) {
	s.handleGetQueue(w, r)
}

func (s *Server) HandleUpdateQueue(w http.ResponseWriter, r *http.Request) {
	s.handleUpdateQueue(w, r)
}

func (s *Server) HandleDeleteQueue(w http.ResponseWriter, r *http.Request) {
	s.handleDeleteQueue(w, r)
}

func (s *Server) HandleListWorkers(w http.ResponseWriter, r *http.Request) {
	s.handleListWorkers(w, r)
}

func (s *Server) HandleHealth(w http.ResponseWriter, r *http.Request) {
	s.handleHealth(w, r)
}

func (s *Server) HandleFileDownload(w http.ResponseWriter, r *http.Request) {
	s.handleFileDownload(w, r)
}
