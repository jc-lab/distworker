package websocket

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	protocol2 "github.com/jc-lab/distworker/go/internal/protocol"
	"github.com/jc-lab/distworker/go/internal/provisioner"
	"github.com/jc-lab/distworker/go/pkg/controller/config"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"github.com/pkg/errors"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

// WorkerConnection represents a single worker WebSocket connection
type WorkerConnection struct {
	WorkerId    string
	Conn        *websocket.Conn
	Session     *models2.WorkerSession
	LastPing    time.Time
	LastPong    time.Time
	SendChannel chan []byte
	Done        chan struct{}
	mu          sync.RWMutex
}

// WorkerManager manages all worker WebSocket connections
type WorkerManager struct {
	db                 database.Database
	provisionerManager *provisioner.Manager

	connections    map[string]*WorkerConnection
	taskAssignChan chan *models2.Task
	mu             sync.RWMutex
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewWorkerManager creates a new worker connection manager
func NewWorkerManager(db database.Database, provisionerManager *provisioner.Manager) *WorkerManager {
	ctx, cancel := context.WithCancel(context.Background())

	wm := &WorkerManager{
		db:                 db,
		provisionerManager: provisionerManager,

		connections:    make(map[string]*WorkerConnection),
		taskAssignChan: make(chan *models2.Task, 100),
		ctx:            ctx,
		cancel:         cancel,
	}

	// Start task assignment goroutine
	go wm.taskAssignmentWorker()

	// Start connection health checker
	go wm.healthCheckWorker()

	return wm
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

	var session *models2.WorkerSession
	if signedRegister.WorkerId != "" {
		fullWorkerId := signedRegister.GetProvisionerName() + "-" + signedRegister.GetWorkerId()
		session, err = wm.db.GetWorkerSessionRepository().GetById(wm.ctx, fullWorkerId)
		if err != nil {
			return fmt.Errorf("worker not found: %+v", err)
		}
		if err = protocol2.ValidateWebSocketSignature(session.WorkerToken, signedRegister.GetDate(), register.Data, register.Signature); err != nil {
			return err
		}

		// Update session status
		session.Status = types.WorkerStatusIdle
		session.LastHeartbeat = models2.Now()
		session.ConnectedAt = models2.Now()

		if err := wm.db.GetWorkerSessionRepository().Update(wm.ctx, session); err != nil {
			return fmt.Errorf("failed to update worker session: %w", err)
		}
	} else {
		prov := wm.provisionerManager.GetProvisioner(signedRegister.GetProvisionerName())
		if prov == nil {
			return fmt.Errorf("worker provisioner(\"%s\") not found", signedRegister.GetProvisionerName())
		}
		if prov.Type != config.ProvisionerTypeNone {
			return fmt.Errorf("no worker id")
		}
		if err = protocol2.ValidateWebSocketSignature(prov.Token, signedRegister.GetDate(), register.Data, register.Signature); err != nil {
			return err
		}

		session = &models2.WorkerSession{
			WorkerId:        prov.Name + "-" + uuid.NewString(),
			ProvisionerName: prov.Name,
			ConnectedAt:     now,
			Status:          types.WorkerStatusIdle,
			ResourceInfo:    types.ResourceInfoFromProto(signedRegister.ResourceInfo),
			WorkerToken:     prov.Token,
			Queues:          prov.Queues,
			LastHeartbeat:   now,
		}

		if err = wm.db.GetWorkerSessionRepository().Create(wm.ctx, session); err != nil {
			return errors.Wrap(err, "worker create failed")
		}
	}

	// Update worker connection and session
	workerConn.WorkerId = session.WorkerId
	workerConn.Session = session

	log.Printf("Worker %s registered successfully", workerConn.WorkerId)
	return nil
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
	workerConn.Session.Status = types.ConvertWorkerStatusFromProto(heartbeat.Status)
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

	return wm.db.GetWorkerSessionRepository().UpdateHeartbeat(wm.ctx, workerConn.WorkerId, workerConn.Session.Status)
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

	// Update task status
	task.Status = types.TaskStatusCompleted
	task.CompletedAt = models2.NowPtr()

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

	// Update worker session
	workerConn.Session.Status = types.WorkerStatusIdle
	workerConn.Session.CurrentTask = ""

	// Update both task and worker session
	if err := wm.db.GetTaskRepository().Update(wm.ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	if err := wm.db.GetWorkerSessionRepository().Update(wm.ctx, workerConn.Session); err != nil {
		return fmt.Errorf("failed to update worker session: %w", err)
	}

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

	// Update task status
	task.Status = types.TaskStatusFailed
	task.CompletedAt = models2.NowPtr()

	// Set error information
	task.Error = &models2.TaskError{
		Code:    types.TaskErrorCode(failed.ErrorCode),
		Message: failed.ErrorMessage,
	}

	if failed.ErrorDetails != nil {
		task.Error.Details = failed.ErrorDetails.AsMap()
	}

	// Update worker session
	workerConn.Session.Status = types.WorkerStatusIdle
	workerConn.Session.CurrentTask = ""

	// Update both task and worker session
	if err := wm.db.GetTaskRepository().Update(wm.ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	if err := wm.db.GetWorkerSessionRepository().Update(wm.ctx, workerConn.Session); err != nil {
		return fmt.Errorf("failed to update worker session: %w", err)
	}

	log.Printf("Task %s failed on worker %s: %s", failed.TaskId, workerConn.WorkerId, failed.ErrorMessage)
	return nil
}

// AssignTask assigns a task to an available worker
func (wm *WorkerManager) AssignTask(task *models2.Task) error {
	wm.mu.RLock()
	defer wm.mu.RUnlock()

	// Find available worker that matches queue patterns
	for _, conn := range wm.connections {
		if conn.Session.Status == types.WorkerStatusIdle {
			if wm.workerMatchesQueue(conn.Session, task.Queue) {
				return wm.sendTaskAssignment(conn, task)
			}
		}
	}

	// No available worker found
	return fmt.Errorf("no available worker found for queue: %s", task.Queue)
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

	// Marshal message
	data, err := proto.Marshal(wsMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal task assignment: %w", err)
	}

	// Send to worker
	select {
	case conn.SendChannel <- data:
		// Update task and worker session
		task.Status = types.TaskStatusProcessing
		task.WorkerId = conn.WorkerId
		task.StartedAt = models2.NowPtr()

		conn.Session.Status = types.WorkerStatusProcessing
		conn.Session.CurrentTask = task.Id

		// Update database
		if err := wm.db.GetTaskRepository().Update(wm.ctx, task); err != nil {
			return fmt.Errorf("failed to update task: %w", err)
		}

		if err := wm.db.GetWorkerSessionRepository().Update(wm.ctx, conn.Session); err != nil {
			return fmt.Errorf("failed to update worker session: %w", err)
		}

		log.Printf("Task %s assigned to worker %s", task.Id, conn.WorkerId)
		return nil
	default:
		return fmt.Errorf("worker send channel is full")
	}
}

// taskAssignmentWorker continuously tries to assign pending tasks
func (wm *WorkerManager) taskAssignmentWorker() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wm.ctx.Done():
			return
		case <-ticker.C:
			wm.assignPendingTasks()
		case task := <-wm.taskAssignChan:
			wm.AssignTask(task)
		}
	}
}

// assignPendingTasks tries to assign pending tasks to available workers
func (wm *WorkerManager) assignPendingTasks() {
	// Get pending tasks
	tasks, err := wm.db.GetTaskRepository().GetPendingTasks(wm.ctx, "", 10)
	if err != nil {
		log.Printf("Failed to get pending tasks: %v", err)
		return
	}

	for _, task := range tasks {
		if err := wm.AssignTask(task); err != nil {
			// Log but continue trying other tasks
			log.Printf("Failed to assign task %s: %v", task.Id, err)
		}
	}
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

// Shutdown gracefully shuts down the worker manager
func (wm *WorkerManager) Shutdown() {
	wm.cancel()

	wm.mu.RLock()
	for _, conn := range wm.connections {
		conn.Conn.Close()
	}
	wm.mu.RUnlock()
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
