"""
DistWorker Python SDK - Worker Class
"""

import asyncio
import logging
import socket
import traceback
from datetime import datetime
from typing import Dict, Any, List, Optional, Callable, Awaitable
from urllib.parse import urlparse, urlunparse

import psutil
import websockets
from websockets.exceptions import ConnectionClosed, InvalidStatusCode

from .auth import generate_websocket_signature, DATE_ONLY_FORMAT, DATE_FORMAT
from .exceptions import ConnectionError, AuthenticationError, TaskError, ProtocolError
from .task import Task
from ..protocol.protocol_pb2 import (
    WebSocketMessage, MessageType, WorkerRegister, SignedWorkerRegister,
    ResourceInfo, Heartbeat, ResourceUsage, TaskProgress, TaskComplete,
    TaskFailed, TaskAssign
)

logger = logging.getLogger(__name__)


class Worker:
    """
    DistWorker Python SDK Worker Client
    
    Connects to the DistWorker controller and processes assigned tasks.
    """
    
    def __init__(
        self,
        controller_url: str,
        provisioner_name: str,
        worker_id: str,
        worker_token: str,
        resource_info: Optional[dict[str, Any]] = None,
        reconnect_interval: float = 5.0,
        heartbeat_interval: float = 30.0,
        max_reconnect_attempts: int = -1
    ):
        """
        Initialize worker client
        
        Args:
            controller_url: URL of the controller (http://host:port/)
            worker_id: Unique worker identifier
            worker_token: Worker authentication token
            resource_info: Optional resource information (CPU, memory, etc.)
            reconnect_interval: Seconds between reconnection attempts
            heartbeat_interval: Seconds between heartbeat messages
            max_reconnect_attempts: Maximum reconnection attempts (-1 for unlimited)
        """
        self.controller_url = controller_url
        self.provisioner_name = provisioner_name
        self.worker_id = worker_id
        self.worker_token = worker_token
        self.resource_info = resource_info or {}
        self.reconnect_interval = reconnect_interval
        self.heartbeat_interval = heartbeat_interval
        self.max_reconnect_attempts = max_reconnect_attempts
        
        # Connection state
        self.websocket: Optional[websockets.WebSocketServerProtocol] = None
        self.connected = False
        self.running = False
        self.reconnect_count = 0
        
        # Task handling
        self.task_handlers: Dict[str, Callable[[Task], Awaitable[Dict[str, Any]]]] = {}
        self.current_task: Optional[Task] = None
        
        # Background tasks
        self._heartbeat_task: Optional[asyncio.Task] = None
        self._connection_task: Optional[asyncio.Task] = None
        
    def register_handler(self, queue_pattern: str, handler: Callable[[Task], Awaitable[Dict[str, Any]]]):
        """
        Register a task handler for a specific queue pattern
        
        Args:
            queue_pattern: Queue pattern to handle (supports * and # wildcards)
            handler: Async function that processes tasks and returns results
        """
        self.task_handlers[queue_pattern] = handler
        
    async def start(self):
        """Start the worker and connect to the controller"""
        if self.running:
            return
            
        self.running = True
        logger.info(f"Starting worker {self.worker_id}")
        
        # Start connection management task
        self._connection_task = asyncio.create_task(self._connection_manager())
        
    async def stop(self):
        """Stop the worker and disconnect from the controller"""
        if not self.running:
            return
            
        self.running = False
        logger.info(f"Stopping worker {self.worker_id}")
        
        # Cancel background tasks
        if self._heartbeat_task:
            self._heartbeat_task.cancel()
        if self._connection_task:
            self._connection_task.cancel()
            
        # Close WebSocket connection
        if self.websocket:
            await self.websocket.close()
            
        self.connected = False
        
    async def _connection_manager(self):
        """Manage WebSocket connection with automatic reconnection"""
        while self.running:
            try:
                await self._connect_and_run()
            except Exception as e:
                logger.error(f"Connection error: {e}")
                
                if not self.running:
                    break
                    
                self.reconnect_count += 1
                if 0 <= self.max_reconnect_attempts < self.reconnect_count:
                    logger.error(f"Max reconnect attempts ({self.max_reconnect_attempts}) exceeded")
                    break
                    
                logger.info(f"Reconnecting in {self.reconnect_interval} seconds (attempt {self.reconnect_count})")
                await asyncio.sleep(self.reconnect_interval)
                
    async def _connect_and_run(self):
        """Connect to controller and handle messages"""
        try:
            # Parse controller URL
            parsed_url = urlparse(self.controller_url)
            if parsed_url.scheme not in ['http', 'https']:
                raise ConnectionError(f"Invalid Controller URL scheme: {parsed_url.scheme}")

            # 웹소켓 스키마 결정
            ws_scheme = "wss" if parsed_url.scheme == "https" else "ws"

            # 새 URL 생성 (ParseResult의 요소들을 사용)
            ws_url = urlunparse((
                ws_scheme,
                parsed_url.netloc,
                parsed_url.path.rstrip("/") + "/worker/v1/ws",
                parsed_url.params,
                parsed_url.query,
                parsed_url.fragment,
            ))

            logger.info(f"Connecting to {ws_url}")
            
            # Connect to WebSocket
            async with websockets.connect(
                ws_url,
                ping_interval=20,
                ping_timeout=10,
                close_timeout=10
            ) as websocket:
                self.websocket = websocket
                self.connected = True
                self.reconnect_count = 0
                
                logger.info(f"Connected to controller")
                
                # Send worker registration
                await self._send_registration()
                
                # Start heartbeat task
                self._heartbeat_task = asyncio.create_task(self._heartbeat_worker())
                
                # Handle incoming messages
                await self._message_handler()
                
        except (ConnectionClosed, InvalidStatusCode) as e:
            raise ConnectionError(f"WebSocket connection failed: {e}")
        except Exception as e:
            raise ConnectionError(f"Connection error: {e}")
        finally:
            self.connected = False
            if self._heartbeat_task:
                self._heartbeat_task.cancel()
                
    async def _send_registration(self):
        """Send worker registration message"""
        try:
            now = datetime.utcnow()

            # Create resource info protobuf
            resource_proto = ResourceInfo()
            resource_proto.hostname = self.resource_info.get('hostname', socket.gethostname())
            resource_proto.cpu_cores = self.resource_info.get('cpu_cores', psutil.cpu_count(logical=True))
            resource_proto.memory_mb = self.resource_info.get('memory_mb', psutil.virtual_memory().total/1024/1024)
            
            # Add additional resource info
            additional = {}
            for key, value in self.resource_info.items():
                if key not in ['cpu_cores', 'memory_mb']:
                    additional[key] = value
            if additional:
                from google.protobuf.struct_pb2 import Struct
                resource_proto.additional.CopyFrom(Struct())
                resource_proto.additional.update(additional)
            
            # Create signed registration
            signed_register = SignedWorkerRegister()
            signed_register.provisioner_name = self.provisioner_name
            signed_register.worker_id = self.worker_id
            signed_register.date = now.strftime(DATE_FORMAT)
            signed_register.resource_info.CopyFrom(resource_proto)

            # Serialize for signing
            signed_data = signed_register.SerializeToString()
            
            register = WorkerRegister()
            register.data = signed_data
            register.signature = generate_websocket_signature(
                self.worker_token, now.strftime(DATE_ONLY_FORMAT), signed_data
            )
            
            # Create WebSocket message
            ws_msg = WebSocketMessage()
            ws_msg.type = MessageType.MESSAGE_TYPE_WORKER_REGISTER
            ws_msg.worker_register.CopyFrom(register)
            
            # Send registration
            await self._send_message(ws_msg)
            logger.info(f"Worker registration sent")
            
        except Exception as e:
            traceback.print_exception(e)
            raise AuthenticationError(f"Registration failed: {e}")
            
    async def _heartbeat_worker(self):
        """Send periodic heartbeat messages"""
        while self.connected and self.running:
            try:
                await self._send_heartbeat()
                await asyncio.sleep(self.heartbeat_interval)
            except asyncio.CancelledError:
                break
            except Exception as e:
                logger.error(f"Heartbeat error: {e}")
                break
                
    async def _send_heartbeat(self):
        """Send heartbeat message with current status"""
        try:
            # Create resource usage
            resource_usage = ResourceUsage()
            resource_usage.cpu_percent = self.resource_info.get('cpu_percent', 0.0)
            resource_usage.memory_used_mb = self.resource_info.get('memory_used_mb', 0)
            resource_usage.gpu_utilization = self.resource_info.get('gpu_utilization', 0.0)
            
            # Create heartbeat
            heartbeat = Heartbeat()
            heartbeat.status = 1 if self.current_task is None else 2  # IDLE or PROCESSING
            heartbeat.resource_usage.CopyFrom(resource_usage)
            
            # Create WebSocket message
            ws_msg = WebSocketMessage()
            ws_msg.type = MessageType.MESSAGE_TYPE_HEARTBEAT
            ws_msg.heartbeat.CopyFrom(heartbeat)
            
            await self._send_message(ws_msg)
            
        except Exception as e:
            logger.error(f"Failed to send heartbeat: {e}")
            
    async def _message_handler(self):
        """Handle incoming WebSocket messages"""
        async for message in self.websocket:
            try:
                if isinstance(message, bytes):
                    await self._handle_binary_message(message)
                else:
                    logger.warning(f"Received non-binary message: {type(message)}")
            except Exception as e:
                logger.error(f"Message handling error: {e}")
                
    async def _handle_binary_message(self, data: bytes):
        """Handle binary protobuf messages"""
        try:
            ws_msg = WebSocketMessage()
            ws_msg.ParseFromString(data)
            
            if ws_msg.type == MessageType.MESSAGE_TYPE_TASK_ASSIGN:
                await self._handle_task_assignment(ws_msg.task_assign)
            else:
                logger.warning(f"Unknown message type: {ws_msg.type}")
                
        except Exception as e:
            raise ProtocolError(f"Failed to parse message: {e}")
            
    async def _handle_task_assignment(self, task_assign: TaskAssign):
        """Handle task assignment from controller"""
        try:
            if self.current_task is not None:
                logger.warning(f"Received task assignment while already processing task {self.current_task.task_id}")
                return
                
            # Create Task object
            task = Task(
                task_id=task_assign.task_id,
                queue=task_assign.queue,
                timeout_ms=task_assign.timeout_ms,
                metadata=dict(task_assign.metadata) if task_assign.metadata else {},
                input_data=dict(task_assign.input) if task_assign.input else {},
                files=[{
                    'file_id': f.file_id,
                    'filename': f.filename,
                    'content_type': f.content_type,
                    'size': f.size,
                    'storage_url': f.storage_url
                } for f in task_assign.files]
            )
            
            self.current_task = task
            
            # Find matching handler
            handler = self._find_handler(task.queue)
            if not handler:
                await self._send_task_failed(
                    task.task_id, 
                    "NO_HANDLER", 
                    f"No handler found for queue: {task.queue}"
                )
                return
                
            logger.info(f"Processing task {task.task_id} from queue {task.queue}")
            
            # Process task in background
            asyncio.create_task(self._process_task(task, handler))
            
        except Exception as e:
            logger.error(f"Task assignment error: {e}")
            if task_assign.task_id:
                await self._send_task_failed(task_assign.task_id, "INTERNAL_ERROR", str(e))
                
    def _find_handler(self, queue: str) -> Optional[Callable]:
        """Find handler that matches the queue pattern"""
        for pattern, handler in self.task_handlers.items():
            if self._match_queue_pattern(pattern, queue):
                return handler
        return None
        
    def _match_queue_pattern(self, pattern: str, queue: str) -> bool:
        """Match queue against pattern (supports * and # wildcards)"""
        pattern_parts = pattern.split('.')
        queue_parts = queue.split('.')
        
        return self._match_parts(pattern_parts, queue_parts)
        
    def _match_parts(self, pattern_parts: List[str], queue_parts: List[str]) -> bool:
        """Recursively match pattern parts against queue parts"""
        if not pattern_parts and not queue_parts:
            return True
        if not pattern_parts or not queue_parts:
            return False
            
        pattern_part = pattern_parts[0]
        queue_part = queue_parts[0]
        
        if pattern_part == '#':
            # Multi-level wildcard - matches remaining parts
            return True
        elif pattern_part == '*':
            # Single-level wildcard - matches one part
            return self._match_parts(pattern_parts[1:], queue_parts[1:])
        elif pattern_part == queue_part:
            # Exact match
            return self._match_parts(pattern_parts[1:], queue_parts[1:])
        else:
            return False
            
    async def _process_task(self, task: Task, handler: Callable):
        """Process task with handler"""
        try:
            # Call task handler
            result = await handler(task)
            
            # Send completion
            await self._send_task_complete(task.task_id, result)
            logger.info(f"Task {task.task_id} completed successfully")
            
        except Exception as e:
            logger.error(f"Task {task.task_id} failed: {e}")
            await self._send_task_failed(task.task_id, "HANDLER_ERROR", str(e))
        finally:
            self.current_task = None
            
    async def _send_task_progress(self, task_id: str, progress: float, message: str = "", data: Optional[Dict] = None):
        """Send task progress update"""
        try:
            task_progress = TaskProgress()
            task_progress.task_id = task_id
            task_progress.progress = progress
            task_progress.message = message
            
            if data:
                from google.protobuf.struct_pb2 import Struct
                task_progress.data.CopyFrom(Struct())
                task_progress.data.update(data)
                
            ws_msg = WebSocketMessage()
            ws_msg.type = MessageType.MESSAGE_TYPE_TASK_PROGRESS
            ws_msg.task_progress.CopyFrom(task_progress)
            
            await self._send_message(ws_msg)
            
        except Exception as e:
            logger.error(f"Failed to send task progress: {e}")
            
    async def _send_task_complete(self, task_id: str, result: Dict[str, Any], result_files: Optional[List[Dict]] = None):
        """Send task completion"""
        try:
            task_complete = TaskComplete()
            task_complete.task_id = task_id
            
            if result:
                from google.protobuf.struct_pb2 import Struct
                task_complete.result.CopyFrom(Struct())
                task_complete.result.update(result)
                
            if result_files:
                for file_info in result_files:
                    file_proto = task_complete.result_files.add()
                    file_proto.file_id = file_info.get('file_id', '')
                    file_proto.filename = file_info.get('filename', '')
                    file_proto.content_type = file_info.get('content_type', '')
                    file_proto.size = file_info.get('size', 0)
                    file_proto.storage_url = file_info.get('storage_url', '')
                    
            ws_msg = WebSocketMessage()
            ws_msg.type = MessageType.MESSAGE_TYPE_TASK_COMPLETE
            ws_msg.task_complete.CopyFrom(task_complete)
            
            await self._send_message(ws_msg)
            
        except Exception as e:
            logger.error(f"Failed to send task completion: {e}")
            
    async def _send_task_failed(self, task_id: str, error_code: str, error_message: str, error_details: Optional[Dict] = None):
        """Send task failure"""
        try:
            task_failed = TaskFailed()
            task_failed.task_id = task_id
            task_failed.error_code = error_code
            task_failed.error_message = error_message
            
            if error_details:
                from google.protobuf.struct_pb2 import Struct
                task_failed.error_details.CopyFrom(Struct())
                task_failed.error_details.update(error_details)
                
            ws_msg = WebSocketMessage()
            ws_msg.type = MessageType.MESSAGE_TYPE_TASK_FAILED
            ws_msg.task_failed.CopyFrom(task_failed)
            
            await self._send_message(ws_msg)
            
        except Exception as e:
            logger.error(f"Failed to send task failure: {e}")
            
    async def _send_message(self, message: WebSocketMessage):
        """Send protobuf message over WebSocket"""
        if not self.websocket or not self.connected:
            raise ConnectionError("Not connected to controller")
            
        try:
            data = message.SerializeToString()
            await self.websocket.send(data)
        except Exception as e:
            raise ConnectionError(f"Failed to send message: {e}")
            
    async def send_task_progress(self, progress: float, message: str = "", data: Optional[Dict] = None):
        """
        Send progress update for current task
        
        Args:
            progress: Progress percentage (0.0 to 100.0)
            message: Optional progress message
            data: Optional progress data
        """
        if not self.current_task:
            raise TaskError("No current task to update progress for")
            
        await self._send_task_progress(self.current_task.task_id, progress, message, data)