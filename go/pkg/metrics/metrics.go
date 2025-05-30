package metrics

import (
	"context"
	"github.com/jc-lab/distworker/go/pkg/controller/database"
	"github.com/jc-lab/distworker/go/pkg/types"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all the Prometheus metrics for the distworker system
type Metrics struct {
	// Task metrics
	TasksTotal      *prometheus.CounterVec
	TasksCreated    prometheus.Counter
	TasksCompleted  prometheus.Counter
	TasksFailed     prometheus.Counter
	TasksCancelled  prometheus.Counter
	TaskDuration    prometheus.Histogram
	TasksInProgress prometheus.Gauge
	TasksPending    prometheus.Gauge

	// Queue metrics
	QueuesTotal     prometheus.Gauge
	QueueTasksTotal *prometheus.GaugeVec

	// Worker metrics
	WorkersTotal     *prometheus.GaugeVec
	WorkersConnected prometheus.Gauge
	WorkerSessions   prometheus.Gauge

	// HTTP metrics
	HTTPRequests *prometheus.CounterVec
	HTTPDuration *prometheus.HistogramVec

	// Storage metrics
	StorageOperations *prometheus.CounterVec
	StorageDuration   *prometheus.HistogramVec
	StorageSize       prometheus.Gauge

	// Database metrics
	DatabaseOperations  *prometheus.CounterVec
	DatabaseDuration    *prometheus.HistogramVec
	DatabaseConnections prometheus.Gauge

	// System metrics
	SystemUptime prometheus.Gauge
	SystemInfo   *prometheus.GaugeVec
}

// NewMetrics creates and registers all Prometheus metrics
func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		// Task metrics
		TasksTotal: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "distworker_tasks_total",
				Help: "Total number of tasks by status",
			},
			[]string{"status", "queue"},
		),
		TasksCreated: promauto.With(registerer).NewCounter(prometheus.CounterOpts{
			Name: "distworker_tasks_created_total",
			Help: "Total number of tasks created",
		}),
		TasksCompleted: promauto.With(registerer).NewCounter(prometheus.CounterOpts{
			Name: "distworker_tasks_completed_total",
			Help: "Total number of tasks completed successfully",
		}),
		TasksFailed: promauto.With(registerer).NewCounter(prometheus.CounterOpts{
			Name: "distworker_tasks_failed_total",
			Help: "Total number of tasks that failed",
		}),
		TasksCancelled: promauto.With(registerer).NewCounter(prometheus.CounterOpts{
			Name: "distworker_tasks_cancelled_total",
			Help: "Total number of tasks cancelled",
		}),
		TaskDuration: promauto.With(registerer).NewHistogram(prometheus.HistogramOpts{
			Name:    "distworker_task_duration_seconds",
			Help:    "Duration of task execution in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~102s
		}),
		TasksInProgress: promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
			Name: "distworker_tasks_in_progress",
			Help: "Number of tasks currently being processed",
		}),
		TasksPending: promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
			Name: "distworker_tasks_pending",
			Help: "Number of tasks waiting to be processed",
		}),

		// Queue metrics
		QueuesTotal: promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
			Name: "distworker_queues_total",
			Help: "Total number of queues",
		}),
		QueueTasksTotal: promauto.With(registerer).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "distworker_queue_tasks_total",
				Help: "Number of tasks per queue by status",
			},
			[]string{"queue", "status"},
		),

		// Worker metrics
		WorkersTotal: promauto.With(registerer).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "distworker_workers_total",
				Help: "Number of workers by status",
			},
			[]string{"status", "provisioner"},
		),
		WorkersConnected: promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
			Name: "distworker_workers_connected",
			Help: "Number of currently connected workers",
		}),
		WorkerSessions: promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
			Name: "distworker_worker_sessions_total",
			Help: "Total number of active worker sessions",
		}),

		// HTTP metrics
		HTTPRequests: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "distworker_http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "endpoint", "status_code"},
		),
		HTTPDuration: promauto.With(registerer).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "distworker_http_request_duration_seconds",
				Help:    "Duration of HTTP requests in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "endpoint"},
		),

		// Storage metrics
		StorageOperations: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "distworker_storage_operations_total",
				Help: "Total number of storage operations",
			},
			[]string{"operation", "status"},
		),
		StorageDuration: promauto.With(registerer).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "distworker_storage_operation_duration_seconds",
				Help:    "Duration of storage operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),
		StorageSize: promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
			Name: "distworker_storage_size_bytes",
			Help: "Total size of stored files in bytes",
		}),

		// Database metrics
		DatabaseOperations: promauto.With(registerer).NewCounterVec(
			prometheus.CounterOpts{
				Name: "distworker_database_operations_total",
				Help: "Total number of database operations",
			},
			[]string{"operation", "collection", "status"},
		),
		DatabaseDuration: promauto.With(registerer).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "distworker_database_operation_duration_seconds",
				Help:    "Duration of database operations in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "collection"},
		),
		DatabaseConnections: promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
			Name: "distworker_database_connections",
			Help: "Number of active database connections",
		}),

		// System metrics
		SystemUptime: promauto.With(registerer).NewGauge(prometheus.GaugeOpts{
			Name: "distworker_uptime_seconds",
			Help: "Time in seconds since the service started",
		}),
		SystemInfo: promauto.With(registerer).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "distworker_info",
				Help: "Information about the distworker instance",
			},
			[]string{"version", "build_time", "go_version"},
		),
	}

	return m
}

// RecordTaskCreated increments the task creation counter
func (m *Metrics) RecordTaskCreated(queue string) {
	m.TasksCreated.Inc()
	m.TasksTotal.WithLabelValues("created", queue).Inc()
}

// RecordTaskCompleted increments the task completion counter and records duration
func (m *Metrics) RecordTaskCompleted(queue string, duration time.Duration) {
	m.TasksCompleted.Inc()
	m.TasksTotal.WithLabelValues("completed", queue).Inc()
	m.TaskDuration.Observe(duration.Seconds())
}

// RecordTaskFailed increments the task failure counter
func (m *Metrics) RecordTaskFailed(queue string) {
	m.TasksFailed.Inc()
	m.TasksTotal.WithLabelValues("failed", queue).Inc()
}

// RecordTaskCancelled increments the task cancellation counter
func (m *Metrics) RecordTaskCancelled(queue string) {
	m.TasksCancelled.Inc()
	m.TasksTotal.WithLabelValues("cancelled", queue).Inc()
}

// RecordHTTPRequest records an HTTP request
func (m *Metrics) RecordHTTPRequest(method, endpoint, statusCode string, duration time.Duration) {
	m.HTTPRequests.WithLabelValues(method, endpoint, statusCode).Inc()
	m.HTTPDuration.WithLabelValues(method, endpoint).Observe(duration.Seconds())
}

// RecordStorageOperation records a storage operation
func (m *Metrics) RecordStorageOperation(operation, status string, duration time.Duration) {
	m.StorageOperations.WithLabelValues(operation, status).Inc()
	m.StorageDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// RecordDatabaseOperation records a database operation
func (m *Metrics) RecordDatabaseOperation(operation, collection, status string, duration time.Duration) {
	m.DatabaseOperations.WithLabelValues(operation, collection, status).Inc()
	m.DatabaseDuration.WithLabelValues(operation, collection).Observe(duration.Seconds())
}

// UpdateSystemMetrics updates system-level metrics
func (m *Metrics) UpdateSystemMetrics(version, buildTime, goVersion string, uptime time.Duration) {
	m.SystemUptime.Set(uptime.Seconds())
	m.SystemInfo.WithLabelValues(version, buildTime, goVersion).Set(1)
}

// CollectDatabaseStats collects current database statistics
func (m *Metrics) CollectDatabaseStats(ctx context.Context, db database.Database) {
	// Collect task counts by status
	pendingTasks, _, _ := db.GetTaskRepository().List(ctx, database.TaskFilter{Status: string(types.TaskStatusPending)}, 1, 1)
	if pendingTasks != nil {
		m.TasksPending.Set(float64(len(pendingTasks)))
	}

	processingTasks, _, _ := db.GetTaskRepository().List(ctx, database.TaskFilter{Status: string(types.TaskStatusProcessing)}, 1, 1)
	if processingTasks != nil {
		m.TasksInProgress.Set(float64(len(processingTasks)))
	}

	// Collect queue counts
	queues, _ := db.GetQueueRepository().List(ctx)
	if queues != nil {
		m.QueuesTotal.Set(float64(len(queues)))

		// Collect tasks per queue
		for _, queue := range queues {
			for _, status := range []types.TaskStatus{types.TaskStatusPending, types.TaskStatusProcessing, types.TaskStatusCompleted, types.TaskStatusFailed, types.TaskStatusCancelled} {
				tasks, total, _ := db.GetTaskRepository().List(ctx, database.TaskFilter{Queue: queue.Name, Status: string(status)}, 1, 1)
				_ = tasks // We only need the total count
				m.QueueTasksTotal.WithLabelValues(queue.Name, string(status)).Set(float64(total))
			}
		}
	}

	// Collect worker session counts
	sessions, _ := db.GetWorkerSessionRepository().List(ctx)
	if sessions != nil {
		m.WorkerSessions.Set(float64(len(sessions)))

		// Count workers by status
		statusCounts := make(map[types.WorkerStatus]int)
		provisionerCounts := make(map[string]map[types.WorkerStatus]int)

		for _, session := range sessions {
			statusCounts[session.Status]++

			if provisionerCounts[session.ProvisionerName] == nil {
				provisionerCounts[session.ProvisionerName] = make(map[types.WorkerStatus]int)
			}
			provisionerCounts[session.ProvisionerName][session.Status]++
		}

		// Update worker metrics
		for status, count := range statusCounts {
			m.WorkersTotal.WithLabelValues(string(status), "").Set(float64(count))
		}

		for provisioner, statusMap := range provisionerCounts {
			for status, count := range statusMap {
				m.WorkersTotal.WithLabelValues(string(status), provisioner).Set(float64(count))
			}
		}

		// Count connected workers
		connectedCount := statusCounts[types.WorkerStatusIdle] + statusCounts[types.WorkerStatusProcessing]
		m.WorkersConnected.Set(float64(connectedCount))
	}
}
