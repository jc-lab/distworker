package integration_test

import (
	"bytes"
	"context"
	"github.com/jc-lab/distworker/go/internal/protocol"
	storage_impl "github.com/jc-lab/distworker/go/internal/storage"
	"github.com/jc-lab/distworker/go/internal/testing/database_mock"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	"github.com/jc-lab/distworker/go/pkg/controller/storage"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"io"
	"net/http/httptest"
	"testing"
)

// TestFullWorkflow tests a complete end-to-end workflow
func TestFullWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup test environment
	mockDB := database_mock.NewMockDatabase()
	defer mockDB.Close(context.Background())

	// Use local storage for integration test
	tempDir := t.TempDir()
	localStorage, err := storage_impl.NewLocalStorage(&storage_impl.LocalConfig{
		Path:    tempDir,
		MaxSize: 1024 * 1024, // 1MB
	})
	if err != nil {
		t.Fatalf("Failed to create local storage: %v", err)
	}
	defer localStorage.Close(context.Background())

	// Create server (this would need to be properly implemented)
	server := &MockServer{
		db:      mockDB,
		storage: localStorage,
	}
	_ = server

	t.Run("CreateQueueAndTask", func(t *testing.T) {
		ctx := context.Background()

		// 1. Create a queue
		queue := &models2.Queue{
			Name:        "integration-test-queue",
			Description: "Integration test queue",
			Status:      types.QueueStatusActive,
			CreatedAt:   models2.Now(),
			UpdatedAt:   models2.Now(),
		}

		err := mockDB.GetQueueRepository().Create(ctx, queue)
		if err != nil {
			t.Fatalf("Failed to create queue: %v", err)
		}

		// 2. Create a task
		task := &models2.Task{
			Id:        models2.NewTaskId(),
			Status:    types.TaskStatusPending,
			Queue:     "integration-test-queue",
			Metadata:  map[string]interface{}{"priority": "high", "type": "integration-test"},
			Input:     map[string]interface{}{"operation": "test", "param1": "value1"},
			TimeoutMS: 60000, // 1 minute
			CreatedAt: models2.Now(),
		}

		err = mockDB.GetTaskRepository().Create(ctx, task)
		if err != nil {
			t.Fatalf("Failed to create task: %v", err)
		}

		// 3. Verify task was created
		retrievedTask, err := mockDB.GetTaskRepository().GetById(ctx, task.Id)
		if err != nil {
			t.Fatalf("Failed to retrieve created task: %v", err)
		}

		if retrievedTask.Status != types.TaskStatusPending {
			t.Errorf("Expected task status %s, got: %s", types.TaskStatusPending, retrievedTask.Status)
		}

		// 4. Simulate worker registration
		workerSession := &models2.WorkerSession{
			WorkerId:        models2.NewWorkerId(),
			Status:          types.WorkerStatusIdle,
			Queues:          []string{"integration-test-*"},
			ConnectedAt:     models2.Now(),
			LastHeartbeat:   models2.Now(),
			ProvisionerName: "test-provisioner",
			ResourceInfo:    map[string]interface{}{"cpu": 4, "memory": "8GB"},
			WorkerToken:     "test-token-123",
		}

		err = mockDB.GetWorkerSessionRepository().Create(ctx, workerSession)
		if err != nil {
			t.Fatalf("Failed to create worker session: %v", err)
		}

		// 5. Simulate task assignment (worker picks up task)
		task.Status = types.TaskStatusProcessing
		task.WorkerId = workerSession.WorkerId
		task.StartedAt = models2.NowPtr()

		err = mockDB.GetTaskRepository().Update(ctx, task)
		if err != nil {
			t.Fatalf("Failed to update task to processing: %v", err)
		}

		// 6. Update worker session
		workerSession.Status = types.WorkerStatusProcessing
		workerSession.CurrentTask = task.Id

		err = mockDB.GetWorkerSessionRepository().Update(ctx, workerSession)
		if err != nil {
			t.Fatalf("Failed to update worker session: %v", err)
		}

		// 7. Simulate task completion
		task.Status = types.TaskStatusCompleted
		task.Result = map[string]interface{}{
			"status":    "success",
			"output":    "test result",
			"duration":  "30s",
			"processed": true,
		}
		task.CompletedAt = models2.NowPtr()

		err = mockDB.GetTaskRepository().Update(ctx, task)
		if err != nil {
			t.Fatalf("Failed to update task to completed: %v", err)
		}

		// 8. Update worker back to idle
		workerSession.Status = types.WorkerStatusIdle
		workerSession.CurrentTask = ""

		err = mockDB.GetWorkerSessionRepository().Update(ctx, workerSession)
		if err != nil {
			t.Fatalf("Failed to update worker session to idle: %v", err)
		}

		// 9. Verify final state
		finalTask, err := mockDB.GetTaskRepository().GetById(ctx, task.Id)
		if err != nil {
			t.Fatalf("Failed to retrieve final task: %v", err)
		}

		if finalTask.Status != types.TaskStatusCompleted {
			t.Errorf("Expected final task status %s, got: %s", types.TaskStatusCompleted, finalTask.Status)
		}

		if finalTask.Result == nil {
			t.Error("Expected task result to be set")
		}

		if finalTask.CompletedAt == nil {
			t.Error("Expected task completion time to be set")
		}

		finalWorker, err := mockDB.GetWorkerSessionRepository().GetById(ctx, workerSession.WorkerId)
		if err != nil {
			t.Fatalf("Failed to retrieve final worker session: %v", err)
		}

		if finalWorker.Status != types.WorkerStatusIdle {
			t.Errorf("Expected final worker status %s, got: %s", types.WorkerStatusIdle, finalWorker.Status)
		}
	})

	t.Run("FileUploadDownloadWorkflow", func(t *testing.T) {
		ctx := context.Background()

		// 1. Upload a file
		testContent := "This is test file content for integration testing"
		fileInfo, err := localStorage.Upload(ctx, "integration-test.txt", bytes.NewReader([]byte(testContent)), "text/plain")
		if err != nil {
			t.Fatalf("Failed to upload file: %v", err)
		}

		// 2. Create task with file
		task := &models2.Task{
			Id:       models2.NewTaskId(),
			Status:   types.TaskStatusPending,
			Queue:    "file-processing-queue",
			Metadata: map[string]interface{}{"has_files": true},
			Input:    map[string]interface{}{"operation": "process-file"},
			Files: []models2.FileInfo{
				{
					FileId:      fileInfo.FileId,
					Filename:    fileInfo.Filename,
					ContentType: fileInfo.ContentType,
					Size:        fileInfo.Size,
					StorageURL:  fileInfo.StorageURL,
				},
			},
			TimeoutMS: 120000, // 2 minutes
			CreatedAt: models2.Now(),
		}

		err = mockDB.GetTaskRepository().Create(ctx, task)
		if err != nil {
			t.Fatalf("Failed to create task with file: %v", err)
		}

		// 3. Verify file can be downloaded
		reader, downloadedFileInfo, err := localStorage.Download(ctx, fileInfo.FileId)
		if err != nil {
			t.Fatalf("Failed to download file: %v", err)
		}
		defer reader.Close()

		downloadedContent, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read downloaded file: %v", err)
		}

		if string(downloadedContent) != testContent {
			t.Errorf("Downloaded content doesn't match uploaded content")
		}

		if downloadedFileInfo.Filename != "integration-test.txt" {
			t.Errorf("Expected filename 'integration-test.txt', got: %s", downloadedFileInfo.Filename)
		}

		// 4. Simulate task processing and result file creation
		resultContent := "This is the processed result file"
		resultFileInfo, err := localStorage.Upload(ctx, "result.txt", bytes.NewReader([]byte(resultContent)), "text/plain")
		if err != nil {
			t.Fatalf("Failed to upload result file: %v", err)
		}

		// 5. Complete task with result files
		task.Status = types.TaskStatusCompleted
		task.Result = map[string]interface{}{
			"status":          "success",
			"files_processed": 1,
		}
		task.ResultFiles = []models2.FileInfo{
			{
				FileId:      resultFileInfo.FileId,
				Filename:    resultFileInfo.Filename,
				ContentType: resultFileInfo.ContentType,
				Size:        resultFileInfo.Size,
				StorageURL:  resultFileInfo.StorageURL,
			},
		}
		task.CompletedAt = models2.NowPtr()

		err = mockDB.GetTaskRepository().Update(ctx, task)
		if err != nil {
			t.Fatalf("Failed to update task with result files: %v", err)
		}

		// 6. Verify task has result files
		finalTask, err := mockDB.GetTaskRepository().GetById(ctx, task.Id)
		if err != nil {
			t.Fatalf("Failed to retrieve final task: %v", err)
		}

		if len(finalTask.ResultFiles) != 1 {
			t.Errorf("Expected 1 result file, got: %d", len(finalTask.ResultFiles))
		}

		// 7. Cleanup files
		err = localStorage.Delete(ctx, fileInfo.FileId)
		if err != nil {
			t.Errorf("Failed to delete original file: %v", err)
		}

		err = localStorage.Delete(ctx, resultFileInfo.FileId)
		if err != nil {
			t.Errorf("Failed to delete result file: %v", err)
		}
	})

	t.Run("AuthenticationWorkflow", func(t *testing.T) {
		// Test HMAC signature generation and validation
		workerToken := "integration-test-token"
		workerId := "integration-test-worker"

		// Create a mock HTTP request
		req := httptest.NewRequest("GET", "http://localhost:8080/worker/v1/file/test-file-123?param=value", nil)

		// Build authorization header
		authHeader := protocol.BuildAuthorizationHeader(workerId, workerToken, req)
		req.Header.Set("Authorization", authHeader)

		// Validate the signature
		vctx, err := protocol.NewValidateContext(req)
		if err != nil {
			t.Errorf("Failed to validate generated signature: %v", err)
		}
		if err := vctx.ValidateSignature(workerToken); err != nil {
			t.Errorf("Failed to validate generated signature: %v", err)
		}

		// Verify worker ID was extracted
		if vctx.WorkerId != workerId {
			t.Errorf("Expected extracted worker ID %s, got: %s", workerId, vctx.WorkerId)
		}

		// Test with wrong token
		req2 := httptest.NewRequest("GET", "http://localhost:8080/worker/v1/file/test-file-123", nil)
		protocol.BuildAuthorizationHeader(workerId, workerToken, req2)
		req2.Header.Set("Authorization", authHeader)

		// Validate the signature
		vctx, err = protocol.NewValidateContext(req2)
		if err != nil {
			t.Errorf("Failed to validate generated signature: %v", err)
		}
		if err := vctx.ValidateSignature("wrong-token"); err == nil {
			t.Error("Expected validation to fail with wrong token")
		}
	})

	t.Run("QueuePatternMatching", func(t *testing.T) {
		// Test queue pattern matching functionality used in worker assignment
		testCases := []struct {
			pattern  string
			queue    string
			expected bool
		}{
			{"integration/*", "integration/test", true},
			{"integration/*", "integration/test/sub", false},
			{"integration/#", "integration/test/sub/deep", true},
			{"ml/training/*", "ml/training/image", true},
			{"ml/training/*", "ml/inference/image", false},
			{"data/#", "data/processing/batch", true},
			{"#", "any/queue/name", true},
		}

		for _, tc := range testCases {
			result := database_mock.MatchQueuePattern(tc.pattern, tc.queue)
			if result != tc.expected {
				t.Errorf("Pattern '%s' vs Queue '%s': expected %v, got %v",
					tc.pattern, tc.queue, tc.expected, result)
			}
		}
	})

	t.Run("ConcurrentOperations", func(t *testing.T) {
		ctx := context.Background()

		// Test concurrent task creation and processing
		numTasks := 5
		done := make(chan error, numTasks)

		for i := 0; i < numTasks; i++ {
			go func(taskNum int) {
				task := &models2.Task{
					Id:        models2.NewTaskId(),
					Status:    types.TaskStatusPending,
					Queue:     "concurrent-test-queue",
					Metadata:  map[string]interface{}{"task_num": taskNum},
					Input:     map[string]interface{}{"operation": "concurrent-test"},
					TimeoutMS: 30000,
					CreatedAt: models2.Now(),
				}

				err := mockDB.GetTaskRepository().Create(ctx, task)
				if err != nil {
					done <- err
					return
				}

				// Simulate processing
				task.Status = types.TaskStatusCompleted
				task.Result = map[string]interface{}{"task_num": taskNum, "status": "success"}
				task.CompletedAt = models2.NowPtr()

				err = mockDB.GetTaskRepository().Update(ctx, task)
				done <- err
			}(i)
		}

		// Wait for all tasks to complete
		for i := 0; i < numTasks; i++ {
			if err := <-done; err != nil {
				t.Errorf("Concurrent task %d failed: %v", i, err)
			}
		}

		// Verify all tasks were created and completed
		tasks, total, err := mockDB.GetTaskRepository().List(ctx, database.TaskFilter{
			Queue: "concurrent-test-queue",
		}, 1, 10)
		if err != nil {
			t.Fatalf("Failed to list concurrent test tasks: %v", err)
		}

		if total != int64(numTasks) {
			t.Errorf("Expected %d concurrent tasks, got: %d", numTasks, total)
		}

		completedCount := 0
		for _, task := range tasks {
			if task.Status == types.TaskStatusCompleted {
				completedCount++
			}
		}

		if completedCount != numTasks {
			t.Errorf("Expected %d completed tasks, got: %d", numTasks, completedCount)
		}
	})
}

// TestErrorScenarios tests various error conditions
func TestErrorScenarios(t *testing.T) {
	mockDB := database_mock.NewMockDatabase()
	defer mockDB.Close(context.Background())

	ctx := context.Background()

	t.Run("DatabaseErrors", func(t *testing.T) {
		// Test database connection failure
		mockDB.Close(ctx)

		task := &models2.Task{
			Id:     "error-test-task",
			Status: types.TaskStatusPending,
			Queue:  "error-test-queue",
		}

		err := mockDB.GetTaskRepository().Create(ctx, task)
		if err == nil {
			t.Error("Expected error when database is closed")
		}
	})

	t.Run("StorageErrors", func(t *testing.T) {
		// Test storage failure scenarios
		tempDir := t.TempDir()
		localStorage, err := storage_impl.NewLocalStorage(&storage_impl.LocalConfig{
			Path:    tempDir,
			MaxSize: 100, // Very small limit to trigger errors
		})
		if err != nil {
			t.Fatalf("Failed to create local storage: %v", err)
		}
		defer localStorage.Close(context.Background())

		// Try to upload a file that's too large
		largeContent := make([]byte, 200) // Larger than the 100-byte limit
		_, err = localStorage.Upload(ctx, "large-file.txt", bytes.NewReader(largeContent), "text/plain")
		if err == nil {
			t.Error("Expected error for oversized file upload")
		}

		// Try to download non-existent file
		_, _, err = localStorage.Download(ctx, "non-existent-file-id")
		if err == nil {
			t.Error("Expected error for non-existent file download")
		}
	})

	t.Run("InvalidData", func(t *testing.T) {
		mockDB2 := database_mock.NewMockDatabase()
		defer mockDB2.Close(context.Background())

		// Test with invalid task data
		task := &models2.Task{
			// Missing required ID
			Status: types.TaskStatusPending,
			Queue:  "test-queue",
		}

		err := mockDB2.GetTaskRepository().Create(ctx, task)
		if err == nil {
			t.Error("Expected error for task without ID")
		}

		// Test with invalid queue data
		queue := &models2.Queue{
			// Missing required name
			Description: "Test queue",
			Status:      types.QueueStatusActive,
		}

		err = mockDB2.GetQueueRepository().Create(ctx, queue)
		if err == nil {
			t.Error("Expected error for queue without name")
		}
	})
}

// MockServer is a minimal server implementation for testing
type MockServer struct {
	db      database.Database
	storage storage.Storage
}

func (s *MockServer) GetDatabase() database.Database {
	return s.db
}

func (s *MockServer) GetStorage() storage.Storage {
	return s.storage
}
