package controller

import (
	"encoding/json"
	"github.com/jc-lab/distworker/go/internal/protocol"
	"github.com/jc-lab/distworker/go/internal/version"
	"github.com/jc-lab/distworker/go/pkg/api"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	"github.com/jc-lab/distworker/go/pkg/healthchecker"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"

	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// handleCreateTask handles POST /api/v1/tasks
func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	// Check if it's multipart form data (file upload)
	if strings.HasPrefix(contentType, "multipart/form-data") {
		s.handleCreateTaskWithFiles(w, r)
		return
	}

	// Handle JSON request
	var request api.CreateTaskRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if request.Queue == "" {
		http.Error(w, "queue is required", http.StatusBadRequest)
		return
	}

	// Parse timeout
	timeoutMS := int64(0)
	if request.Timeout != "" {
		duration, err := time.ParseDuration(request.Timeout)
		if err != nil {
			http.Error(w, "Invalid timeout format", http.StatusBadRequest)
			return
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
	}

	if err := s.db.GetTaskRepository().Create(r.Context(), task); err != nil {
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// Try to assign the task to an available worker
	go func() {
		if err := s.workerManager.AssignTask(task); err != nil {
			// Task will remain pending and be picked up by the assignment worker
		}
	}()

	writeJson(w, http.StatusOK, task)
}

// handleCreateTaskWithFiles handles POST /api/v1/tasks with file uploads
func (s *Server) handleCreateTaskWithFiles(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(32 << 20) // 32MB max memory
	if err != nil {
		http.Error(w, "Failed to parse multipart form", http.StatusBadRequest)
		return
	}

	defer func() {
		_ = r.MultipartForm.RemoveAll()
	}()

	// Get task data from form
	taskDataStr := r.FormValue("task")
	if taskDataStr == "" {
		http.Error(w, "task field is required", http.StatusBadRequest)
		return
	}

	// Parse task JSON
	var request api.CreateTaskRequest

	if err := json.Unmarshal([]byte(taskDataStr), &request); err != nil {
		http.Error(w, "Invalid task JSON", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if request.Queue == "" {
		http.Error(w, "queue is required", http.StatusBadRequest)
		return
	}

	// Parse timeout
	timeoutMS := int64(0)
	if request.Timeout != "" {
		duration, err := time.ParseDuration(request.Timeout)
		if err != nil {
			http.Error(w, "Invalid timeout format", http.StatusBadRequest)
			return
		}
		timeoutMS = duration.Milliseconds()
	}

	// Handle file uploads
	var fileInfos []models2.FileInfo
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for fieldName, fileHeaders := range r.MultipartForm.File {
			for _, fileHeader := range fileHeaders {
				fileInfo, err := s.uploadFile(r, fieldName, fileHeader)
				if err != nil {
					http.Error(w, "Failed to upload file: "+err.Error(), http.StatusInternalServerError)
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

	if err := s.db.GetTaskRepository().Create(r.Context(), task); err != nil {
		http.Error(w, "Failed to create task", http.StatusInternalServerError)
		return
	}

	// Try to assign the task to an available worker
	go func() {
		if err := s.workerManager.AssignTask(task); err != nil {
			// Task will remain pending and be picked up by the assignment worker
		}
	}()

	writeJson(w, http.StatusOK, task)
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
func (s *Server) handleGetTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskId := vars["task_id"]

	task, err := s.db.GetTaskRepository().GetById(r.Context(), taskId)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	writeJson(w, http.StatusOK, task)
}

// handleListTasks handles GET /api/v1/tasks
func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	queue := r.URL.Query().Get("queue")
	status := r.URL.Query().Get("status")
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

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
		Queue:  queue,
		Status: status,
	}

	tasks, total, err := s.db.GetTaskRepository().List(r.Context(), filter, page, limit)
	if err != nil {
		http.Error(w, "Failed to list tasks", http.StatusInternalServerError)
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

	writeJson(w, http.StatusOK, response)
}

// handleDeleteTask handles DELETE /api/v1/tasks/{task_id}
func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	taskId := vars["task_id"]

	// Get task first to check if it exists and can be cancelled
	task, err := s.db.GetTaskRepository().GetById(r.Context(), taskId)
	if err != nil {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	// Only allow cancellation of pending or processing tasks
	if task.Status != types.TaskStatusPending && task.Status != types.TaskStatusProcessing {
		http.Error(w, "Task cannot be cancelled", http.StatusBadRequest)
		return
	}

	// Update task status to cancelled
	task.Status = types.TaskStatusCancelled
	task.CompletedAt = models2.NowPtr()

	if err := s.db.GetTaskRepository().Update(r.Context(), task); err != nil {
		http.Error(w, "Failed to cancel task", http.StatusInternalServerError)
		return
	}

	response := &api.DeleteTaskResponse{
		TaskId: taskId,
		Status: string(types.TaskStatusCancelled),
	}

	writeJson(w, http.StatusOK, response)
}

// handleCreateQueue handles POST /api/v1/queues
func (s *Server) handleCreateQueue(w http.ResponseWriter, r *http.Request) {
	var request api.CreateQueueRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if request.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	queue := &models2.Queue{
		Name:        request.Name,
		Description: request.Description,
		Status:      types.QueueStatusInactive,
		CreatedAt:   models2.Now(),
		UpdatedAt:   models2.Now(),
	}

	if err := s.db.GetQueueRepository().Create(r.Context(), queue); err != nil {
		http.Error(w, "Failed to create queue", http.StatusInternalServerError)
		return
	}

	writeJson(w, http.StatusOK, queue)
}

// handleListQueues handles GET /api/v1/queues
func (s *Server) handleListQueues(w http.ResponseWriter, r *http.Request) {
	queues, err := s.db.GetQueueRepository().List(r.Context())
	if err != nil {
		http.Error(w, "Failed to list queues", http.StatusInternalServerError)
		return
	}

	response := &api.ListQueuesResponse{
		Queues: queues,
	}

	writeJson(w, http.StatusOK, response)
}

// handleGetQueue handles GET /api/v1/queues/{queue_name}
func (s *Server) handleGetQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queueName := vars["queue_name"]

	queue, err := s.db.GetQueueRepository().GetByName(r.Context(), queueName)
	if err != nil {
		http.Error(w, "Queue not found", http.StatusNotFound)
		return
	}

	writeJson(w, http.StatusOK, queue)
}

// handleUpdateQueue handles PUT /api/v1/queues/{queue_name}
func (s *Server) handleUpdateQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queueName := vars["queue_name"]

	queue, err := s.db.GetQueueRepository().GetByName(r.Context(), queueName)
	if err != nil {
		http.Error(w, "Queue not found", http.StatusNotFound)
		return
	}

	var request api.UpdateQueueRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	queue.Description = request.Description
	queue.UpdatedAt = models2.Now()

	if err := s.db.GetQueueRepository().Update(r.Context(), queue); err != nil {
		http.Error(w, "Failed to update queue", http.StatusInternalServerError)
		return
	}

	writeJson(w, http.StatusOK, queue)
}

// handleDeleteQueue handles DELETE /api/v1/queues/{queue_name}
func (s *Server) handleDeleteQueue(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	queueName := vars["queue_name"]

	if err := s.db.GetQueueRepository().Delete(r.Context(), queueName); err != nil {
		http.Error(w, "Failed to delete queue", http.StatusInternalServerError)
		return
	}

	response := &api.DeleteQueueResponse{
		Status: "deleted",
	}

	writeJson(w, http.StatusOK, response)
}

// handleGetQueueStats handles GET /api/v1/queues/{queue_name}/stats
func (s *Server) handleGetQueueStats(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement queue statistics
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleListWorkers handles GET /api/v1/workers
func (s *Server) handleListWorkers(w http.ResponseWriter, r *http.Request) {
	sessions, err := s.db.GetWorkerSessionRepository().List(r.Context())
	if err != nil {
		http.Error(w, "Failed to list workers", http.StatusInternalServerError)
		return
	}

	workers := make([]*models2.Worker, len(sessions))
	for i, session := range sessions {
		workers[i] = &models2.Worker{
			Id:            session.WorkerId,
			Status:        session.Status,
			Queues:        session.Queues,
			CurrentTask:   session.CurrentTask,
			ConnectedAt:   session.ConnectedAt,
			LastHeartbeat: session.LastHeartbeat,
			Provisioner:   session.ProvisionerName,
			ResourceInfo:  session.ResourceInfo,
		}
	}

	response := &api.ListWorkersResponse{
		Workers: workers,
	}

	writeJson(w, http.StatusOK, response)
}

// handleDeleteWorker handles DELETE /api/v1/workers/{worker_id}
func (s *Server) handleDeleteWorker(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement worker disconnection
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleListProvisioners handles GET /api/v1/provisioners
func (s *Server) handleListProvisioners(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement provisioner listing
	http.Error(w, "Not implemented", http.StatusNotImplemented)
}

// handleHealth handles GET /health
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	response := &api.HealthResponse{
		Timestamp: time.Now().UnixMilli(),
		Version:   version.Version,
		Details:   make(map[string]*api.HealthDetail),
	}

	response.Details, response.Status = healthchecker.Check(r.Context(), []healthchecker.Checkable{
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
		writeJson(w, http.StatusServiceUnavailable, response)
	} else {
		writeJson(w, http.StatusOK, response)
	}
}

// handleMetrics handles GET /metrics
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Collect latest metrics from database before serving
	if s.metrics != nil {
		s.metrics.CollectDatabaseStats(r.Context(), s.db)
	}

	// Serve Prometheus metrics
	s.metricsHandler.ServeHTTP(w, r)
}

// handleWorkerWebSocket handles WebSocket connections from workers
func (s *Server) handleWorkerWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade HTTP connection to WebSocket
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade to WebSocket", http.StatusBadRequest)
		return
	}

	// Hand off connection to worker manager
	go s.workerManager.HandleConnection(conn)
}

// handleFileDownload handles GET /worker/v1/file/{file_id}
func (s *Server) handleFileDownload(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileId := vars["file_id"]

	vctx, err := protocol.NewValidateContext(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	workerInfo, err := s.db.GetWorkerSessionRepository().GetById(r.Context(), vctx.WorkerId)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = vctx.ValidateSignature(workerInfo.WorkerToken); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if fileId == "" {
		http.Error(w, "file_id is required", http.StatusBadRequest)
		return
	}

	// Download file from storage
	reader, fileInfo, err := s.storage.Download(r.Context(), fileId)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "File not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to download file: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	defer reader.Close()

	// Set response headers
	w.Header().Set("Content-Type", fileInfo.ContentType)
	w.Header().Set("Content-Length", strconv.FormatInt(fileInfo.Size, 10))
	w.Header().Set("Content-Disposition", "attachment; filename=\""+fileInfo.Filename+"\"")

	w.WriteHeader(http.StatusOK)

	// Stream file content
	_, err = io.Copy(w, reader)
	if err != nil {
		// Log error but don't send HTTP error as headers are already sent
		// log.Printf("Error streaming file: %v", err)
		return
	}
}

func writeJson(w http.ResponseWriter, statusCode int, data interface{}) {
	bytes, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Length", strconv.FormatInt(int64(len(bytes)), 10))
	w.WriteHeader(statusCode)
	_, _ = w.Write(bytes)
}
