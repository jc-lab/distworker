package types

import (
	"github.com/jc-lab/distworker/internal/protocol"
)

// ConvertTaskStatusFromProto converts protobuf TaskStatus to pkg TaskStatus
func ConvertTaskStatusFromProto(protoStatus protocol.TaskStatus) TaskStatus {
	switch protoStatus {
	case protocol.TaskStatus_TASK_STATUS_PENDING:
		return TaskStatusPending
	case protocol.TaskStatus_TASK_STATUS_PROCESSING:
		return TaskStatusProcessing
	case protocol.TaskStatus_TASK_STATUS_COMPLETED:
		return TaskStatusCompleted
	case protocol.TaskStatus_TASK_STATUS_FAILED:
		return TaskStatusFailed
	case protocol.TaskStatus_TASK_STATUS_CANCELLED:
		return TaskStatusCancelled
	default:
		return TaskStatusPending
	}
}

// ConvertTaskStatusToProto converts pkg TaskStatus to protobuf TaskStatus
func ConvertTaskStatusToProto(status TaskStatus) protocol.TaskStatus {
	switch status {
	case TaskStatusPending:
		return protocol.TaskStatus_TASK_STATUS_PENDING
	case TaskStatusProcessing:
		return protocol.TaskStatus_TASK_STATUS_PROCESSING
	case TaskStatusCompleted:
		return protocol.TaskStatus_TASK_STATUS_COMPLETED
	case TaskStatusFailed:
		return protocol.TaskStatus_TASK_STATUS_FAILED
	case TaskStatusCancelled:
		return protocol.TaskStatus_TASK_STATUS_CANCELLED
	default:
		return protocol.TaskStatus_TASK_STATUS_PENDING
	}
}

// ConvertWorkerStatusFromProto converts protobuf WorkerStatus to pkg WorkerStatus
func ConvertWorkerStatusFromProto(protoStatus protocol.WorkerStatus) WorkerStatus {
	switch protoStatus {
	case protocol.WorkerStatus_WORKER_STATUS_IDLE:
		return WorkerStatusIdle
	case protocol.WorkerStatus_WORKER_STATUS_PROCESSING:
		return WorkerStatusProcessing
	case protocol.WorkerStatus_WORKER_STATUS_DISCONNECTING:
		return WorkerStatusDisconnecting
	case protocol.WorkerStatus_WORKER_STATUS_ERROR:
		return WorkerStatusError
	default:
		return WorkerStatusIdle
	}
}

// ConvertWorkerStatusToProto converts pkg WorkerStatus to protobuf WorkerStatus
func ConvertWorkerStatusToProto(status WorkerStatus) protocol.WorkerStatus {
	switch status {
	case WorkerStatusIdle:
		return protocol.WorkerStatus_WORKER_STATUS_IDLE
	case WorkerStatusProcessing:
		return protocol.WorkerStatus_WORKER_STATUS_PROCESSING
	case WorkerStatusDisconnecting:
		return protocol.WorkerStatus_WORKER_STATUS_DISCONNECTING
	case WorkerStatusError:
		return protocol.WorkerStatus_WORKER_STATUS_ERROR
	default:
		return protocol.WorkerStatus_WORKER_STATUS_IDLE
	}
}