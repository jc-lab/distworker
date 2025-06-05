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

package websocket

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	blocking_dequeue "github.com/AmrSaber/go-blocking-dequeue"
	"github.com/google/uuid"
	protocol2 "github.com/jc-lab/distworker/go/internal/protocol"
	"github.com/jc-lab/distworker/go/internal/provisioner"
	"github.com/jc-lab/distworker/go/internal/worker"
	"github.com/jc-lab/distworker/go/pkg/controller/config"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const maxBusyLoop = 100

// WorkerManager manages all worker WebSocket connections
type WorkerManager struct {
	ctx    context.Context
	cancel context.CancelFunc

	db                 database.Database
	provisionerManager provisioner.Manager

	connections    map[string]*WorkerConnection
	taskAssignChan chan *models2.Task
	mu             sync.RWMutex

	queueConsuming  bool
	taskAssignQueue *blocking_dequeue.BlockingDequeue[*models2.Task]

	logger *zap.SugaredLogger

	httpClient *http.Client
}

// NewWorkerManager creates a new worker connection manager
func NewWorkerManager(rootCtx context.Context, db database.Database, provisionerManager provisioner.Manager, rootLogger *zap.Logger) *WorkerManager {
	ctx, cancel := context.WithCancel(rootCtx)

	wm := &WorkerManager{
		ctx:    ctx,
		cancel: cancel,

		db:                 db,
		provisionerManager: provisionerManager,
		logger:             rootLogger.Named("worker_manager").Sugar(),

		connections:     make(map[string]*WorkerConnection),
		taskAssignChan:  make(chan *models2.Task, 100),
		taskAssignQueue: blocking_dequeue.NewBlockingDequeue[*models2.Task](make([]*models2.Task, 1024)),

		httpClient: &http.Client{
			Transport: http.DefaultTransport,
		},
	}

	return wm
}

func (wm *WorkerManager) Start() error {
	wm.cleanupGarbage()

	wm.queueConsuming = true
	// Start task assignment goroutine
	go wm.taskAssignmentWorker()

	// Start connection health checker
	go wm.healthCheckWorker()

	return nil
}

// HandleConnection handles a new WebSocket connection from a worker
func (wm *WorkerManager) HandleConnection(conn *websocket.Conn) {
	defer conn.Close()

	// Set connection timeouts
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	workerConn := &WorkerConnection{
		Conn:        conn,
		LastPing:    time.Now(),
		LastPong:    time.Now(),
		SendChannel: make(chan []byte, 10),
		Done:        make(chan struct{}),
	}

	// Start sender goroutine
	go workerConn.sender()

	// Wait for worker registration
	if err := wm.handleWorkerRegistration(workerConn); err != nil {
		log.Printf("Worker registration failed: %v", err)
		return
	}

	// Add to connections map
	wm.mu.Lock()
	wm.connections[workerConn.WorkerId] = workerConn
	wm.mu.Unlock()

	defer func() {
		// Remove from connections map
		wm.mu.Lock()
		delete(wm.connections, workerConn.WorkerId)
		wm.mu.Unlock()

		// Update worker session status
		if workerConn.Session != nil {
			wm.db.GetWorkerSessionRepository().Delete(wm.ctx, workerConn.WorkerId)
		}

		close(workerConn.Done)
		log.Printf("Worker %s disconnected", workerConn.WorkerId)
	}()

	log.Printf("Worker %s connected", workerConn.WorkerId)

	// Handle incoming messages
	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-workerConn.Done:
			return
		default:
			messageType, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Printf("WebSocket error for worker %s: %v", workerConn.WorkerId, err)
				}
				return
			}

			switch messageType {
			case websocket.BinaryMessage:
				if err := wm.handleWorkerMessage(workerConn, data); err != nil {
					log.Printf("Error handling message from worker %s: %v", workerConn.WorkerId, err)
				}
			}
		}
	}
}

func (wm *WorkerManager) GetWorkersInfo() []*models2.Worker {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	result := make([]*models2.Worker, 0, len(wm.connections))
	for _, conn := range wm.connections {
		conn.mu.RLock()
		session := conn.Session
		conn.mu.RUnlock()
		if session != nil {
			result = append(result, session.ToModel())
		}
	}
	return result
}

// WaitForWorkerReady waits for a worker with the specified ID to be ready and connected
func (wm *WorkerManager) WaitForWorkerReady(ctx context.Context, workerId string) (worker.Connection, error) {
	// Check if worker is already connected and ready
	wm.mu.RLock()
	if conn, exists := wm.connections[workerId]; exists {
		wm.mu.RUnlock()
		return conn, nil
	}
	wm.mu.RUnlock()

	// Create a channel to receive worker ready notification
	readyChan := make(chan *WorkerConnection, 1)

	// Start a goroutine to poll for the worker
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond) // Poll every 100 ms
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				wm.mu.RLock()
				if conn, exists := wm.connections[workerId]; exists {
					wm.mu.RUnlock()
					// Verify the connection is healthy
					select {
					case <-conn.Done:
						// Connection is closed, continue waiting
						continue
					default:
						// Connection is active and ready
						readyChan <- conn
						return
					}
				}
				wm.mu.RUnlock()
			}
		}
	}()

	// Wait for either success or error
	select {
	case conn := <-readyChan:
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (wm *WorkerManager) workerIsIdle(workerConn *WorkerConnection) bool {
	return workerConn.Session.CurrentTask == nil
}

// handleWorkerRegistration handles the initial worker registration
func (wm *WorkerManager) handleWorkerRegistration(workerConn *WorkerConnection) error {
	now := models2.Now()

	// Read registration message
	_ = workerConn.Conn.SetReadDeadline(time.Now().Add(time.Second * 30))
	messageType, data, err := workerConn.Conn.ReadMessage()
	_ = workerConn.Conn.SetReadDeadline(time.Time{})
	if err != nil {
		return fmt.Errorf("failed to read registration message: %w", err)
	}

	if messageType != websocket.BinaryMessage {
		return fmt.Errorf("expected binary message for registration")
	}

	// Parse WebSocket message
	var wsMsg protocol2.WebSocketMessage
	if err := proto.Unmarshal(data, &wsMsg); err != nil {
		return fmt.Errorf("failed to unmarshal registration message: %w", err)
	}

	if wsMsg.Type != protocol2.MessageType_MESSAGE_TYPE_WORKER_REGISTER {
		return fmt.Errorf("expected worker registration message, got: %v", wsMsg.Type)
	}

	register := wsMsg.GetWorkerRegister()
	if register == nil {
		return fmt.Errorf("worker register payload is nil")
	}

	// Validate HMAC signature
	var signedRegister protocol2.SignedWorkerRegister
	if err := proto.Unmarshal(register.Data, &signedRegister); err != nil {
		return fmt.Errorf("failed to unmarshal signed registration: %w", err)
	}

	var prov provisioner.Provisioner
	var session *models2.WorkerSession
	if signedRegister.WorkerId != "" {
		session, err = wm.db.GetWorkerSessionRepository().GetById(wm.ctx, signedRegister.GetWorkerId())
		if err != nil {
			return fmt.Errorf("worker not found (id=%s): %+v", signedRegister.WorkerId, err)
		}
		if err = protocol2.ValidateWebSocketSignature(session.WorkerToken, signedRegister.GetDate(), register.Data, register.Signature); err != nil {
			return errors.Wrap(err, "failed to validate signature")
		}

		prov = wm.provisionerManager.GetProvisioner(signedRegister.GetProvisionerName())
		if prov == nil {
			return fmt.Errorf("worker provisioner(\"%s\") not found", signedRegister.GetProvisionerName())
		}

		// Update session status
		session.Health = types.WorkerHealthUp
		session.LastHeartbeat = models2.Now()
		session.ConnectedAt = models2.Now()

		if err := wm.db.GetWorkerSessionRepository().Update(wm.ctx, session); err != nil {
			return fmt.Errorf("failed to update worker session: %w", err)
		}
	} else {
		prov = wm.provisionerManager.GetProvisioner(signedRegister.GetProvisionerName())
		if prov == nil {
			return fmt.Errorf("worker provisioner(\"%s\") not found", signedRegister.GetProvisionerName())
		}

		provSetting := prov.GetSetting()
		if provSetting.Type != config.ProvisionerTypeNone {
			return fmt.Errorf("no worker id")
		}
		if err = protocol2.ValidateWebSocketSignature(provSetting.Token, signedRegister.GetDate(), register.Data, register.Signature); err != nil {
			return err
		}

		session = &models2.WorkerSession{
			WorkerId:        prov.GetName() + "-" + uuid.NewString(),
			ProvisionerName: prov.GetName(),
			ConnectedAt:     now,
			Health:          types.WorkerHealthUp,
			ResourceInfo:    types.ResourceInfoFromProto(signedRegister.ResourceInfo),
			WorkerToken:     provSetting.Token,
			Queues:          provSetting.Queues,
			LastHeartbeat:   now,
		}

		if err = wm.db.GetWorkerSessionRepository().Create(wm.ctx, session); err != nil {
			return errors.Wrap(err, "worker create failed")
		}
	}

	// Update worker connection and session
	workerConn.WorkerId = session.WorkerId
	workerConn.Session = session

	return workerConn.sendPayload(&protocol2.WebSocketMessage{
		Type: protocol2.MessageType_MESSAGE_TYPE_WORKER_REGISTER_RESPONSE,
		Payload: &protocol2.WebSocketMessage_WorkerRegisterResponse{
			WorkerRegisterResponse: &protocol2.WorkerRegisterResponse{
				IdleTime: int32(prov.GetSetting().Autoscale.IdleTimeout.Milliseconds() / 1000),
			},
		},
	})
}

// handleWorkerMessage processes incoming messages from workers
func (wm *WorkerManager) handleWorkerMessage(workerConn *WorkerConnection, data []byte) error {
	var wsMsg protocol2.WebSocketMessage
	if err := proto.Unmarshal(data, &wsMsg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	switch wsMsg.Type {
	case protocol2.MessageType_MESSAGE_TYPE_HEARTBEAT:
		return wm.handleHeartbeat(workerConn, wsMsg.GetHeartbeat())
	case protocol2.MessageType_MESSAGE_TYPE_TASK_PROGRESS:
		return wm.handleTaskProgress(workerConn, wsMsg.GetTaskProgress())
	case protocol2.MessageType_MESSAGE_TYPE_TASK_COMPLETE:
		return wm.handleTaskComplete(workerConn, wsMsg.GetTaskComplete())
	case protocol2.MessageType_MESSAGE_TYPE_TASK_FAILED:
		return wm.handleTaskFailed(workerConn, wsMsg.GetTaskFailed())
	default:
		return fmt.Errorf("unknown message type: %v", wsMsg.Type)
	}
}

// handleHeartbeat processes heartbeat messages
func (wm *WorkerManager) handleHeartbeat(workerConn *WorkerConnection, heartbeat *protocol2.Heartbeat) error {
	if heartbeat == nil {
		return fmt.Errorf("heartbeat payload is nil")
	}

	workerConn.mu.Lock()
	workerConn.LastPong = time.Now()
	workerConn.mu.Unlock()

	// Update worker session
	workerConn.Session.Health = types.ConvertWorkerHealthFromProto(heartbeat.Health)
	workerConn.Session.LastHeartbeat = models2.Now()

	// Update resource usage if provided
	if heartbeat.ResourceUsage != nil {
		resourceMap := make(map[string]interface{})
		if existingInfo := workerConn.Session.ResourceInfo; existingInfo != nil {
			// Copy existing resource info
			for k, v := range existingInfo {
				resourceMap[k] = v
			}
		}

		// Add current usage
		resourceMap["cpu_percent"] = heartbeat.ResourceUsage.CpuPercent
		resourceMap["memory_used_mb"] = heartbeat.ResourceUsage.MemoryUsedMb
		resourceMap["gpu_utilization"] = heartbeat.ResourceUsage.GpuUtilization

		if heartbeat.ResourceUsage.Additional != nil {
			for k, v := range heartbeat.ResourceUsage.Additional.AsMap() {
				resourceMap[k] = v
			}
		}

		workerConn.Session.ResourceInfo = resourceMap
	}

	return wm.db.GetWorkerSessionRepository().UpdateHeartbeat(wm.ctx, workerConn.WorkerId, workerConn.Session.Health)
}

// handleTaskProgress processes task progress updates
func (wm *WorkerManager) handleTaskProgress(workerConn *WorkerConnection, progress *protocol2.TaskProgress) error {
	if progress == nil {
		return fmt.Errorf("task progress payload is nil")
	}

	// Get task from database
	task, err := wm.db.GetTaskRepository().GetById(wm.ctx, progress.TaskId)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Verify worker owns this task
	if task.WorkerId != workerConn.WorkerId {
		return fmt.Errorf("worker does not own this task")
	}

	// Update task with progress information
	if task.Metadata == nil {
		task.Metadata = make(map[string]interface{})
	}
	task.Metadata["progress"] = progress.Progress
	task.Metadata["progress_message"] = progress.Message

	if progress.Data != nil {
		task.Metadata["progress_data"] = progress.Data.AsMap()
	}

	return wm.db.GetTaskRepository().Update(wm.ctx, task)
}

// handleTaskComplete processes task completion
func (wm *WorkerManager) handleTaskComplete(workerConn *WorkerConnection, complete *protocol2.TaskComplete) error {
	if complete == nil {
		return fmt.Errorf("task complete payload is nil")
	}

	// Get task from database
	task, err := wm.db.GetTaskRepository().GetById(wm.ctx, complete.TaskId)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Verify worker owns this task
	if task.WorkerId != workerConn.WorkerId {
		return fmt.Errorf("worker does not own this task")
	}

	// Update worker session
	workerConn.Session.CurrentTask = nil

	task.Error = nil
	if complete.Result != nil {
		task.Result = complete.Result.AsMap()
	}

	// Convert result files
	if len(complete.ResultFiles) > 0 {
		task.ResultFiles = make([]models2.FileInfo, len(complete.ResultFiles))
		for i, file := range complete.ResultFiles {
			task.ResultFiles[i] = models2.FileInfo{
				FileId:      file.FileId,
				Filename:    file.Filename,
				ContentType: file.ContentType,
				Size:        file.Size,
				StorageURL:  file.StorageUrl,
			}
		}
	}

	wm.taskComplete(task)

	log.Printf("Task %s completed by worker %s", complete.TaskId, workerConn.WorkerId)

	return nil
}

// handleTaskFailed processes task failure
func (wm *WorkerManager) handleTaskFailed(workerConn *WorkerConnection, failed *protocol2.TaskFailed) error {
	if failed == nil {
		return fmt.Errorf("task failed payload is nil")
	}

	// Get task from database
	task, err := wm.db.GetTaskRepository().GetById(wm.ctx, failed.TaskId)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Verify worker owns this task
	if task.WorkerId != workerConn.WorkerId {
		return fmt.Errorf("worker does not own this task")
	}

	task.Error = &models2.TaskError{
		Code:    types.TaskErrorCode(failed.ErrorCode),
		Message: failed.ErrorMessage,
	}
	if failed.ErrorDetails != nil {
		task.Error.Details = failed.ErrorDetails.AsMap()
	}

	// Update worker session
	workerConn.Session.CurrentTask = nil

	wm.taskComplete(task)

	log.Printf("Task %s failed on worker %s: %s", failed.TaskId, workerConn.WorkerId, failed.ErrorMessage)
	return nil
}

// RequestAssignTask assigns a task to an available worker
func (wm *WorkerManager) RequestAssignTask(task *models2.Task) {
	wm.taskAssignQueue.PushBack(task)
}

func (wm *WorkerManager) findGrowableProvisionerByQueue(queue string, taskDelay time.Duration) []provisioner.Provisioner {
	var plist []provisioner.Provisioner
	for _, prov := range wm.provisionerManager.GetProvisioners() {
		if slices.Contains(prov.GetSetting().Queues, queue) && prov.IsGrowable(taskDelay) {
			plist = append(plist, prov)
		}
	}
	return plist
}

// workerMatchesQueue checks if worker's queue patterns match the task queue
func (wm *WorkerManager) workerMatchesQueue(session *models2.WorkerSession, queue string) bool {
	for _, name := range session.Queues {
		if name == queue {
			return true
		}
	}
	return false
}

// sendTaskAssignment sends a task assignment to a worker
func (wm *WorkerManager) sendTaskAssignment(conn *WorkerConnection, task *models2.Task) error {
	// Convert task to protocol message
	taskAssign := &protocol2.TaskAssign{
		TaskId:    task.Id,
		Queue:     task.Queue,
		TimeoutMs: task.TimeoutMS,
	}

	// Convert metadata
	if task.Metadata != nil {
		metadata, err := structpb.NewStruct(task.Metadata)
		if err != nil {
			return fmt.Errorf("failed to convert metadata: %w", err)
		}
		taskAssign.Metadata = metadata
	}

	// Convert input
	if task.Input != nil {
		input, err := structpb.NewStruct(task.Input)
		if err != nil {
			return fmt.Errorf("failed to convert input: %w", err)
		}
		taskAssign.Input = input
	}

	// Convert files
	if len(task.Files) > 0 {
		taskAssign.Files = make([]*protocol2.FileInfo, len(task.Files))
		for i, file := range task.Files {
			taskAssign.Files[i] = &protocol2.FileInfo{
				FileId:      file.FileId,
				Filename:    file.Filename,
				ContentType: file.ContentType,
				Size:        file.Size,
				StorageUrl:  file.StorageURL,
			}
		}
	}

	// Create WebSocket message
	wsMsg := &protocol2.WebSocketMessage{
		Type: protocol2.MessageType_MESSAGE_TYPE_TASK_ASSIGN,
		Payload: &protocol2.WebSocketMessage_TaskAssign{
			TaskAssign: taskAssign,
		},
	}

	if err := conn.sendPayload(wsMsg); err != nil {
		return err
	}
	// Update task and worker session
	task.Status = types.TaskStatusProcessing
	task.WorkerId = conn.WorkerId
	task.StartedAt = models2.NowPtr()

	conn.Session.CurrentTask = task

	// Update database
	if err := wm.db.GetTaskRepository().Update(wm.ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	log.Printf("Task %s assigned to worker %s", task.Id, conn.WorkerId)
	return nil
}

// taskAssignmentWorker continuously tries to assign pending tasks
func (wm *WorkerManager) taskAssignmentWorker() {
	go wm.assignPendingTasks()

	go func() {
		<-wm.ctx.Done()
		wm.taskAssignQueue.PushBack(nil)
	}()

	for wm.ctx.Err() == nil {
		i := 0
		for !wm.taskAssignQueue.IsEmpty() && wm.ctx.Err() == nil && i < maxBusyLoop {
			i++

			task := wm.taskAssignQueue.PopFront()
			if task == nil {
				break
			}
			if task.Status == types.TaskStatusCompleted {
				wm.asyncTaskFinish(task)
			} else if task.Status != types.TaskStatusPending {
				wm.logger.Warnf("TASK[%s] status is not pending (%s)", task.Id, task.Status)
				continue
			}
			if err := wm.tryAssignTask(task); err != nil {
				wm.logger.Debugf("TASK[%s] tryAssignTask failed: %+v", task.Id, err)
				wm.taskAssignQueue.PushBack(task) // Requeue
			} else {
				wm.logger.Warnf("TASK[%s] tryAssignTask success", task.Id)
			}
		}

		select {
		case <-wm.ctx.Done():
		case <-time.After(time.Millisecond * 10):
		}
	}
}

func (wm *WorkerManager) taskComplete(task *models2.Task) {
	taskRepository := wm.db.GetTaskRepository()

	task.CompletedAt = models2.NowPtr()

	if task.Error != nil {
		if task.Retried >= task.MaxRetry {
			task.Status = types.TaskStatusCompleted
		} else {
			task.Status = types.TaskStatusPending
			task.Retried++
		}
	} else {
		task.Status = types.TaskStatusCompleted
	}

	_ = taskRepository.Update(wm.ctx, task)

	if task.Status == types.TaskStatusPending && wm.queueConsuming {
		wm.RequestAssignTask(task)
	} else {
		wm.asyncTaskFinish(task)
	}
}

func (wm *WorkerManager) taskFinish(task *models2.Task) error {
	taskRepository := wm.db.GetTaskRepository()

	task.Status = types.TaskStatusFinished
	if task.WebhookUrl != "" {
		raw, err := json.Marshal(task)
		if err != nil {
			wm.logger.Warnf("failed to marshal task to json: %s", err)
			return err
		}
		request, err := http.NewRequestWithContext(wm.ctx, http.MethodPost, task.WebhookUrl, bytes.NewReader(raw))
		if err != nil {
			wm.logger.Warnf("failed to build request to webhook: %s", err)
			return err
		}
		response, err := wm.httpClient.Do(request)
		if err != nil {
			wm.logger.Warnf("failed to send request to webhook: %s", err)
			return err
		}
		_ = response.Body.Close()
	}

	return taskRepository.Update(wm.ctx, task)
}

// TODO fixed concurrently worker
func (wm *WorkerManager) asyncTaskFinish(task *models2.Task) {
	go func() {
		if err := wm.taskFinish(task); err != nil {
			wm.logger.Warnf("TASK[%s] task finish failed: %+v", task.Id, err)
			wm.RequestAssignTask(task)
		}
	}()
}

// assignPendingTasks tries to assign pending tasks to available workers
func (wm *WorkerManager) assignPendingTasks() {
	// Get pending tasks
	tasks, err := wm.db.GetTaskRepository().ListAll(wm.ctx, database.TaskFilter{
		StatusGte: types.TaskStatusPending,
		StatusLt:  types.TaskStatusPending + 1,
	})
	if err != nil {
		log.Printf("Failed to get pending tasks: %v", err)
		return
	}

	for task := range tasks {
		log.Printf("ADD PENDING QUEUE: %+v", task)
		wm.taskAssignQueue.PushBack(task)
	}
}

func (wm *WorkerManager) tryAssignTask(task *models2.Task) error {
	wm.mu.RLock()

	// Find available worker that matches queue patterns
	for _, conn := range wm.connections {
		if wm.workerMatchesQueue(conn.Session, task.Queue) && wm.workerIsIdle(conn) {
			if err := wm.sendTaskAssignment(conn, task); err != nil {
				wm.logger.Warnf("Failed to send task assignment to worker: %v", err)
			} else {
				wm.mu.RUnlock()
				return nil
			}
		}
	}

	wm.mu.RUnlock()

	taskDelay := time.Since(task.CreatedAt.Time)
	for _, prov := range wm.findGrowableProvisionerByQueue(task.Queue, taskDelay) {
		result := func() bool {
			ctx, cancel := context.WithTimeout(wm.ctx, time.Second*60)
			defer cancel()

			if workerConn, err := prov.Grow(ctx); err != nil {
				wm.logger.Warnf("Failed to grow provisioner: %v", err)
				return false
			} else if err := wm.sendTaskAssignment(workerConn.(*WorkerConnection), task); err != nil {
				wm.logger.Warnf("Failed to send task assignment to worker: %v", err)
				return false
			} else {
				return true
			}
		}()
		if result {
			return nil
		}
	}

	// No available worker found
	return errors.New("no available worker")
}

// healthCheckWorker periodically checks worker connection health
func (wm *WorkerManager) healthCheckWorker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-ticker.C:
			wm.checkWorkerHealth()
		}
	}
}

// checkWorkerHealth checks the health of all worker connections
func (wm *WorkerManager) checkWorkerHealth() {
	wm.mu.RLock()
	connections := make([]*WorkerConnection, 0, len(wm.connections))
	for _, conn := range wm.connections {
		connections = append(connections, conn)
	}
	wm.mu.RUnlock()

	for _, conn := range connections {
		conn.mu.RLock()
		lastPong := conn.LastPong
		conn.mu.RUnlock()

		// If no pong received in 90 seconds, consider connection dead
		if time.Since(lastPong) > 90*time.Second {
			log.Printf("Worker %s appears to be dead, closing connection", conn.WorkerId)
			conn.Conn.Close()
		}
	}
}

func (wm *WorkerManager) cleanupGarbage() {
	workerSessionRepository := wm.db.GetWorkerSessionRepository()
	taskRepository := wm.db.GetTaskRepository()

	sessions, err := workerSessionRepository.List(wm.ctx)
	if err != nil {
		wm.logger.Warnf("Failed to list worker sessions: %v", err)
		return
	}
	for _, session := range sessions {
		// TOOD: shutdown
		_ = workerSessionRepository.Delete(wm.ctx, session.WorkerId)
	}

	tasks, err := taskRepository.ListAll(wm.ctx, database.TaskFilter{
		StatusGte: types.TaskStatusProcessing,
		StatusLt:  types.TaskStatusProcessing + 1,
	})
	if err != nil {
		wm.logger.Warnf("Failed to list tasks: %v", err)
		return
	}
	for task := range tasks {
		task.Error = &models2.TaskError{
			Code:    types.TaskErrorCodeCancelled,
			Message: "controller restarted",
		}
		wm.taskComplete(task)
	}
}

// Shutdown gracefully shuts down the worker manager
func (wm *WorkerManager) Shutdown() {
	wm.cancel()

	wm.mu.RLock()
	for _, conn := range wm.connections {
		conn.Conn.Close()
	}
	wm.mu.RUnlock()

	// TODO: wait for gracefully shutdown
}

// sender handles sending messages to the worker
func (conn *WorkerConnection) sender() {
	pingTicker := time.NewTicker(3 * time.Second)
	defer pingTicker.Stop()

	for {
		select {
		case data := <-conn.SendChannel:
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.Conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				log.Printf("Failed to send message to worker %s: %v", conn.WorkerId, err)
				return
			}
		case <-pingTicker.C:
			// Send ping
			conn.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
			conn.mu.Lock()
			conn.LastPing = time.Now()
			conn.mu.Unlock()
		case <-conn.Done:
			return
		}
	}
}
