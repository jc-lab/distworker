# DistWorker Python SDK

A Python SDK for connecting workers to the DistWorker distributed task processing system.

## Features

- **WebSocket Communication**: Real-time communication with the DistWorker controller using WebSocket + Protocol Buffers
- **HMAC Authentication**: Secure authentication using HMAC-SHA256 signatures
- **Automatic Reconnection**: Built-in reconnection logic with configurable retry policies
- **Task Progress Reporting**: Send progress updates during task execution
- **Resource Monitoring**: Report worker resource information and usage
- **Async/Await Support**: Fully asynchronous API using Python's asyncio

## Installation

```bash
pip install distworker-sdk
```

### Development Installation

```bash
git clone https://github.com/jc-lab/distworker.git
cd distworker/python
pip install -e .
```

## Quick Start

Here's a simple worker that handles mathematical operations:

```python
import asyncio
from distworker import Worker, Task

async def handle_math_task(task: Task):
    """Handle mathematical operations"""
    operation = task.get_input('operation')
    a = task.get_input('a')
    b = task.get_input('b')
    
    if operation == 'add':
        result = a + b
    elif operation == 'multiply':
        result = a * b
    else:
        raise ValueError(f"Unknown operation: {operation}")
    
    return {'result': result}

async def main():
    # Create worker
    worker = Worker(
        controller_url='ws://localhost:8080/ws',
        worker_id='math-worker-001',
        worker_token='your-worker-token',
        resource_info={'cpu_cores': 4, 'memory_mb': 8192}
    )
    
    # Register task handler
    worker.register_handler('math.*', handle_math_task)
    
    # Start worker
    await worker.start()
    
    # Keep running
    try:
        while True:
            await asyncio.sleep(1)
    except KeyboardInterrupt:
        await worker.stop()

if __name__ == '__main__':
    asyncio.run(main())
```

## Configuration

### Environment Variables

You can configure the worker using environment variables:

```bash
export DISTWORKER_CONTROLLER_URL="ws://localhost:8080/ws"
export DISTWORKER_WORKER_ID="my-worker-001"
export DISTWORKER_WORKER_TOKEN="your-secret-token"
```

### Worker Parameters

- **controller_url**: WebSocket URL of the DistWorker controller
- **worker_id**: Unique identifier for this worker instance
- **worker_token**: Secret token for authentication
- **resource_info**: Dictionary with worker resource information
- **reconnect_interval**: Seconds between reconnection attempts (default: 5.0)
- **heartbeat_interval**: Seconds between heartbeat messages (default: 30.0)
- **max_reconnect_attempts**: Maximum reconnection attempts, -1 for unlimited (default: -1)

## Task Handling

### Basic Handler

```python
async def my_task_handler(task: Task):
    # Access input data
    input_value = task.get_input('key', default_value)
    
    # Access metadata
    priority = task.get_metadata('priority', 'normal')
    
    # Access files
    for file_info in task.files:
        file_id = file_info['file_id']
        filename = file_info['filename']
        storage_url = file_info['storage_url']
    
    # Return results
    return {'status': 'completed', 'result': 'success'}
```

### Progress Reporting

```python
async def long_running_task(task: Task):
    # Send progress updates
    await worker.send_task_progress(25.0, "Processing started")
    await asyncio.sleep(2)
    
    await worker.send_task_progress(50.0, "Half way done")
    await asyncio.sleep(2)
    
    await worker.send_task_progress(90.0, "Almost finished")
    await asyncio.sleep(1)
    
    return {'status': 'completed'}
```

### Error Handling

```python
async def safe_task_handler(task: Task):
    try:
        # Task processing logic
        result = process_data(task.get_input('data'))
        return {'result': result}
    except ValueError as e:
        # Task will be marked as failed with this error
        raise e
    except Exception as e:
        # Convert to a more specific error
        raise RuntimeError(f"Processing failed: {e}")
```

## Examples

The SDK includes example workers:

### Basic Worker
```bash
python -m distworker.examples.basic_worker
```

Handles mathematical operations, text processing, and data transformations.

### File Processing Worker
```bash
python -m distworker.examples.file_worker
```

Processes files, including image resizing and document conversion.

## API Reference

### Worker Class

#### Constructor
```python
Worker(
    controller_url: str,
    worker_id: str,
    worker_token: str,
    queue_patterns: List[str],
    resource_info: Optional[Dict[str, Any]] = None,
    reconnect_interval: float = 5.0,
    heartbeat_interval: float = 30.0,
    max_reconnect_attempts: int = -1
)
```

#### Methods
- `register_handler(pattern: str, handler: Callable)` - Register a task handler
- `start()` - Start the worker and connect to controller
- `stop()` - Stop the worker and disconnect
- `send_task_progress(progress: float, message: str, data: Dict)` - Send progress update

### Task Class

#### Properties
- `task_id: str` - Unique task identifier
- `queue: str` - Queue name where task was submitted
- `timeout_ms: int` - Task timeout in milliseconds
- `metadata: Dict[str, Any]` - Task metadata
- `input_data: Dict[str, Any]` - Task input data
- `files: List[Dict[str, Any]]` - List of file information

#### Methods
- `get_input(key: str, default: Any = None)` - Get input value by key
- `get_metadata(key: str, default: Any = None)` - Get metadata value by key
- `get_file_by_id(file_id: str)` - Get file info by ID
- `get_files_by_name(filename: str)` - Get files by filename

## Development

### Running Tests
```bash
pip install -e ".[dev]"
pytest
```

### Code Formatting
```bash
black distworker/
flake8 distworker/
```

### Type Checking
```bash
mypy distworker/
```

## License

Apache-2.0 License - see [LICENSE](./LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run the test suite
6. Submit a pull request

## Support

- GitHub Issues: https://github.com/jc-lab/distworker/issues
- Documentation: https://github.com/jc-lab/distworker/blob/main/README.md