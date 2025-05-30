package types

// TaskStatus represents the status of a task
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusProcessing TaskStatus = "processing"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusFailed     TaskStatus = "failed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// WorkerStatus represents the status of a worker
type WorkerStatus string

const (
	WorkerStatusIdle          WorkerStatus = "idle"
	WorkerStatusProcessing    WorkerStatus = "processing"
	WorkerStatusDisconnecting WorkerStatus = "disconnecting"
	WorkerStatusError         WorkerStatus = "error"
)

// QueueStatus represents the status of a queue
type QueueStatus string

const (
	QueueStatusActive   QueueStatus = "active"
	QueueStatusInactive QueueStatus = "inactive"
)

// HealthStatus represents the health status of a service component
type HealthStatus string

const (
	HealthStatusUp       HealthStatus = "up"
	HealthStatusDown     HealthStatus = "down"
	HealthStatusDegraded HealthStatus = "degraded"
)

// ProvisionerStatus represents the status of a provisioner
type ProvisionerStatus string

const (
	ProvisionerStatusUp   ProvisionerStatus = "up"
	ProvisionerStatusDown ProvisionerStatus = "down"
)

// TaskErrorCode represents predefined error codes for task failures
type TaskErrorCode string

const (
	TaskErrorCodeTimeout      TaskErrorCode = "TIMEOUT"
	TaskErrorCodeWorkerError  TaskErrorCode = "WORKER_ERROR"
	TaskErrorCodeNetworkError TaskErrorCode = "NETWORK_ERROR"
	TaskErrorCodeUnknown      TaskErrorCode = "UNKNOWN"
)