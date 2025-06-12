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
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/jc-lab/distworker/go/internal/protocol"
	"github.com/jc-lab/distworker/go/internal/version"
	"github.com/jc-lab/distworker/go/pkg/api"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	"github.com/jc-lab/distworker/go/pkg/controller/subscriber"
	"github.com/jc-lab/distworker/go/pkg/healthchecker"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	errors2 "github.com/pkg/errors"
	"io"
	"log"
	"mime/multipart"
	"path/filepath"
	"strings"

	"net/http"
	"strconv"
	"time"
)

// handleCreateTask handles POST /api/v1/tasks
//
// @Summary Create a new task
// @Description Create a new task in the specified queue
// @Tags tasks
// @Accept json,multipart/form-data
// @Produce json
// @Param wait query int false "wait timeout (milliseconds). -1 is infinite, 0 is disabled, >=0 is wait"
// @Param task body api.CreateTaskRequest true "Task data"
// @Success 200 {object} models.Task
// @Failure 400 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /tasks [post]
func (s *Server) handleCreateTask(c *gin.Context) {
	contentType := c.GetHeader("Content-Type")

	// Check if it's multipart form data (file upload)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		s.handleCreateTaskWithFiles(c)
		return
	}

	var wait int = 0
	waitParam := c.Request.URL.Query().Get("wait")
	if waitParam != "" {
		var err error
		wait, err = strconv.Atoi(waitParam)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Handle JSON request
	var request api.CreateTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, listener, err := s.CreateTask(c.Request.Context(), &request, wait != 0)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if listener != nil {
		var err error
		var waitCtx context.Context
		var waitCancel context.CancelFunc
		if wait > 0 {
			waitCtx, waitCancel = context.WithTimeout(c.Request.Context(), time.Duration(wait)*time.Millisecond)
		} else {
			waitCtx, waitCancel = context.WithCancel(c.Request.Context())
		}
		defer waitCancel()

		newTask, err := listener.Wait(waitCtx)
		if err != nil {
			log.Printf("Task[%s] wait failed: %+v", task.Id, err)

			newTask, err = s.db.GetTaskRepository().GetById(c.Request.Context(), task.Id)
			if err != nil {
				log.Printf("Task[%s] get failed: %+v", task.Id, err)
			} else {
				task = newTask
			}
		} else {
			task = newTask
		}
	}

	c.JSON(http.StatusOK, task)
}
func (s *Server) CreateTask(ctx context.Context, request *api.CreateTaskRequest, wait bool) (*models2.Task, subscriber.Listener[*models2.Task, *models2.TaskProgress], error) {
	// Validate required fields
	if request.Queue == "" {
		return nil, nil, errors.New("queue is required")
	}

	// Parse timeout
	timeoutMS := int64(0)
	if request.Timeout != "" {
		duration, err := time.ParseDuration(request.Timeout)
		if err != nil {
			return nil, nil, errors2.Wrap(err, "invalid timeout format")
		}
		timeoutMS = duration.Milliseconds()
	}

	// Create task
	task := &models2.Task{
		Id:        models2.NewTaskId(),
		Status:    types.TaskStatusPending,
		Queue:     request.Queue,
		Metadata:  request.Metadata,
		Input:     request.Input,
		TimeoutMS: timeoutMS,
		CreatedAt: models2.Now(),
		MaxRetry:  request.Retry,
	}

	var taskListener subscriber.Listener[*models2.Task, *models2.TaskProgress]

	if wait {
		var err error
		taskListener, err = s.workerManager.GetTaskListener(task.Id)
		if err != nil {
			return nil, nil, err
		}
	}

	if err := s.db.GetTaskRepository().Create(ctx, task); err != nil {
		return nil, nil, errors2.Wrap(err, "failed to create task")
	}
	s.workerManager.EnqueueTask(task)

	return task, taskListener, nil
}

// handleCreateTaskWithFiles handles POST /api/v1/tasks with file uploads
func (s *Server) handleCreateTaskWithFiles(c *gin.Context) {
	// Parse multipart form
	err := c.Request.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Failed to parse multipart form"})
		return
	}

	defer func() {
		_ = c.Request.MultipartForm.RemoveAll()
	}()

	// Get task data from form
	taskDataStr := c.Request.FormValue("task")
	if taskDataStr == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "task field is required"})
		return
	}

	// Parse task JSON
	var request api.CreateTaskRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate required fields
	if request.Queue == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "queue is required"})
		return
	}

	// Parse timeout
	timeoutMS := int64(0)
	if request.Timeout != "" {
		duration, err := time.ParseDuration(request.Timeout)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Invalid timeout format"})
			return
		}
		timeoutMS = duration.Milliseconds()
	}

	// Handle file uploads
	var fileInfos []models2.FileInfo
	if c.Request.MultipartForm != nil && c.Request.MultipartForm.File != nil {
		for fieldName, fileHeaders := range c.Request.MultipartForm.File {
			for _, fileHeader := range fileHeaders {
				fileInfo, err := s.uploadFile(c.Request, fieldName, fileHeader)
				if err != nil {
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file: " + err.Error()})
					return
				}
				fileInfos = append(fileInfos, *fileInfo)
			}
		}
	}

	// Create task
	task := &models2.Task{
		Id:        models2.NewTaskId(),
		Status:    types.TaskStatusPending,
		Queue:     request.Queue,
		Metadata:  request.Metadata,
		Input:     request.Input,
		Files:     fileInfos,
		TimeoutMS: timeoutMS,
		CreatedAt: models2.Now(),
	}

	if err := s.db.GetTaskRepository().Create(c.Request.Context(), task); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to create task"})
		return
	}

	// Try to assign the task to an available worker
	s.workerManager.EnqueueTask(task)

	c.JSON(http.StatusOK, task)
}

// uploadFile uploads a single file to storage
func (s *Server) uploadFile(r *http.Request, fieldName string, fileHeader *multipart.FileHeader) (*models2.FileInfo, error) {
	// Open uploaded file
	file, err := fileHeader.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Detect content type
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		// Try to detect from file extension
		ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
		switch ext {
		case ".jpg", ".jpeg":
			contentType = "image/jpeg"
		case ".png":
			contentType = "image/png"
		case ".pdf":
			contentType = "application/pdf"
		case ".txt":
			contentType = "text/plain"
		case ".json":
			contentType = "application/json"
		default:
			contentType = "application/octet-stream"
		}
	}

	// Upload to storage
	storageFileInfo, err := s.storage.Upload(r.Context(), fileHeader.Filename, file, contentType)
	if err != nil {
		return nil, err
	}

	// Convert to models.FileInfo
	fileInfo := &models2.FileInfo{
		FileId:      storageFileInfo.FileId,
		Filename:    storageFileInfo.Filename,
		ContentType: storageFileInfo.ContentType,
		Size:        storageFileInfo.Size,
		StorageURL:  storageFileInfo.StorageURL,
	}

	return fileInfo, nil
}

// handleGetTask handles GET /api/v1/tasks/{task_id}
//
// @Summary Get task by ID
// @Description Get detailed information about a specific task
// @Tags tasks
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Success 200 {object} models.Task
// @Failure 404 {object} api.ErrorResponse
// @Router /tasks/{task_id} [get]
func (s *Server) handleGetTask(c *gin.Context) {
	taskId := c.Param("task_id")

	task, err := s.db.GetTaskRepository().GetById(c.Request.Context(), taskId)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// handleListTasks handles GET /api/v1/tasks
//
// @Summary List tasks
// @Description Get a paginated list of tasks with optional filtering
// @Tags tasks
// @Accept json
// @Produce json
// @Param queue query string false "Filter by queue name"
// @Param status query string false "Filter by task status"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} api.ListTasksResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /tasks [get]
func (s *Server) handleListTasks(c *gin.Context) {
	// Parse query parameters
	queue := c.Request.URL.Query().Get("queue")
	status := c.Request.URL.Query().Get("status")
	pageStr := c.Request.URL.Query().Get("page")
	limitStr := c.Request.URL.Query().Get("limit")

	page := 1
	if pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	filter := database.TaskFilter{
		Queue: queue,
		//FIXME: STATUS FILTERING
	}
	_ = status

	tasks, total, err := s.db.GetTaskRepository().List(c.Request.Context(), filter, page, limit)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tasks"})
		return
	}

	totalPages := (total + int64(limit) - 1) / int64(limit)

	response := &api.ListTasksResponse{
		Tasks: tasks,
		Pagination: api.PaginationInfo{
			Page:       page,
			Limit:      limit,
			Total:      total,
			TotalPages: totalPages,
		},
	}

	c.JSON(http.StatusOK, response)
}

// handleDeleteTask handles DELETE /api/v1/tasks/{task_id}
//
// @Summary Cancel/Delete a task
// @Description Cancel a pending or processing task
// @Tags tasks
// @Accept json
// @Produce json
// @Param task_id path string true "Task ID"
// @Success 200 {object} api.DeleteTaskResponse
// @Failure 400 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /tasks/{task_id} [delete]
func (s *Server) handleDeleteTask(c *gin.Context) {
	taskId := c.Param("task_id")

	// Get task first to check if it exists and can be cancelled
	task, err := s.db.GetTaskRepository().GetById(c.Request.Context(), taskId)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Only allow cancellation of pending or processing tasks
	if task.Status != types.TaskStatusPending && task.Status != types.TaskStatusProcessing {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Task cannot be cancelled"})
		return
	}

	// Update task status to cancelled
	task.Status = types.TaskStatusFinished
	task.Error = &models2.TaskError{
		Code:    types.TaskErrorCodeCancelled,
		Message: "cancelled by user",
	}
	task.CompletedAt = models2.NowPtr()

	if err := s.db.GetTaskRepository().Update(c.Request.Context(), task); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to cancel task"})
		return
	}

	response := &api.DeleteTaskResponse{
		TaskId: taskId,
		Status: task.Status,
	}

	c.JSON(http.StatusOK, response)
}

// handleCreateQueue handles POST /api/v1/queues
//
// Queue routes
// @Summary Create a new queue
// @Description Create a new task queue
// @Tags queues
// @Accept json
// @Produce json
// @Param queue body api.CreateQueueRequest true "Queue data"
// @Success 200 {object} models.Queue
// @Failure 400 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /queues [post]
func (s *Server) handleCreateQueue(c *gin.Context) {
	var request api.CreateQueueRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.Name == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	queue := &models2.Queue{
		Name:        request.Name,
		Description: request.Description,
		Status:      types.QueueStatusInactive,
		CreatedAt:   models2.Now(),
		UpdatedAt:   models2.Now(),
	}

	if err := s.db.GetQueueRepository().Create(c.Request.Context(), queue); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to create queue"})
		return
	}

	c.JSON(http.StatusOK, queue)
}

// handleListQueues handles GET /api/v1/queues
//
// @Summary List queues
// @Description Get a list of all task queues
// @Tags queues
// @Accept json
// @Produce json
// @Success 200 {object} api.ListQueuesResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /queues [get]
func (s *Server) handleListQueues(c *gin.Context) {
	queues, err := s.db.GetQueueRepository().List(c.Request.Context())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to list queues"})
		return
	}

	response := &api.ListQueuesResponse{
		Queues: queues,
	}

	c.JSON(http.StatusOK, response)
}

// handleGetQueue handles GET /api/v1/queues/{queue_name}
//
// @Summary Get queue by name
// @Description Get detailed information about a specific queue
// @Tags queues
// @Accept json
// @Produce json
// @Param queue_name path string true "Queue name"
// @Success 200 {object} models.Queue
// @Failure 404 {object} api.ErrorResponse
// @Router /queues/{queue_name} [get]
func (s *Server) handleGetQueue(c *gin.Context) {
	queueName := c.Param("queue_name")

	queue, err := s.db.GetQueueRepository().GetByName(c.Request.Context(), queueName)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}

	c.JSON(http.StatusOK, queue)
}

// handleUpdateQueue handles PUT /api/v1/queues/{queue_name}
//
// @Summary Update queue
// @Description Update queue description
// @Tags queues
// @Accept json
// @Produce json
// @Param queue_name path string true "Queue name"
// @Param queue body api.UpdateQueueRequest true "Queue update data"
// @Success 200 {object} models.Queue
// @Failure 400 {object} api.ErrorResponse
// @Failure 404 {object} api.ErrorResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /queues/{queue_name} [put]
func (s *Server) handleUpdateQueue(c *gin.Context) {
	queueName := c.Param("queue_name")

	queue, err := s.db.GetQueueRepository().GetByName(c.Request.Context(), queueName)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "Queue not found"})
		return
	}

	var request api.UpdateQueueRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	queue.Description = request.Description
	queue.UpdatedAt = models2.Now()

	if err := s.db.GetQueueRepository().Update(c.Request.Context(), queue); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to update queue"})
		return
	}

	c.JSON(http.StatusOK, queue)
}

// handleDeleteQueue handles DELETE /api/v1/queues/{queue_name}
//
// @Summary Delete queue
// @Description Delete a task queue
// @Tags queues
// @Accept json
// @Produce json
// @Param queue_name path string true "Queue name"
// @Success 200 {object} api.DeleteQueueResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /queues/{queue_name} [delete]
func (s *Server) handleDeleteQueue(c *gin.Context) {
	queueName := c.Param("queue_name")

	if err := s.db.GetQueueRepository().Delete(c.Request.Context(), queueName); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete queue"})
		return
	}

	response := &api.DeleteQueueResponse{
		Status: "deleted",
	}

	c.JSON(http.StatusOK, response)
}

// handleGetQueueStats handles GET /api/v1/queues/{queue_name}/stats
//
// @Summary Get queue statistics
// @Description Get statistics for a specific queue
// @Tags queues
// @Accept json
// @Produce json
// @Param queue_name path string true "Queue name"
// @Success 200 {object} models.QueueStats
// @Failure 501 {object} api.ErrorResponse
// @Router /queues/{queue_name}/stats [get]
func (s *Server) handleGetQueueStats(c *gin.Context) {
	// TODO: Implement queue statistics
	c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// handleListWorkers handles GET /api/v1/workers
//
// @Summary List workers
// @Description Get a list of all connected workers
// @Tags workers
// @Accept json
// @Produce json
// @Success 200 {object} api.ListWorkersResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /workers [get]
func (s *Server) handleListWorkers(c *gin.Context) {
	sessions, err := s.db.GetWorkerSessionRepository().List(c.Request.Context())
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to list workers"})
		return
	}

	workers := make([]*models2.Worker, len(sessions))
	for i, session := range sessions {
		workers[i] = session.ToModel()
	}

	response := &api.ListWorkersResponse{
		Workers: workers,
	}

	c.JSON(http.StatusOK, response)
}

// handleDeleteWorker handles DELETE /api/v1/workers/{worker_id}
//
// @Summary Disconnect worker
// @Description Disconnect a specific worker
// @Tags workers
// @Accept json
// @Produce json
// @Param worker_id path string true "Worker ID"
// @Success 200 {object} api.DeleteWorkerResponse
// @Failure 501 {object} api.ErrorResponse
// @Router /workers/{worker_id} [delete]
func (s *Server) handleDeleteWorker(c *gin.Context) {
	// TODO: Implement worker disconnection
	c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// handleListProvisioners handles GET /api/v1/provisioners
//
// @Summary List provisioners
// @Description Get a list of all worker provisioners
// @Tags provisioners
// @Accept json
// @Produce json
// @Success 200 {array} models.Provisioner
// @Failure 501 {object} api.ErrorResponse
// @Router /provisioners [get]
func (s *Server) handleListProvisioners(c *gin.Context) {
	// TODO: Implement provisioner listing
	c.AbortWithStatusJSON(http.StatusNotImplemented, gin.H{"error": "Not implemented"})
}

// handleHealth handles GET /health
//
// @Summary Health check
// @Description Get system health status
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} api.HealthResponse
// @Failure 503 {object} api.HealthResponse
// @Router /health [get]
func (s *Server) handleHealth(c *gin.Context) {
	response := &api.HealthResponse{
		Timestamp: time.Now().UnixMilli(),
		Version:   version.Version,
		Details:   make(map[string]*api.HealthDetail),
	}

	response.Details, response.Status = healthchecker.Check(c.Request.Context(), []healthchecker.Checkable{
		&healthchecker.Feature{
			Name:        "mongodb",
			HealthFunc:  s.db.Health,
			Requirement: true,
		},
		&healthchecker.Feature{
			Name:        "storage",
			HealthFunc:  s.storage.Health,
			Requirement: false,
		},
	}, time.Second)
	if response.Status == types.HealthStatusDown {
		c.AbortWithStatusJSON(http.StatusServiceUnavailable, response)
	} else {
		c.JSON(http.StatusOK, response)
	}
}

// handleMetrics handles GET /metrics
//
// @Summary Prometheus metrics
// @Description Get Prometheus metrics
// @Tags system
// @Accept text/plain
// @Produce text/plain
// @Success 200 {string} string "Prometheus metrics"
// @Router /metrics [get]
func (s *Server) handleMetrics(c *gin.Context) {
	// Collect latest metrics from database before serving
	if s.metrics != nil {
		s.metrics.CollectDatabaseStats(c.Request.Context(), s.db)
		s.metrics.CollectWorkerStats(c.Request.Context(), s.workerManager)
	}

	// Serve Prometheus metrics
	s.metricsHandler.ServeHTTP(c.Writer, c.Request)
}

// handleWorkerWebSocket handles WebSocket connections from workers
func (s *Server) handleWorkerWebSocket(c *gin.Context) {
	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "Failed to upgrade to WebSocket"})
		return
	}

	// Hand off connection to worker manager
	s.websocketListener.HandleConnection(conn)
}

// handleFileDownload handles GET /worker/v1/file/{file_id}
func (s *Server) handleFileDownload(c *gin.Context) {
	fileId := c.Param("file_id")

	vctx, err := protocol.NewValidateContext(c.Request)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	workerInfo, err := s.db.GetWorkerSessionRepository().GetById(c.Request.Context(), vctx.WorkerId)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err = vctx.ValidateSignature(workerInfo.WorkerToken); err != nil {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if fileId == "" {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "file_id is required"})
		return
	}

	// Download file from storage
	reader, fileInfo, err := s.storage.Download(c.Request.Context(), fileId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "File not found"})
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to download file: " + err.Error()})
		}
		return
	}
	defer reader.Close()

	// Set response headers
	c.Header("Content-Type", fileInfo.ContentType)
	c.Header("Content-Length", strconv.FormatInt(fileInfo.Size, 10))
	c.Header("Content-Disposition", "attachment; filename=\""+fileInfo.Filename+"\"")
	c.Status(http.StatusOK)

	// Stream file content
	_, err = io.Copy(c.Writer, reader)
	if err != nil {
		// Log error but don't send HTTP error as headers are already sent
		// log.Printf("Error streaming file: %v", err)
		return
	}
}
