package types

import (
	"fmt"
	"github.com/jc-lab/distworker/go/internal/protocol"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
	"go.mongodb.org/mongo-driver/x/bsonx/bsoncore"
)

//goland:noinspection GoMixedReceiverTypes
func (i WorkerStatus) MarshalBSONValue() (bsontype.Type, []byte, error) {
	return bson.TypeString, bsoncore.AppendString(nil, string(i)), nil
}

func (i *WorkerStatus) UnmarshalBSONValue(t bsontype.Type, value []byte) error {
	if t != bson.TypeString {
		return fmt.Errorf("invalid bson value type '%s'", t.String())
	}
	s, _, ok := bsoncore.ReadString(value)
	if !ok {
		return fmt.Errorf("invalid bson string value")
	}

	*i = WorkerStatus(s)
	return nil
}

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

func ResourceInfoFromProto(info *protocol.ResourceInfo) map[string]interface{} {
	out := make(map[string]interface{})
	out["hostname"] = info.GetHostname()
	out["cpu_cores"] = info.GetCpuCores()
	out["memory_mb"] = info.GetMemoryMb()
	for k, v := range info.GetAdditional().AsMap() {
		out[k] = v
	}
	return out
}
