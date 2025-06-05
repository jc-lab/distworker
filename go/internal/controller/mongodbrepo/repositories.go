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
	if filter.StatusGte < filter.StatusLt {
		mongoFilter["status"] = bson.M{
			"$gte": filter.StatusGte,
			"$lt":  filter.StatusLt,
		}
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

func (r *TaskRepository) ListAll(ctx context.Context, filter database.TaskFilter) (chan *models2.Task, error) {
	findOptions := options.Find().SetSort(bson.D{{"_id", 1}})

	mongoFilter := bson.M{}
	if filter.Queue != "" {
		mongoFilter["queue"] = filter.Queue
	}
	if filter.StatusGte < filter.StatusLt {
		mongoFilter["status"] = bson.M{
			"$gte": filter.StatusGte,
			"$lt":  filter.StatusLt,
		}
	}
	if filter.WorkerId != "" {
		mongoFilter["workerId"] = filter.WorkerId
	}

	cursor, err := r.db.TasksCollection().Find(ctx, mongoFilter, findOptions)
	if err != nil {
		return nil, err
	}

	taskChan := make(chan *models2.Task)
	go func() {
		defer close(taskChan)
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			task := &models2.Task{}
			if err := cursor.Decode(task); err != nil {
				break
			}
			taskChan <- task
		}
	}()

	return taskChan, nil
}

func (r *TaskRepository) Stat(ctx context.Context) (*database.TaskCollectionStat, error) {
	cursor, err := r.db.TasksCollection().Aggregate(ctx, mongo.Pipeline{
		{{"$group",
			bson.D{{
				"_id",
				bson.D{
					{"queue", "$queue"},
					{"status", "$status"},
				}},
				{"count", bson.D{{"$sum", "1"}}},
				{"error", bson.D{{"$sum", bson.D{
					{"$cond", bson.A{
						bson.D{{"$ne", bson.A{"$error", nil}}}, // error 필드가 null이 아닌 경우
						1,                                      // true일 때 1을 더함
						0,                                      // false일 때 0을 더함
					}},
				}}}},
			},
		}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []TaskCollectionStat
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}

	result := &database.TaskCollectionStat{
		Queues: make(map[string]*database.TaskCollectionStatCounter),
	}
	for _, row := range rows {
		queueStat, ok := result.Queues[row.Id.Queue]
		if !ok {
			queueStat = &database.TaskCollectionStatCounter{}
			result.Queues[row.Id.Queue] = queueStat
		}
		switch row.Id.Status {
		case types.TaskStatusPending:
			queueStat.Pending = row.Count
		case types.TaskStatusProcessing:
			queueStat.Pending = row.Count
		case types.TaskStatusCompleted:
			queueStat.Completed = row.Count
			queueStat.Error += row.Error
		case types.TaskStatusFinished:
			queueStat.Finished = row.Count
			queueStat.Error += row.Error
		}
	}

	return result, nil
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
func (r *WorkerSessionRepository) UpdateHeartbeat(ctx context.Context, workerId string, health types.WorkerHealth) error {
	update := bson.M{
		"$set": bson.M{
			"last_heartbeat": models2.Now(),
			"health":         health,
		},
	}
	_, err := r.db.WorkerSessionsCollection().UpdateOne(ctx, bson.M{"_id": workerId}, update)
	return err
}
