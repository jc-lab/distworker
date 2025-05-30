package database_mock

import (
	"context"
	"errors"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"strings"
	"sync"
)

// MockDatabase implements the Database interface for testing
type MockDatabase struct {
	mu               sync.RWMutex
	tasks            map[string]*models2.Task
	queues           map[string]*models2.Queue
	workerSessions   map[string]*models2.WorkerSession
	healthError      error
	closed           bool
	taskRepository   *MockTaskRepository
	queueRepository  *MockQueueRepository
	workerRepository *MockWorkerSessionRepository
}

// NewMockDatabase creates a new mock database instance
func NewMockDatabase() *MockDatabase {
	db := &MockDatabase{
		tasks:          make(map[string]*models2.Task),
		queues:         make(map[string]*models2.Queue),
		workerSessions: make(map[string]*models2.WorkerSession),
	}

	db.taskRepository = &MockTaskRepository{db: db}
	db.queueRepository = &MockQueueRepository{db: db}
	db.workerRepository = &MockWorkerSessionRepository{db: db}

	return db
}

// SetHealthError sets an error to be returned by Health()
func (m *MockDatabase) SetHealthError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthError = err
}

// Close implements Database interface
func (m *MockDatabase) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// Health implements Database interface
func (m *MockDatabase) Health(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return errors.New("database is closed")
	}

	return m.healthError
}

// GetTaskRepository implements Database interface
func (m *MockDatabase) GetTaskRepository() database.TaskRepositoryInterface {
	return m.taskRepository
}

// GetQueueRepository implements Database interface
func (m *MockDatabase) GetQueueRepository() database.QueueRepositoryInterface {
	return m.queueRepository
}

// GetWorkerSessionRepository implements Database interface
func (m *MockDatabase) GetWorkerSessionRepository() database.WorkerSessionRepositoryInterface {
	return m.workerRepository
}

// MockTaskRepository implements TaskRepositoryInterface for testing
type MockTaskRepository struct {
	db *MockDatabase
}

// Create implements TaskRepositoryInterface
func (r *MockTaskRepository) Create(ctx context.Context, task *models2.Task) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	if task.Id == "" {
		return errors.New("task ID is required")
	}

	if _, exists := r.db.tasks[task.Id]; exists {
		return errors.New("task already exists")
	}

	// Create a copy to avoid pointer issues
	taskCopy := *task
	if taskCopy.CreatedAt.IsZero() {
		taskCopy.CreatedAt = models2.Now()
	}

	r.db.tasks[task.Id] = &taskCopy
	return nil
}

// GetByID implements TaskRepositoryInterface
func (r *MockTaskRepository) GetById(ctx context.Context, taskId string) (*models2.Task, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	if r.db.closed {
		return nil, errors.New("database is closed")
	}

	task, exists := r.db.tasks[taskId]
	if !exists {
		return nil, errors.New("task not found")
	}

	// Return a copy to avoid pointer issues
	taskCopy := *task
	return &taskCopy, nil
}

// Update implements TaskRepositoryInterface
func (r *MockTaskRepository) Update(ctx context.Context, task *models2.Task) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	if task.Id == "" {
		return errors.New("task ID is required")
	}

	if _, exists := r.db.tasks[task.Id]; !exists {
		return errors.New("task not found")
	}

	// Create a copy
	taskCopy := *task
	r.db.tasks[task.Id] = &taskCopy
	return nil
}

// Delete implements TaskRepositoryInterface
func (r *MockTaskRepository) Delete(ctx context.Context, taskId string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	if _, exists := r.db.tasks[taskId]; !exists {
		return errors.New("task not found")
	}

	delete(r.db.tasks, taskId)
	return nil
}

// List implements TaskRepositoryInterface
func (r *MockTaskRepository) List(ctx context.Context, filter database.TaskFilter, page, limit int) ([]*models2.Task, int64, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	if r.db.closed {
		return nil, 0, errors.New("database is closed")
	}

	var filteredTasks []*models2.Task

	for _, task := range r.db.tasks {
		if r.matchesFilter(task, filter) {
			taskCopy := *task
			filteredTasks = append(filteredTasks, &taskCopy)
		}
	}

	total := int64(len(filteredTasks))

	// Apply pagination
	start := (page - 1) * limit
	end := start + limit

	if start >= len(filteredTasks) {
		return []*models2.Task{}, total, nil
	}

	if end > len(filteredTasks) {
		end = len(filteredTasks)
	}

	return filteredTasks[start:end], total, nil
}

// GetPendingTasks implements TaskRepositoryInterface
func (r *MockTaskRepository) GetPendingTasks(ctx context.Context, queue string, limit int) ([]*models2.Task, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	if r.db.closed {
		return nil, errors.New("database is closed")
	}

	var pendingTasks []*models2.Task
	count := 0

	for _, task := range r.db.tasks {
		if task.Status == types.TaskStatusPending &&
			(queue == "" || task.Queue == queue) &&
			count < limit {
			taskCopy := *task
			pendingTasks = append(pendingTasks, &taskCopy)
			count++
		}
	}

	return pendingTasks, nil
}

func (r *MockTaskRepository) matchesFilter(task *models2.Task, filter database.TaskFilter) bool {
	if filter.Queue != "" && task.Queue != filter.Queue {
		return false
	}

	if filter.Status != "" && string(task.Status) != filter.Status {
		return false
	}

	if filter.WorkerId != "" && task.WorkerId != filter.WorkerId {
		return false
	}

	return true
}

// MockQueueRepository implements QueueRepositoryInterface for testing
type MockQueueRepository struct {
	db *MockDatabase
}

// Create implements QueueRepositoryInterface
func (r *MockQueueRepository) Create(ctx context.Context, queue *models2.Queue) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	if queue.Name == "" {
		return errors.New("queue name is required")
	}

	if _, exists := r.db.queues[queue.Name]; exists {
		return errors.New("queue already exists")
	}

	queueCopy := *queue
	if queueCopy.CreatedAt.IsZero() {
		queueCopy.CreatedAt = models2.Now()
	}
	if queueCopy.UpdatedAt.IsZero() {
		queueCopy.UpdatedAt = queueCopy.CreatedAt
	}

	r.db.queues[queue.Name] = &queueCopy
	return nil
}

// GetByName implements QueueRepositoryInterface
func (r *MockQueueRepository) GetByName(ctx context.Context, name string) (*models2.Queue, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	if r.db.closed {
		return nil, errors.New("database is closed")
	}

	queue, exists := r.db.queues[name]
	if !exists {
		return nil, errors.New("queue not found")
	}

	queueCopy := *queue
	return &queueCopy, nil
}

// Update implements QueueRepositoryInterface
func (r *MockQueueRepository) Update(ctx context.Context, queue *models2.Queue) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	if queue.Name == "" {
		return errors.New("queue name is required")
	}

	if _, exists := r.db.queues[queue.Name]; !exists {
		return errors.New("queue not found")
	}

	queueCopy := *queue
	queueCopy.UpdatedAt = models2.Now()
	r.db.queues[queue.Name] = &queueCopy
	return nil
}

// Delete implements QueueRepositoryInterface
func (r *MockQueueRepository) Delete(ctx context.Context, name string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	if _, exists := r.db.queues[name]; !exists {
		return errors.New("queue not found")
	}

	delete(r.db.queues, name)
	return nil
}

// List implements QueueRepositoryInterface
func (r *MockQueueRepository) List(ctx context.Context) ([]*models2.Queue, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	if r.db.closed {
		return nil, errors.New("database is closed")
	}

	var queues []*models2.Queue
	for _, queue := range r.db.queues {
		queueCopy := *queue
		queues = append(queues, &queueCopy)
	}

	return queues, nil
}

// MockWorkerSessionRepository implements WorkerSessionRepositoryInterface for testing
type MockWorkerSessionRepository struct {
	db *MockDatabase
}

// Create implements WorkerSessionRepositoryInterface
func (r *MockWorkerSessionRepository) Create(ctx context.Context, session *models2.WorkerSession) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	if session.WorkerId == "" {
		return errors.New("worker ID is required")
	}

	if _, exists := r.db.workerSessions[session.WorkerId]; exists {
		return errors.New("worker session already exists")
	}

	sessionCopy := *session
	if sessionCopy.ConnectedAt.IsZero() {
		sessionCopy.ConnectedAt = models2.Now()
	}
	if sessionCopy.LastHeartbeat.IsZero() {
		sessionCopy.LastHeartbeat = sessionCopy.ConnectedAt
	}

	r.db.workerSessions[session.WorkerId] = &sessionCopy
	return nil
}

// GetById implements WorkerSessionRepositoryInterface
func (r *MockWorkerSessionRepository) GetById(ctx context.Context, workerId string) (*models2.WorkerSession, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	if r.db.closed {
		return nil, errors.New("database is closed")
	}

	session, exists := r.db.workerSessions[workerId]
	if !exists {
		return nil, errors.New("worker session not found")
	}

	sessionCopy := *session
	return &sessionCopy, nil
}

// Update implements WorkerSessionRepositoryInterface
func (r *MockWorkerSessionRepository) Update(ctx context.Context, session *models2.WorkerSession) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	if session.WorkerId == "" {
		return errors.New("worker ID is required")
	}

	if _, exists := r.db.workerSessions[session.WorkerId]; !exists {
		return errors.New("worker session not found")
	}

	sessionCopy := *session
	r.db.workerSessions[session.WorkerId] = &sessionCopy
	return nil
}

// Delete implements WorkerSessionRepositoryInterface
func (r *MockWorkerSessionRepository) Delete(ctx context.Context, workerId string) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	if _, exists := r.db.workerSessions[workerId]; !exists {
		return errors.New("worker session not found")
	}

	delete(r.db.workerSessions, workerId)
	return nil
}

// List implements WorkerSessionRepositoryInterface
func (r *MockWorkerSessionRepository) List(ctx context.Context) ([]*models2.WorkerSession, error) {
	r.db.mu.RLock()
	defer r.db.mu.RUnlock()

	if r.db.closed {
		return nil, errors.New("database is closed")
	}

	var sessions []*models2.WorkerSession
	for _, session := range r.db.workerSessions {
		sessionCopy := *session
		sessions = append(sessions, &sessionCopy)
	}

	return sessions, nil
}

// UpdateHeartbeat implements WorkerSessionRepositoryInterface
func (r *MockWorkerSessionRepository) UpdateHeartbeat(ctx context.Context, workerId string, status types.WorkerStatus) error {
	r.db.mu.Lock()
	defer r.db.mu.Unlock()

	if r.db.closed {
		return errors.New("database is closed")
	}

	session, exists := r.db.workerSessions[workerId]
	if !exists {
		return errors.New("worker session not found")
	}

	session.Status = status
	session.LastHeartbeat = models2.Now()
	return nil
}

// Helper methods for testing

// GetAllTasks returns all tasks in the mock database
func (m *MockDatabase) GetAllTasks() map[string]*models2.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*models2.Task)
	for id, task := range m.tasks {
		taskCopy := *task
		result[id] = &taskCopy
	}
	return result
}

// GetAllQueues returns all queues in the mock database
func (m *MockDatabase) GetAllQueues() map[string]*models2.Queue {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*models2.Queue)
	for name, queue := range m.queues {
		queueCopy := *queue
		result[name] = &queueCopy
	}
	return result
}

// GetAllWorkerSessions returns all worker sessions in the mock database
func (m *MockDatabase) GetAllWorkerSessions() map[string]*models2.WorkerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*models2.WorkerSession)
	for id, session := range m.workerSessions {
		sessionCopy := *session
		result[id] = &sessionCopy
	}
	return result
}

// Clear removes all data from the mock database
func (m *MockDatabase) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.tasks = make(map[string]*models2.Task)
	m.queues = make(map[string]*models2.Queue)
	m.workerSessions = make(map[string]*models2.WorkerSession)
}

// TaskExists checks if a task exists in the mock database
func (m *MockDatabase) TaskExists(taskId string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.tasks[taskId]
	return exists
}

// QueueExists checks if a queue exists in the mock database
func (m *MockDatabase) QueueExists(name string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.queues[name]
	return exists
}

// WorkerSessionExists checks if a worker session exists in the mock database
func (m *MockDatabase) WorkerSessionExists(workerId string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.workerSessions[workerId]
	return exists
}

// SimulateError makes the next operation return an error
func (m *MockDatabase) SimulateError(operation string) {
	// This could be enhanced to simulate specific errors for specific operations
	m.SetHealthError(errors.New("simulated " + operation + " error"))
}

// GetTasksByStatus returns all tasks with the given status
func (m *MockDatabase) GetTasksByStatus(status types.TaskStatus) []*models2.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models2.Task
	for _, task := range m.tasks {
		if task.Status == status {
			taskCopy := *task
			result = append(result, &taskCopy)
		}
	}
	return result
}

// GetTasksByQueue returns all tasks in the given queue
func (m *MockDatabase) GetTasksByQueue(queue string) []*models2.Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models2.Task
	for _, task := range m.tasks {
		if task.Queue == queue {
			taskCopy := *task
			result = append(result, &taskCopy)
		}
	}
	return result
}

// GetWorkerSessionsByStatus returns all worker sessions with the given status
func (m *MockDatabase) GetWorkerSessionsByStatus(status types.WorkerStatus) []*models2.WorkerSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models2.WorkerSession
	for _, session := range m.workerSessions {
		if session.Status == status {
			sessionCopy := *session
			result = append(result, &sessionCopy)
		}
	}
	return result
}

// CountTasks returns the number of tasks in the database
func (m *MockDatabase) CountTasks() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.tasks)
}

// CountQueues returns the number of queues in the database
func (m *MockDatabase) CountQueues() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.queues)
}

// CountWorkerSessions returns the number of worker sessions in the database
func (m *MockDatabase) CountWorkerSessions() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.workerSessions)
}

// MatchQueuePattern checks if a queue name matches a pattern (supports * and #)
func MatchQueuePattern(pattern, queue string) bool {
	// Convert pattern to regex-like matching
	// * matches a single segment
	// # matches any number of segments

	if pattern == "#" {
		return true // # matches everything
	}

	patternParts := strings.Split(pattern, "/")
	queueParts := strings.Split(queue, "/")

	return matchParts(patternParts, queueParts, 0, 0)
}

func matchParts(pattern []string, queue []string, pi, qi int) bool {
	if pi >= len(pattern) && qi >= len(queue) {
		return true // Both exhausted
	}

	if pi >= len(pattern) {
		return false // Pattern exhausted but queue has more
	}

	if pattern[pi] == "#" {
		// # can match any number of segments
		if pi == len(pattern)-1 {
			return true // # at end matches everything remaining
		}

		// Try matching # with 0, 1, 2, ... segments
		for skip := 0; skip <= len(queue)-qi; skip++ {
			if matchParts(pattern, queue, pi+1, qi+skip) {
				return true
			}
		}
		return false
	}

	if qi >= len(queue) {
		return false // Queue exhausted but pattern has more
	}

	if pattern[pi] == "*" || pattern[pi] == queue[qi] {
		return matchParts(pattern, queue, pi+1, qi+1)
	}

	return false
}
