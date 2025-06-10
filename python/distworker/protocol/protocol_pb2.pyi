from google.protobuf import struct_pb2 as _struct_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class MessageType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    MESSAGE_TYPE_UNSPECIFIED: _ClassVar[MessageType]
    MESSAGE_TYPE_WORKER_REGISTER: _ClassVar[MessageType]
    MESSAGE_TYPE_WORKER_REGISTER_RESPONSE: _ClassVar[MessageType]
    MESSAGE_TYPE_HEARTBEAT: _ClassVar[MessageType]
    MESSAGE_TYPE_TASK_ASSIGN: _ClassVar[MessageType]
    MESSAGE_TYPE_TASK_PROGRESS: _ClassVar[MessageType]
    MESSAGE_TYPE_TASK_COMPLETE: _ClassVar[MessageType]
    MESSAGE_TYPE_TASK_FAILED: _ClassVar[MessageType]

class WorkerHealth(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    WORKER_HEALTH_UNSPECIFIED: _ClassVar[WorkerHealth]
    WORKER_HEALTH_UP: _ClassVar[WorkerHealth]
    WORKER_HEALTH_DOWN: _ClassVar[WorkerHealth]
    WORKER_HEALTH_WARN: _ClassVar[WorkerHealth]

class TaskStatus(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    TASK_STATUS_UNSPECIFIED: _ClassVar[TaskStatus]
    TASK_STATUS_PENDING: _ClassVar[TaskStatus]
    TASK_STATUS_PROCESSING: _ClassVar[TaskStatus]
    TASK_STATUS_COMPLETED: _ClassVar[TaskStatus]
    TASK_STATUS_FAILED: _ClassVar[TaskStatus]
    TASK_STATUS_CANCELLED: _ClassVar[TaskStatus]
MESSAGE_TYPE_UNSPECIFIED: MessageType
MESSAGE_TYPE_WORKER_REGISTER: MessageType
MESSAGE_TYPE_WORKER_REGISTER_RESPONSE: MessageType
MESSAGE_TYPE_HEARTBEAT: MessageType
MESSAGE_TYPE_TASK_ASSIGN: MessageType
MESSAGE_TYPE_TASK_PROGRESS: MessageType
MESSAGE_TYPE_TASK_COMPLETE: MessageType
MESSAGE_TYPE_TASK_FAILED: MessageType
WORKER_HEALTH_UNSPECIFIED: WorkerHealth
WORKER_HEALTH_UP: WorkerHealth
WORKER_HEALTH_DOWN: WorkerHealth
WORKER_HEALTH_WARN: WorkerHealth
TASK_STATUS_UNSPECIFIED: TaskStatus
TASK_STATUS_PENDING: TaskStatus
TASK_STATUS_PROCESSING: TaskStatus
TASK_STATUS_COMPLETED: TaskStatus
TASK_STATUS_FAILED: TaskStatus
TASK_STATUS_CANCELLED: TaskStatus

class WorkerRegister(_message.Message):
    __slots__ = ("data", "signature")
    DATA_FIELD_NUMBER: _ClassVar[int]
    SIGNATURE_FIELD_NUMBER: _ClassVar[int]
    data: bytes
    signature: bytes
    def __init__(self, data: _Optional[bytes] = ..., signature: _Optional[bytes] = ...) -> None: ...

class WorkerRegisterResponse(_message.Message):
    __slots__ = ("idle_time",)
    IDLE_TIME_FIELD_NUMBER: _ClassVar[int]
    idle_time: int
    def __init__(self, idle_time: _Optional[int] = ...) -> None: ...

class SignedWorkerRegister(_message.Message):
    __slots__ = ("provisioner_name", "worker_id", "date", "resource_info")
    PROVISIONER_NAME_FIELD_NUMBER: _ClassVar[int]
    WORKER_ID_FIELD_NUMBER: _ClassVar[int]
    DATE_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_INFO_FIELD_NUMBER: _ClassVar[int]
    provisioner_name: str
    worker_id: str
    date: str
    resource_info: ResourceInfo
    def __init__(self, provisioner_name: _Optional[str] = ..., worker_id: _Optional[str] = ..., date: _Optional[str] = ..., resource_info: _Optional[_Union[ResourceInfo, _Mapping]] = ...) -> None: ...

class TaskAssign(_message.Message):
    __slots__ = ("task_id", "queue", "metadata", "input", "timeout_ms", "files")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    QUEUE_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    INPUT_FIELD_NUMBER: _ClassVar[int]
    TIMEOUT_MS_FIELD_NUMBER: _ClassVar[int]
    FILES_FIELD_NUMBER: _ClassVar[int]
    task_id: str
    queue: str
    metadata: _struct_pb2.Struct
    input: _struct_pb2.Struct
    timeout_ms: int
    files: _containers.RepeatedCompositeFieldContainer[FileInfo]
    def __init__(self, task_id: _Optional[str] = ..., queue: _Optional[str] = ..., metadata: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., input: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., timeout_ms: _Optional[int] = ..., files: _Optional[_Iterable[_Union[FileInfo, _Mapping]]] = ...) -> None: ...

class TaskProgress(_message.Message):
    __slots__ = ("task_id", "progress", "message", "data")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    PROGRESS_FIELD_NUMBER: _ClassVar[int]
    MESSAGE_FIELD_NUMBER: _ClassVar[int]
    DATA_FIELD_NUMBER: _ClassVar[int]
    task_id: str
    progress: float
    message: str
    data: _struct_pb2.Struct
    def __init__(self, task_id: _Optional[str] = ..., progress: _Optional[float] = ..., message: _Optional[str] = ..., data: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class TaskComplete(_message.Message):
    __slots__ = ("task_id", "result", "result_files")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    RESULT_FIELD_NUMBER: _ClassVar[int]
    RESULT_FILES_FIELD_NUMBER: _ClassVar[int]
    task_id: str
    result: _struct_pb2.Struct
    result_files: _containers.RepeatedCompositeFieldContainer[FileInfo]
    def __init__(self, task_id: _Optional[str] = ..., result: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ..., result_files: _Optional[_Iterable[_Union[FileInfo, _Mapping]]] = ...) -> None: ...

class TaskFailed(_message.Message):
    __slots__ = ("task_id", "error_code", "error_message", "error_details")
    TASK_ID_FIELD_NUMBER: _ClassVar[int]
    ERROR_CODE_FIELD_NUMBER: _ClassVar[int]
    ERROR_MESSAGE_FIELD_NUMBER: _ClassVar[int]
    ERROR_DETAILS_FIELD_NUMBER: _ClassVar[int]
    task_id: str
    error_code: str
    error_message: str
    error_details: _struct_pb2.Struct
    def __init__(self, task_id: _Optional[str] = ..., error_code: _Optional[str] = ..., error_message: _Optional[str] = ..., error_details: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class Heartbeat(_message.Message):
    __slots__ = ("timestamp", "health", "resource_usage")
    TIMESTAMP_FIELD_NUMBER: _ClassVar[int]
    HEALTH_FIELD_NUMBER: _ClassVar[int]
    RESOURCE_USAGE_FIELD_NUMBER: _ClassVar[int]
    timestamp: int
    health: WorkerHealth
    resource_usage: ResourceUsage
    def __init__(self, timestamp: _Optional[int] = ..., health: _Optional[_Union[WorkerHealth, str]] = ..., resource_usage: _Optional[_Union[ResourceUsage, _Mapping]] = ...) -> None: ...

class ResourceInfo(_message.Message):
    __slots__ = ("hostname", "cpu_cores", "memory_mb", "additional")
    HOSTNAME_FIELD_NUMBER: _ClassVar[int]
    CPU_CORES_FIELD_NUMBER: _ClassVar[int]
    MEMORY_MB_FIELD_NUMBER: _ClassVar[int]
    ADDITIONAL_FIELD_NUMBER: _ClassVar[int]
    hostname: str
    cpu_cores: int
    memory_mb: int
    additional: _struct_pb2.Struct
    def __init__(self, hostname: _Optional[str] = ..., cpu_cores: _Optional[int] = ..., memory_mb: _Optional[int] = ..., additional: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class ResourceUsage(_message.Message):
    __slots__ = ("cpu_percent", "memory_used_mb", "gpu_utilization", "additional")
    CPU_PERCENT_FIELD_NUMBER: _ClassVar[int]
    MEMORY_USED_MB_FIELD_NUMBER: _ClassVar[int]
    GPU_UTILIZATION_FIELD_NUMBER: _ClassVar[int]
    ADDITIONAL_FIELD_NUMBER: _ClassVar[int]
    cpu_percent: float
    memory_used_mb: int
    gpu_utilization: float
    additional: _struct_pb2.Struct
    def __init__(self, cpu_percent: _Optional[float] = ..., memory_used_mb: _Optional[int] = ..., gpu_utilization: _Optional[float] = ..., additional: _Optional[_Union[_struct_pb2.Struct, _Mapping]] = ...) -> None: ...

class FileInfo(_message.Message):
    __slots__ = ("file_id", "filename", "content_type", "size", "storage_url")
    FILE_ID_FIELD_NUMBER: _ClassVar[int]
    FILENAME_FIELD_NUMBER: _ClassVar[int]
    CONTENT_TYPE_FIELD_NUMBER: _ClassVar[int]
    SIZE_FIELD_NUMBER: _ClassVar[int]
    STORAGE_URL_FIELD_NUMBER: _ClassVar[int]
    file_id: str
    filename: str
    content_type: str
    size: int
    storage_url: str
    def __init__(self, file_id: _Optional[str] = ..., filename: _Optional[str] = ..., content_type: _Optional[str] = ..., size: _Optional[int] = ..., storage_url: _Optional[str] = ...) -> None: ...

class WebSocketMessage(_message.Message):
    __slots__ = ("type", "worker_register", "worker_register_response", "heartbeat", "task_assign", "task_progress", "task_complete", "task_failed")
    TYPE_FIELD_NUMBER: _ClassVar[int]
    WORKER_REGISTER_FIELD_NUMBER: _ClassVar[int]
    WORKER_REGISTER_RESPONSE_FIELD_NUMBER: _ClassVar[int]
    HEARTBEAT_FIELD_NUMBER: _ClassVar[int]
    TASK_ASSIGN_FIELD_NUMBER: _ClassVar[int]
    TASK_PROGRESS_FIELD_NUMBER: _ClassVar[int]
    TASK_COMPLETE_FIELD_NUMBER: _ClassVar[int]
    TASK_FAILED_FIELD_NUMBER: _ClassVar[int]
    type: MessageType
    worker_register: WorkerRegister
    worker_register_response: WorkerRegisterResponse
    heartbeat: Heartbeat
    task_assign: TaskAssign
    task_progress: TaskProgress
    task_complete: TaskComplete
    task_failed: TaskFailed
    def __init__(self, type: _Optional[_Union[MessageType, str]] = ..., worker_register: _Optional[_Union[WorkerRegister, _Mapping]] = ..., worker_register_response: _Optional[_Union[WorkerRegisterResponse, _Mapping]] = ..., heartbeat: _Optional[_Union[Heartbeat, _Mapping]] = ..., task_assign: _Optional[_Union[TaskAssign, _Mapping]] = ..., task_progress: _Optional[_Union[TaskProgress, _Mapping]] = ..., task_complete: _Optional[_Union[TaskComplete, _Mapping]] = ..., task_failed: _Optional[_Union[TaskFailed, _Mapping]] = ...) -> None: ...
