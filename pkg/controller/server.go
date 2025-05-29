package controller

import (
	"context"
	"fmt"
	database2 "github.com/jc-lab/distworker/pkg/controller/database"
	"github.com/jc-lab/distworker/pkg/controller/storage"
	"github.com/jc-lab/distworker/pkg/models"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"

	"github.com/jc-lab/distworker/pkg/types"
)

// Server represents the main controller server
type Server struct {
	config   *Config
	db       database2.Database
	storage  storage.Storage
	router   *mux.Router
	wsRouter *mux.Router
	upgrader websocket.Upgrader

	// WebSocket connections
	workerConnections map[string]*websocket.Conn

	// Shutdown channel
	shutdown chan struct{}
}

// NewServer creates a new controller server
func NewServer(config *Config, options ...Option) (*Server, error) {
	// Create server
	server := &Server{
		config:            config,
		router:            mux.NewRouter(),
		wsRouter:          mux.NewRouter(),
		workerConnections: make(map[string]*websocket.Conn),
		shutdown:          make(chan struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
	}

	for _, option := range options {
		if err := option(config, server); err != nil {
			return nil, err
		}
	}

	if server.storage == nil {
		if err := defaultStorage(config, server); err != nil {
			return nil, err
		}
	}

	if server.db == nil {
		if err := defaultDatabase(config, server); err != nil {
			return nil, err
		}
	}

	// Setup routes
	server.setupAPIRoutes()
	server.setupWorkerRoutes()

	// Initialize queues
	if err := server.initializeQueues(); err != nil {
		return nil, fmt.Errorf("failed to initialize queues: %w", err)
	}

	return server, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Start API server
	apiServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Server.API.Port),
		Handler:      s.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start Worker server
	workerServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Server.Worker.Port),
		Handler:      s.wsRouter,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start servers in goroutines
	go func() {
		log.Printf("Starting API server on port %d", s.config.Server.API.Port)
		if err := apiServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Starting Worker server on port %d", s.config.Server.Worker.Port)
		if err := workerServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Worker server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-s.shutdown

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Shutting down servers...")
	if err := apiServer.Shutdown(ctx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}
	if err := workerServer.Shutdown(ctx); err != nil {
		log.Printf("Worker server shutdown error: %v", err)
	}

	// Close database connection
	if err := s.db.Close(ctx); err != nil {
		log.Printf("Database close error: %v", err)
	}

	// Close storage connection
	if s.storage != nil {
		if err := s.storage.Close(ctx); err != nil {
			log.Printf("Storage close error: %v", err)
		}
	}

	return nil
}

// Stop stops the server
func (s *Server) Stop() {
	close(s.shutdown)
}

// setupAPIRoutes sets up HTTP API routes
func (s *Server) setupAPIRoutes() {
	api := s.router.PathPrefix("/api/v1").Subrouter()

	// Task routes
	api.HandleFunc("/tasks", s.handleCreateTask).Methods("POST")
	api.HandleFunc("/tasks", s.handleListTasks).Methods("GET")
	api.HandleFunc("/tasks/{task_id}", s.handleGetTask).Methods("GET")
	api.HandleFunc("/tasks/{task_id}", s.handleDeleteTask).Methods("DELETE")

	// Queue routes
	api.HandleFunc("/queues", s.handleCreateQueue).Methods("POST")
	api.HandleFunc("/queues", s.handleListQueues).Methods("GET")
	api.HandleFunc("/queues/{queue_name}", s.handleGetQueue).Methods("GET")
	api.HandleFunc("/queues/{queue_name}", s.handleUpdateQueue).Methods("PUT")
	api.HandleFunc("/queues/{queue_name}", s.handleDeleteQueue).Methods("DELETE")
	api.HandleFunc("/queues/{queue_name}/stats", s.handleGetQueueStats).Methods("GET")

	// Worker routes
	api.HandleFunc("/workers", s.handleListWorkers).Methods("GET")
	api.HandleFunc("/workers/{worker_id}", s.handleDeleteWorker).Methods("DELETE")

	// Provisioner routes
	api.HandleFunc("/provisioners", s.handleListProvisioners).Methods("GET")

	// Health routes
	s.router.HandleFunc("/health", s.handleHealth).Methods("GET")
	s.router.HandleFunc("/metrics", s.handleMetrics).Methods("GET")
}

// setupWorkerRoutes sets up WebSocket and worker-specific routes
func (s *Server) setupWorkerRoutes() {
	// WebSocket endpoint
	s.wsRouter.HandleFunc("/worker/v1/ws", s.handleWorkerWebSocket)

	// File download endpoint
	s.wsRouter.HandleFunc("/worker/v1/file/{file_id}", s.handleFileDownload).Methods("GET")
}

// initializeQueues creates predefined queues from configuration
func (s *Server) initializeQueues() error {
	ctx := context.Background()
	queueRepo := s.db.GetQueueRepository()

	for _, queueConfig := range s.config.Queues {
		// Check if queue already exists
		existing, err := queueRepo.GetByName(ctx, queueConfig.Name)
		if err == nil && existing != nil {
			log.Printf("Queue %s already exists, skipping creation", queueConfig.Name)
			continue
		}

		// Create new queue
		queue := &models.Queue{
			Name:        queueConfig.Name,
			Description: queueConfig.Description,
			Status:      types.QueueStatusInactive, // Start as inactive
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := queueRepo.Create(ctx, queue); err != nil {
			log.Printf("Failed to create queue %s: %v", queueConfig.Name, err)
			continue
		}

		log.Printf("Created queue: %s", queueConfig.Name)
	}

	return nil
}
