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

package worker

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
	"github.com/jc-lab/distworker/go/pkg/controller/eventbus"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"hash/fnv"
	"log"
	"net/http"
	"slices"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

const maxBusyLoop = 100
const queueBuckets = 8

// Manager manages all worker WebSocket connections
type Manager struct {
	ctx    context.Context
	cancel context.CancelFunc

	db                 database.Database
	provisionerManager provisioner.Manager

	connections    map[string]Connection
	taskAssignChan chan *models2.Task
	mu             sync.RWMutex

	queueConsuming   bool
	taskAssignQueues [queueBuckets]*blocking_dequeue.BlockingDequeue[*models2.Task]
	taskEventBus     eventbus.Bus[*models2.Task]

	processingTasks map[string]*models2.Task

	logger *zap.SugaredLogger

	httpClient *http.Client
}

// NewManager creates a new worker connection manager
func NewManager(rootCtx context.Context, db database.Database, provisionerManager provisioner.Manager, rootLogger *zap.Logger) *Manager {
	ctx, cancel := context.WithCancel(rootCtx)

	wm := &Manager{
		ctx:    ctx,
		cancel: cancel,

		db:                 db,
		provisionerManager: provisionerManager,
		logger:             rootLogger.Named("worker_manager").Sugar(),

		connections:     make(map[string]Connection),
		taskAssignChan:  make(chan *models2.Task, 100),
		taskEventBus:    eventbus.New[*models2.Task](),
		processingTasks: make(map[string]*models2.Task),

		httpClient: &http.Client{
			Transport: http.DefaultTransport,
		},
	}

	// Initialize queue buckets
	for i := 0; i < queueBuckets; i++ {
		wm.taskAssignQueues[i] = blocking_dequeue.NewBlockingDequeue[*models2.Task](make([]*models2.Task, 100)) // TODO: FIX ME
	}

	return wm
}

func (wm *Manager) Start() error {
	wm.cleanupGarbage()

	wm.queueConsuming = true

	go wm.assignPendingTasks()

	// Start multiple task assignment workers for each bucket
	for i := 0; i < queueBuckets; i++ {
		go wm.taskAssignmentWorkerBucket(i)
	}

	// Start connection health checker
	go wm.healthCheckWorker()

	return nil
}

// getQueueBucket returns the appropriate queue bucket index for a given queue name
func (wm *Manager) getQueueBucket(queueName string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(queueName))
	return int(hash.Sum32() % queueBuckets)
}

// EnqueueTask adds a task to the appropriate queue bucket based on queue name
func (wm *Manager) EnqueueTask(task *models2.Task) {
	bucketIndex := wm.getQueueBucket(task.Queue)
	wm.taskAssignQueues[bucketIndex].PushBack(task)
}

// taskAssignmentWorkerBucket processes tasks from a specific bucket
func (wm *Manager) taskAssignmentWorkerBucket(bucketIndex int) {
	queue := wm.taskAssignQueues[bucketIndex]

	go func() {
		<-wm.ctx.Done()
		queue.PushBack(nil)
	}()

	i := 0
	for wm.queueConsuming {
		select {
		case <-wm.ctx.Done():
			return
		default:
			if i++; i >= maxBusyLoop {
				time.Sleep(time.Millisecond * 100)
				i = 0
			}

			task := queue.PopFront()
			if task != nil {
				err := wm.processTaskAssignment(task)
				if err != nil {
					wm.logger.Errorf("Failed to process task assignment in bucket %d: %v", bucketIndex, err)
					wm.EnqueueTask(task)
				}
			}
		}
	}
}

func (wm *Manager) findSuitableConns(task *models2.Task) []Connection {
	var suitableConns []Connection

	wm.mu.RLock()
	defer wm.mu.RUnlock()

	for _, conn := range wm.connections {
		if wm.isWorkerSuitableForTask(conn, task) {
			suitableConns = append(suitableConns, conn)
			break
		}
	}

	return suitableConns
}

// processTaskAssignment handles the actual task assignment logic
func (wm *Manager) processTaskAssignment(task *models2.Task) error {
	log.Printf("processTaskAssignment id: %s", task.Id)

	// Find suitable worker for the task
	suitableConns := wm.findSuitableConns(task)

	// try Assign task to worker
	var err error
	for _, conn := range suitableConns {
		if err = wm.assignTaskToWorker(conn, task); err == nil {
			return nil
		}
	}

	taskDelay := time.Since(task.CreatedAt.Time)
	for _, prov := range wm.findGrowableProvisionerByQueue(task.Queue, taskDelay) {
		result := func() bool {
			ctx, cancel := context.WithTimeout(wm.ctx, time.Second*60)
			defer cancel()

			log.Printf("Grow with %s", prov.GetName())
			if conn, err := prov.Grow(ctx); err != nil {
				wm.logger.Warnf("Failed to grow provisioner: %v", err)
				return false
			} else if err := wm.assignTaskToWorker(conn.(Connection), task); err != nil {
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

// isWorkerSuitableForTask checks if a worker is suitable for a specific task
func (wm *Manager) isWorkerSuitableForTask(conn Connection, task *models2.Task) bool {
	availableQueues := conn.GetSession().Queues
	return slices.Contains(availableQueues, task.Queue)
}

// assignTaskToWorker assigns a task to a specific worker
func (wm *Manager) assignTaskToWorker(conn Connection, task *models2.Task) error {
	session := conn.GetSession()
	if !session.AssignTask(task) {
		return errors.New("already have assigned task")
	}
	err := wm.sendTaskAssignment(conn, task)
	if err != nil {
		log.Printf("Task[%s] Failed to send task assignment to Worker[%s]: %v", task.Id, conn.GetWorkerId(), err)
		session.ClearTask()
	}
	return err
}

func (wm *Manager) HandleRegister(conn Connection, register *protocol2.WorkerRegister) (*models2.WorkerSession, *protocol2.WorkerRegisterResponse, error) {
	var err error

	now := models2.Now()

	// Validate HMAC signature
	var signedRegister protocol2.SignedWorkerRegister
	if err = proto.Unmarshal(register.Data, &signedRegister); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal signed registration: %w", err)
	}

	var prov provisioner.Provisioner
	var session *models2.WorkerSession
	if signedRegister.WorkerId != "" {
		session, err = wm.db.GetWorkerSessionRepository().GetById(wm.ctx, signedRegister.GetWorkerId())
		if err != nil {
			return nil, nil, fmt.Errorf("worker not found (id=%s): %+v", signedRegister.WorkerId, err)
		}
		if err = protocol2.ValidateWebSocketSignature(session.WorkerToken, signedRegister.GetDate(), register.Data, register.Signature); err != nil {
			return nil, nil, errors.Wrap(err, "failed to validate signature")
		}

		prov = wm.provisionerManager.GetProvisioner(signedRegister.GetProvisionerName())
		if prov == nil {
			return nil, nil, fmt.Errorf("worker provisioner(\"%s\") not found", signedRegister.GetProvisionerName())
		}

		// Update session status
		session.Health = types.WorkerHealthUp
		session.LastHeartbeat = models2.Now()
		session.ConnectedAt = models2.Now()

		if err := wm.db.GetWorkerSessionRepository().Update(wm.ctx, session); err != nil {
			return nil, nil, fmt.Errorf("failed to update worker session: %w", err)
		}
	} else {
		prov = wm.provisionerManager.GetProvisioner(signedRegister.GetProvisionerName())
		if prov == nil {
			return nil, nil, fmt.Errorf("worker provisioner(\"%s\") not found", signedRegister.GetProvisionerName())
		}

		provSetting := prov.GetSetting()
		if provSetting.Type != config.ProvisionerTypeNone {
			return nil, nil, fmt.Errorf("no worker id")
		}
		if err = protocol2.ValidateWebSocketSignature(provSetting.Token, signedRegister.GetDate(), register.Data, register.Signature); err != nil {
			return nil, nil, err
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
			return nil, nil, errors.Wrap(err, "worker create failed")
		}
	}

	wm.mu.Lock()
	wm.connections[session.WorkerId] = conn
	wm.mu.Unlock()

	return session, &protocol2.WorkerRegisterResponse{
		IdleTime: int32(prov.GetSetting().Autoscale.IdleTimeout.Milliseconds() / 1000),
	}, nil
}

func (wm *Manager) RemoveConnection(conn Connection) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	delete(wm.connections, conn.GetWorkerId())
}

func (wm *Manager) GetWorkersInfo() []*models2.Worker {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	result := make([]*models2.Worker, 0, len(wm.connections))
	for _, conn := range wm.connections {
		session := conn.GetSession()
		if session != nil {
			result = append(result, session.ToModel())
		}
	}
	return result
}

// WaitForWorkerReady waits for a worker with the specified ID to be ready and connected
func (wm *Manager) WaitForWorkerReady(ctx context.Context, workerId string) (worker.Connection, error) {
	// Check if worker is already connected and ready
	wm.mu.RLock()
	if conn, exists := wm.connections[workerId]; exists {
		wm.mu.RUnlock()
		return conn, nil
	}
	wm.mu.RUnlock()

	// Create a channel to receive worker ready notification
	readyChan := make(chan Connection, 1)

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
					readyChan <- conn
					return
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

func (wm *Manager) WaitTask(ctx context.Context, taskId string) (*models2.Task, error) {
	return wm.taskEventBus.Listen(ctx, taskId)
}

func (wm *Manager) workerIsIdle(conn Connection) bool {
	return conn.GetSession().CurrentTask == nil
}

// HandleWorkerMessage processes incoming messages from workers
func (wm *Manager) HandleWsWorkerMessage(conn Connection, wsMsg *protocol2.WebSocketMessage) error {
	switch wsMsg.Type {
	case protocol2.MessageType_MESSAGE_TYPE_HEARTBEAT:
		return wm.handleHeartbeat(conn, wsMsg.GetHeartbeat())
	case protocol2.MessageType_MESSAGE_TYPE_TASK_PROGRESS:
		return wm.handleTaskProgress(conn, wsMsg.GetTaskProgress())
	case protocol2.MessageType_MESSAGE_TYPE_TASK_COMPLETE:
		return wm.handleTaskComplete(conn, wsMsg.GetTaskComplete())
	case protocol2.MessageType_MESSAGE_TYPE_TASK_FAILED:
		return wm.handleTaskFailed(conn, wsMsg.GetTaskFailed())
	default:
		return fmt.Errorf("unknown message type: %v", wsMsg.Type)
	}
}

// handleHeartbeat processes heartbeat messages
func (wm *Manager) handleHeartbeat(conn Connection, heartbeat *protocol2.Heartbeat) error {
	if heartbeat == nil {
		return fmt.Errorf("heartbeat payload is nil")
	}

	rtt := conn.GetRtt()

	// Update worker session
	session := conn.GetSession()
	session.Health = types.ConvertWorkerHealthFromProto(heartbeat.Health)
	session.LastHeartbeat = models2.Now()
	session.Rtt = rtt.Microseconds()

	// Update resource usage if provided
	if heartbeat.ResourceUsage != nil {
		resourceMap := make(map[string]interface{})
		if existingInfo := conn.GetSession().ResourceInfo; existingInfo != nil {
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

		conn.GetSession().ResourceInfo = resourceMap
	}

	return wm.db.GetWorkerSessionRepository().UpdateHeartbeat(wm.ctx, session)
}

func (wm *Manager) GetTaskById(ctx context.Context, id string) (*models2.Task, error) {
	wm.mu.RLock()
	cachedTask := wm.processingTasks[id]
	wm.mu.RUnlock()
	if cachedTask != nil {
		return cachedTask, nil
	}
	return wm.db.GetTaskRepository().GetById(ctx, id)
}

func (wm *Manager) UpdateTask(ctx context.Context, task *models2.Task, remove bool) error {
	err := wm.db.GetTaskRepository().Update(ctx, task)
	wm.mu.Lock()
	if remove {
		delete(wm.processingTasks, task.Id)
	} else {
		wm.processingTasks[task.Id] = task
	}
	wm.mu.Unlock()
	return err
}

// processingTasks

// handleTaskProgress processes task progress updates
func (wm *Manager) handleTaskProgress(conn Connection, progress *protocol2.TaskProgress) error {
	if progress == nil {
		return fmt.Errorf("task progress payload is nil")
	}

	// Get task from database
	task, err := wm.GetTaskById(wm.ctx, progress.TaskId)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Verify worker owns this task
	if task.WorkerId != conn.GetWorkerId() {
		return fmt.Errorf("worker does not own this task (%s != %s)", task.WorkerId, conn.GetWorkerId())
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
func (wm *Manager) handleTaskComplete(conn Connection, complete *protocol2.TaskComplete) error {
	if complete == nil {
		return fmt.Errorf("task complete payload is nil")
	}

	// Get task from database
	task, err := wm.GetTaskById(wm.ctx, complete.TaskId)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Verify worker owns this task
	if task.WorkerId != conn.GetWorkerId() {
		return fmt.Errorf("worker does not own this task (%s != %s)", task.WorkerId, conn.GetWorkerId())
	}

	// Update worker session
	conn.GetSession().ClearTask()

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

	log.Printf("Task %s completed by worker %s", complete.TaskId, conn.GetWorkerId())

	return nil
}

// handleTaskFailed processes task failure
func (wm *Manager) handleTaskFailed(conn Connection, failed *protocol2.TaskFailed) error {
	if failed == nil {
		return fmt.Errorf("task failed payload is nil")
	}

	// Get task from database
	task, err := wm.GetTaskById(wm.ctx, failed.TaskId)
	if err != nil {
		return fmt.Errorf("task not found: %w", err)
	}

	// Verify worker owns this task
	if task.WorkerId != conn.GetWorkerId() {
		return fmt.Errorf("worker does not own this task (%s != %s)", task.WorkerId, conn.GetWorkerId())
	}

	task.Error = &models2.TaskError{
		Code:    types.TaskErrorCode(failed.ErrorCode),
		Message: failed.ErrorMessage,
	}
	if failed.ErrorDetails != nil {
		task.Error.Details = failed.ErrorDetails.AsMap()
	}

	// Update worker session
	conn.GetSession().ClearTask()

	wm.taskComplete(task)

	log.Printf("Task %s failed on worker %s: %s", failed.TaskId, conn.GetWorkerId(), failed.ErrorMessage)
	return nil
}

func (wm *Manager) findGrowableProvisionerByQueue(queue string, taskDelay time.Duration) []provisioner.Provisioner {
	var plist []provisioner.Provisioner
	for _, prov := range wm.provisionerManager.GetProvisioners() {
		if slices.Contains(prov.GetSetting().Queues, queue) && prov.IsGrowable(taskDelay) {
			plist = append(plist, prov)
		}
	}
	return plist
}

// workerMatchesQueue checks if worker's queue patterns match the task queue
func (wm *Manager) workerMatchesQueue(session *models2.WorkerSession, queue string) bool {
	for _, name := range session.Queues {
		if name == queue {
			return true
		}
	}
	return false
}

// sendTaskAssignment sends a task assignment to a worker
func (wm *Manager) sendTaskAssignment(conn Connection, task *models2.Task) error {
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

	// Update task and worker session
	task.Status = types.TaskStatusProcessing
	task.WorkerId = conn.GetWorkerId()
	task.StartedAt = models2.NowPtr()

	// Update database
	if err := wm.UpdateTask(wm.ctx, task, false); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	if err := conn.SendPayload(wsMsg); err != nil {
		task.Status = types.TaskStatusPending
		task.WorkerId = ""
		_ = wm.UpdateTask(wm.ctx, task, true)
		return err
	}

	log.Printf("Task %s assigned to worker %s", task.Id, conn.GetWorkerId())
	return nil
}

func getTaskCompleteStatus(task *models2.Task) types.TaskStatus {
	if task.WebhookUrl != "" {
		return types.TaskStatusCompleted
	} else {
		return types.TaskStatusFinished
	}
}

func (wm *Manager) taskComplete(task *models2.Task) {
	taskRepository := wm.db.GetTaskRepository()

	task.CompletedAt = models2.NowPtr()

	if task.Error != nil {
		if task.Retried >= task.MaxRetry {
			task.Status = getTaskCompleteStatus(task)
		} else {
			task.Status = types.TaskStatusPending
			task.Retried++
		}
	} else {
		task.Status = getTaskCompleteStatus(task)
	}

	_ = taskRepository.Update(wm.ctx, task)

	wm.taskEventBus.Publish(task.Id, task)

	if task.Status == types.TaskStatusPending && wm.queueConsuming {
		wm.EnqueueTask(task)
	} else if task.Status != types.TaskStatusFinished {
		wm.asyncTaskFinish(task)
	}
}

func (wm *Manager) taskFinish(task *models2.Task) error {
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

func (wm *Manager) asyncTaskFinish(task *models2.Task) {
	go func() {
		if err := wm.taskFinish(task); err != nil {
			wm.logger.Warnf("TASK[%s] task finish failed: %+v", task.Id, err)
			wm.EnqueueTask(task)
		}
	}()
}

// assignPendingTasks tries to assign pending tasks to available workers
func (wm *Manager) assignPendingTasks() {
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
		wm.EnqueueTask(task)
	}
}

// healthCheckWorker periodically checks worker connection health
func (wm *Manager) healthCheckWorker() {
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
func (wm *Manager) checkWorkerHealth() {
	wm.mu.RLock()
	connections := make([]Connection, 0, len(wm.connections))
	for _, conn := range wm.connections {
		connections = append(connections, conn)
	}
	wm.mu.RUnlock()

	for _, conn := range connections {
		lastPong := conn.GetLastPong()

		// If no pong received in 90 seconds, consider connection dead
		if time.Since(lastPong) > 90*time.Second {
			log.Printf("Worker[%s] appears to be dead, closing connection", conn.GetWorkerId())
			conn.Close()
		}
	}
}

func (wm *Manager) cleanupGarbage() {
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
func (wm *Manager) Shutdown() {
	wm.cancel()

	wm.mu.RLock()
	for _, conn := range wm.connections {
		_ = conn.Close()
	}
	wm.mu.RUnlock()

	// TODO: wait for gracefully shutdown
}
