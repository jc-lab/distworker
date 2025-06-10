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
	database2 "github.com/jc-lab/distworker/go/pkg/controller/database"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoDB represents the MongoDB database connection
type MongoDB struct {
	client            *mongo.Client
	database          *mongo.Database
	taskRepo          *TaskRepository
	queueRepo         *QueueRepository
	workerSessionRepo *WorkerSessionRepository
}

type IndexDefine struct {
	mongo.IndexModel
	ReCreatable bool
}

var regexpIndexName = regexp.MustCompile(`name:\s*"([^"]+)"`)

func createIndexes(ctx context.Context, indexView mongo.IndexView, indexDefines []IndexDefine) error {
	var other []mongo.IndexModel

	for _, define := range indexDefines {
		if define.ReCreatable {
			if _, err := indexView.CreateOne(ctx, define.IndexModel); err != nil {
				var commandErr mongo.CommandError
				if errors.As(err, &commandErr) && commandErr.Name == "IndexOptionsConflict" {
					p1 := strings.Index(commandErr.Message, "existing index:")
					if p1 < 0 {
						return err
					}
					matches := regexpIndexName.FindStringSubmatch(commandErr.Message[p1:])
					if len(matches) <= 0 {
						return err
					}
					if _, err = indexView.DropOne(ctx, matches[1]); err != nil {
						return err
					}
					if _, err = indexView.CreateOne(ctx, define.IndexModel); err != nil {
						return err
					}
				} else {
					return err
				}
			}

		} else {
			other = append(other, define.IndexModel)
		}
	}

	_, err := indexView.CreateMany(ctx, other)
	return err
}

// NewDB creates a new MongoDB connection
func NewDB(config *MongoDBConfig) (*MongoDB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Set client options
	clientOptions := options.Client().ApplyURI(config.URI)

	// Connect to MongoDB
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	// Ping the database
	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	db := &MongoDB{
		client:   client,
		database: client.Database(config.Database),
	}

	// Create repositories
	db.taskRepo = NewTaskRepository(db)
	db.queueRepo = NewQueueRepository(db)
	db.workerSessionRepo = NewWorkerSessionRepository(db)

	// Create indexes
	if err := db.createIndexes(ctx); err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return db, nil
}

// Close closes the MongoDB connection
func (m *MongoDB) Close(ctx context.Context) error {
	return m.client.Disconnect(ctx)
}

// GetDatabase returns the database instance
func (m *MongoDB) GetDatabase() *mongo.Database {
	return m.database
}

// GetClient returns the client instance
func (m *MongoDB) GetClient() *mongo.Client {
	return m.client
}

// TasksCollection returns the tasks collection
func (m *MongoDB) TasksCollection() *mongo.Collection {
	return m.database.Collection("tasks")
}

// QueuesCollection returns the queues collection
func (m *MongoDB) QueuesCollection() *mongo.Collection {
	return m.database.Collection("queues")
}

// WorkerSessionsCollection returns the worker sessions collection
func (m *MongoDB) WorkerSessionsCollection() *mongo.Collection {
	return m.database.Collection("worker_sessions")
}

// ProvisionersCollection returns the provisioners collection
func (m *MongoDB) ProvisionersCollection() *mongo.Collection {
	return m.database.Collection("provisioners")
}

// createIndexes creates necessary database indexes
func (m *MongoDB) createIndexes(ctx context.Context) error {
	// Tasks collection indexes
	tasksIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{"status", 1}},
		},
		{
			Keys: bson.D{{"queue", 1}},
		},
		{
			Keys: bson.D{{"created_at", 1}},
		},
		{
			Keys: bson.D{{"worker_id", 1}},
		},
		{
			Keys: bson.D{{"status", 1}, {"queue", 1}},
		},
		{
			Keys: bson.D{{"status", 1}, {"created_at", 1}},
		},
	}

	if _, err := m.TasksCollection().Indexes().CreateMany(ctx, tasksIndexes); err != nil {
		return fmt.Errorf("failed to create tasks indexes: %w", err)
	}

	// Queues collection indexes
	queuesIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{"status", 1}},
		},
		{
			Keys: bson.D{{"created_at", 1}},
		},
	}

	if _, err := m.QueuesCollection().Indexes().CreateMany(ctx, queuesIndexes); err != nil {
		return fmt.Errorf("failed to create queues indexes: %w", err)
	}

	// Worker sessions collection indexes
	workerSessionsIndexes := []IndexDefine{
		{
			IndexModel: mongo.IndexModel{
				Keys: bson.D{{"provisioner_name", 1}},
			},
		},
		{
			IndexModel: mongo.IndexModel{
				Keys: bson.D{{"status", 1}},
			},
		},
		{
			IndexModel: mongo.IndexModel{
				Keys: bson.D{{"connected_at", 1}},
			},
		},
		// TTL index for cleanup - remove sessions after 3 minutes of no heartbeat
		{
			IndexModel: mongo.IndexModel{
				Keys:    bson.D{{"last_heartbeat", 1}},
				Options: options.Index().SetExpireAfterSeconds(60 * 3),
			},
			ReCreatable: true,
		},
	}

	if err := createIndexes(ctx, m.WorkerSessionsCollection().Indexes(), workerSessionsIndexes); err != nil {
		return fmt.Errorf("failed to create worker_sessions indexes: %w", err)
	}

	// Provisioners collection indexes
	provisionersIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{"name", 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{"status", 1}},
		},
		{
			Keys: bson.D{{"last_heartbeat", 1}},
		},
	}

	if _, err := m.ProvisionersCollection().Indexes().CreateMany(ctx, provisionersIndexes); err != nil {
		return fmt.Errorf("failed to create provisioners indexes: %w", err)
	}

	return nil
}

// Health checks the database health
func (m *MongoDB) Health(ctx context.Context) error {
	return m.client.Ping(ctx, nil)
}

// GetTaskRepository returns the task repository
func (m *MongoDB) GetTaskRepository() database2.TaskRepositoryInterface {
	return m.taskRepo
}

// GetQueueRepository returns the queue repository
func (m *MongoDB) GetQueueRepository() database2.QueueRepositoryInterface {
	return m.queueRepo
}

// GetWorkerSessionRepository returns the worker session repository
func (m *MongoDB) GetWorkerSessionRepository() database2.WorkerSessionRepositoryInterface {
	return m.workerSessionRepo
}
