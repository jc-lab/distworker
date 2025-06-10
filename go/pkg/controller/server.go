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

// Package controller provides the main controller server
// @title Distworker API
// @version 1.0
// @description Distributed task processing system API
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io
// @license.name AGPL-3.0
// @license.url https://www.gnu.org/licenses/agpl-3.0.html
// @host localhost:8080
// @BasePath /api/v1
// @schemes http https
package controller

import (
	"context"
	"errors"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/jc-lab/distworker/go/internal/provisioner"
	"github.com/jc-lab/distworker/go/internal/version"
	config2 "github.com/jc-lab/distworker/go/pkg/controller/config"
	database2 "github.com/jc-lab/distworker/go/pkg/controller/database"
	"github.com/jc-lab/distworker/go/pkg/controller/storage"
	wsmanager "github.com/jc-lab/distworker/go/pkg/controller/worker"
	websocket2 "github.com/jc-lab/distworker/go/pkg/controller/worker/websocket"
	"github.com/jc-lab/distworker/go/pkg/metrics"
	models2 "github.com/jc-lab/distworker/go/pkg/models"
	"github.com/jc-lab/distworker/go/pkg/types"
	errors2 "github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"
	"log"
	"net"
	"net/http"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// @host

// Server represents the main controller server
type Server struct {
	config     *config2.Config
	db         database2.Database
	storage    storage.Storage
	rootLogger *zap.Logger
	logger     *zap.SugaredLogger

	router   *mux.Router
	wsRouter *mux.Router
	upgrader websocket.Upgrader

	// Metrics
	metrics        *metrics.Metrics
	metricsHandler http.Handler
	startTime      time.Time

	// Provisioner management
	provisionerManager provisioner.Manager

	// Worker management
	workerManager *wsmanager.Manager

	websocketListener *websocket2.Listener

	// Shutdown channel
	shutdown          chan struct{}
	shutdownRequested bool
	mu                sync.Mutex
}

// NewServer creates a new controller server
func NewServer(config *config2.Config, options ...Option) (*Server, error) {
	var err error

	ctx, cancel := context.WithCancel(context.Background())
	_ = cancel

	// Initialize metrics
	promRegistry := prometheus.NewRegistry()
	appMetrics := metrics.NewMetrics(promRegistry)

	// Create server
	server := &Server{
		config:   config,
		router:   mux.NewRouter(),
		wsRouter: mux.NewRouter(),
		metrics:  appMetrics,
		metricsHandler: promhttp.InstrumentMetricHandler(
			promRegistry, promhttp.HandlerFor(promRegistry, promhttp.HandlerOpts{}),
		),
		startTime: time.Now(),
		shutdown:  make(chan struct{}),
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

	if server.rootLogger == nil {
		server.rootLogger = makeDefaultLogger()
	}
	server.logger = server.rootLogger.Named("server").Sugar()

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

	server.provisionerManager, err = provisioner.NewManager(ctx, config.Provisioner, &config.Server.Worker, server.db, server.rootLogger)
	if err != nil {
		return nil, err
	}

	server.workerManager = wsmanager.NewManager(ctx, server.db, server.provisionerManager, server.rootLogger)
	server.provisionerManager.SetWorkerManager(server.workerManager)

	server.websocketListener = websocket2.NewListener(ctx, server.db, server.provisionerManager, server.workerManager)

	// Setup routes
	server.setupAPIRoutes()
	server.setupWorkerRoutes()

	// Initialize queues
	if err := server.initializeQueues(); err != nil {
		return nil, fmt.Errorf("failed to initialize queues: %w", err)
	}

	// Initialize system metrics
	server.metrics.UpdateSystemMetrics(
		version.Version,
		version.BuildTimestamp,
		runtime.Version(),
		time.Since(server.startTime),
	)

	return server, nil
}

// Start starts the server
func (s *Server) Start() error {
	if err := s.workerManager.Start(); err != nil {
		return err
	}

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

	if s.config.ControllerSetting.WorkerAccessibleBaseUrl == "" {
		s.config.ControllerSetting.WorkerAccessibleBaseUrl = fmt.Sprintf("http://127.0.0.1:%d", s.config.Server.Worker.Port)
	}

	// Start metrics collection goroutine
	go s.startMetricsCollection()

	apiListener, err := net.Listen("tcp", apiServer.Addr)
	if err != nil {
		return errors2.Wrap(err, "failed to start API server")
	}

	workerListener, err := net.Listen("tcp", workerServer.Addr)
	if err != nil {
		return errors2.Wrap(err, "failed to start Worker server")
	}

	// Start servers in goroutines
	go func() {
		defer apiListener.Close()

		s.router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
			//httpSwagger.URL("/swagger/doc.json"),
			httpSwagger.DeepLinking(true),
			httpSwagger.DocExpansion("none"),
			httpSwagger.DomID("swagger-ui"),
		)).Methods(http.MethodGet)

		log.Printf("Starting API server on port %d", s.config.Server.API.Port)
		if err := apiServer.Serve(apiListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("API server error: %v", err)
			s.Stop()
		}
	}()

	go func() {
		defer workerListener.Close()

		log.Printf("Starting Worker server on port %d", s.config.Server.Worker.Port)
		if err := workerServer.Serve(workerListener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("Worker server error: %v", err)
			s.Stop()
		}
	}()

	_, _ = <-s.shutdown

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log.Println("Shutting down servers...")

	// Shutdown worker manager first
	log.Println("Shutting down worker manager...")
	if s.workerManager != nil {
		s.workerManager.Shutdown()
	}

	log.Println("Shutting down api server...")
	if err := apiServer.Shutdown(ctx); err != nil {
		log.Printf("API server shutdown error: %v", err)
	}

	log.Println("Shutting down worker server...")
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.shutdownRequested {
		s.shutdownRequested = true
		close(s.shutdown)
	}
}

// setupAPIRoutes sets up HTTP API routes
func (s *Server) setupAPIRoutes() {
	api := s.router.PathPrefix("/api/v1").Subrouter()
	api.Use(handlers.CORS(
		handlers.AllowedMethods([]string{"GET"}),
	))

	api.HandleFunc("/tasks", s.handleCreateTask).Methods("POST")
	api.HandleFunc("/tasks", s.handleListTasks).Methods("GET")
	api.HandleFunc("/tasks/{task_id}", s.handleGetTask).Methods("GET")
	api.HandleFunc("/tasks/{task_id}", s.handleDeleteTask).Methods("DELETE")

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
	// @Summary Worker WebSocket connection
	// @Description Establish WebSocket connection for workers
	// @Tags workers
	// @Accept json
	// @Produce json
	// @Success 101 {string} string "Switching Protocols"
	// @Failure 400 {object} api.ErrorResponse
	// @Router /worker/v1/ws [get]
	s.wsRouter.HandleFunc("/worker/v1/ws", s.handleWorkerWebSocket)

	// File download endpoint
	// @Summary Download file
	// @Description Download a file by ID (worker authentication required)
	// @Tags files
	// @Accept */*
	// @Produce application/octet-stream
	// @Param file_id path string true "File ID"
	// @Success 200 {file} binary "File content"
	// @Failure 400 {object} api.ErrorResponse
	// @Failure 403 {object} api.ErrorResponse
	// @Failure 404 {object} api.ErrorResponse
	// @Failure 500 {object} api.ErrorResponse
	// @Router /worker/v1/file/{file_id} [get]
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
		queue := &models2.Queue{
			Name:        queueConfig.Name,
			Description: queueConfig.Description,
			Status:      types.QueueStatusInactive, // Start as inactive
			CreatedAt:   models2.Now(),
			UpdatedAt:   models2.Now(),
		}

		if err := queueRepo.Create(ctx, queue); err != nil {
			log.Printf("Failed to create queue %s: %v", queueConfig.Name, err)
			continue
		}

		log.Printf("Created queue: %s", queueConfig.Name)
	}

	return nil
}

// startMetricsCollection starts a goroutine to periodically collect metrics
func (s *Server) startMetricsCollection() {
	ticker := time.NewTicker(30 * time.Second) // Collect metrics every 30 seconds
	defer ticker.Stop()

	ctx := context.Background()

	for {
		select {
		case <-ticker.C:
			// Update system uptime
			s.metrics.UpdateSystemMetrics(
				version.Version,
				version.BuildTimestamp,
				runtime.Version(),
				time.Since(s.startTime),
			)

			// Collect statistics
			s.metrics.CollectDatabaseStats(ctx, s.db)
			s.metrics.CollectWorkerStats(ctx, s.workerManager)

		case <-s.shutdown:
			return
		}
	}
}
