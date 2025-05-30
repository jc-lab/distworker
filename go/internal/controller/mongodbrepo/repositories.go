package mongodbrepo

import (
	"context"
	"errors"
	"fmt"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// TaskRepository handles task database operations for MongoDB
type TaskRepository struct {
	db *MongoDB
}

var _ database.TaskRepositoryInterface = (*TaskRepository)(nil)

// NewTaskRepository creates a new task repository
func NewTaskRepository(db *MongoDB) *TaskRepository {
	return &TaskRepository{db: db}
}

// Create creates a new task
func (r *TaskRepository) Create(ctx context.Context, task *models2.Task) error {
	_, err := r.db.TasksCollection().InsertOne(ctx, task)
	return err
}

// GetById retrieves a task by ID
func (r *TaskRepository) GetById(ctx context.Context, taskId string) (*models2.Task, error) {
	var task models2.Task
	err := r.db.TasksCollection().FindOne(ctx, bson.M{"_id": taskId}).Decode(&task)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("task not found")
		}
		return nil, err
	}
	return &task, nil
}

// Update updates a task
func (r *TaskRepository) Update(ctx context.Context, task *models2.Task) error {
	_, err := r.db.TasksCollection().ReplaceOne(ctx, bson.M{"_id": task.Id}, task)
	return err
}

// Delete deletes a task
func (r *TaskRepository) Delete(ctx context.Context, taskId string) error {
	_, err := r.db.TasksCollection().DeleteOne(ctx, bson.M{"_id": taskId})
	return err
}

// List retrieves tasks with pagination and filters
func (r *TaskRepository) List(ctx context.Context, filter database.TaskFilter, page, limit int) ([]*models2.Task, int64, error) {
	// Build filter
	mongoFilter := bson.M{}
	if filter.Queue != "" {
		mongoFilter["queue"] = filter.Queue
	}
	if filter.Status != "" {
		mongoFilter["status"] = filter.Status
	}
	if filter.WorkerId != "" {
		mongoFilter["worker_id"] = filter.WorkerId
	}

	// Count total
	total, err := r.db.TasksCollection().CountDocuments(ctx, mongoFilter)
	if err != nil {
		return nil, 0, err
	}

	// Find with pagination
	skip := (page - 1) * limit
	findOptions := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(limit)).
		SetSort(bson.D{{"created_at", -1}})

	cursor, err := r.db.TasksCollection().Find(ctx, mongoFilter, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var tasks []*models2.Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, 0, err
	}

	return tasks, total, nil
}

// GetPendingTasks retrieves pending tasks for a queue
func (r *TaskRepository) GetPendingTasks(ctx context.Context, queue string, limit int) ([]*models2.Task, error) {
	filter := bson.M{
		"status": types.TaskStatusPending,
		"queue":  queue,
	}

	findOptions := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{"created_at", 1}})

	cursor, err := r.db.TasksCollection().Find(ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []*models2.Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}

// QueueRepository handles queue database operations for MongoDB
type QueueRepository struct {
	db *MongoDB
}

var _ database.QueueRepositoryInterface = (*QueueRepository)(nil)

// NewQueueRepository creates a new queue repository
func NewQueueRepository(db *MongoDB) *QueueRepository {
	return &QueueRepository{db: db}
}

// Create creates a new queue
func (r *QueueRepository) Create(ctx context.Context, queue *models2.Queue) error {
	_, err := r.db.QueuesCollection().InsertOne(ctx, queue)
	return err
}

// GetByName retrieves a queue by name
func (r *QueueRepository) GetByName(ctx context.Context, name string) (*models2.Queue, error) {
	var queue models2.Queue
	err := r.db.QueuesCollection().FindOne(ctx, bson.M{"_id": name}).Decode(&queue)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("queue not found")
		}
		return nil, err
	}
	return &queue, nil
}

// Update updates a queue
func (r *QueueRepository) Update(ctx context.Context, queue *models2.Queue) error {
	queue.UpdatedAt = models2.Now()
	_, err := r.db.QueuesCollection().ReplaceOne(ctx, bson.M{"_id": queue.Name}, queue)
	return err
}

// Delete deletes a queue
func (r *QueueRepository) Delete(ctx context.Context, name string) error {
	_, err := r.db.QueuesCollection().DeleteOne(ctx, bson.M{"_id": name})
	return err
}

// List retrieves all queues
func (r *QueueRepository) List(ctx context.Context) ([]*models2.Queue, error) {
	cursor, err := r.db.QueuesCollection().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var queues []*models2.Queue
	if err := cursor.All(ctx, &queues); err != nil {
		return nil, err
	}

	return queues, nil
}

// WorkerSessionRepository handles worker session database operations for MongoDB
type WorkerSessionRepository struct {
	db *MongoDB
}

var _ database.WorkerSessionRepositoryInterface = (*WorkerSessionRepository)(nil)

// NewWorkerSessionRepository creates a new worker session repository
func NewWorkerSessionRepository(db *MongoDB) *WorkerSessionRepository {
	return &WorkerSessionRepository{db: db}
}

// Create creates a new worker session
func (r *WorkerSessionRepository) Create(ctx context.Context, session *models2.WorkerSession) error {
	_, err := r.db.WorkerSessionsCollection().InsertOne(ctx, session)
	return err
}

// GetById retrieves a worker session by ID
func (r *WorkerSessionRepository) GetById(ctx context.Context, workerId string) (*models2.WorkerSession, error) {
	var session models2.WorkerSession
	err := r.db.WorkerSessionsCollection().FindOne(ctx, bson.M{"_id": workerId}).Decode(&session)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, fmt.Errorf("worker session not found")
		}
		return nil, err
	}
	return &session, nil
}

// Update updates a worker session
func (r *WorkerSessionRepository) Update(ctx context.Context, session *models2.WorkerSession) error {
	_, err := r.db.WorkerSessionsCollection().ReplaceOne(ctx, bson.M{"_id": session.WorkerId}, session)
	return err
}

// Delete deletes a worker session
func (r *WorkerSessionRepository) Delete(ctx context.Context, workerId string) error {
	_, err := r.db.WorkerSessionsCollection().DeleteOne(ctx, bson.M{"_id": workerId})
	return err
}

// List retrieves all active worker sessions
func (r *WorkerSessionRepository) List(ctx context.Context) ([]*models2.WorkerSession, error) {
	cursor, err := r.db.WorkerSessionsCollection().Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var sessions []*models2.WorkerSession
	if err := cursor.All(ctx, &sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

// UpdateHeartbeat updates the last heartbeat time for a worker
func (r *WorkerSessionRepository) UpdateHeartbeat(ctx context.Context, workerId string, status types.WorkerStatus) error {
	update := bson.M{
		"$set": bson.M{
			"last_heartbeat": models2.Now(),
			"status":         status,
		},
	}
	_, err := r.db.WorkerSessionsCollection().UpdateOne(ctx, bson.M{"_id": workerId}, update)
	return err
}
