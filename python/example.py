"""
Basic DistWorker Example

This example demonstrates how to create a simple worker that can process
different types of tasks from multiple queues.
"""

import asyncio
import logging
import os
import time
from typing import Dict, Any

from distworker import Worker, Task

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


class BasicWorker:
    """Basic worker implementation with example task handlers"""
    
    def __init__(self):
        # Worker configuration
        self.controller_url = os.getenv('DISTWORKER_CONTROLLER_URL', 'ws://localhost:8080/ws')
        self.provisioner_name = os.getenv('DISTWORKER_PROVISIONER_NAME', 'example')
        self.worker_id = os.getenv('DISTWORKER_WORKER_ID', '')
        self.worker_token = os.getenv('DISTWORKER_WORKER_TOKEN', 'demo-token-123')

        # Resource information
        self.resource_info = {
            'cpu_cores': 4,
            'memory_mb': 8192,
            'worker_type': 'basic',
            'version': '1.0.0'
        }
        
        # Create worker instance
        self.worker = Worker(
            controller_url=self.controller_url,
            provisioner_name=self.provisioner_name,
            worker_id=self.worker_id,
            worker_token=self.worker_token,
            resource_info=self.resource_info,
            reconnect_interval=5.0,
            heartbeat_interval=5.0
        )
        
        # Register task handlers
        self._register_handlers()
        
    def _register_handlers(self):
        """Register task handlers for different queue patterns"""
        self.worker.register_handler('math.*', self.handle_math_task)
        self.worker.register_handler('text.process', self.handle_text_task)
        self.worker.register_handler('data.transform.*', self.handle_data_task)
        self.worker.register_handler('test.*', self.handle_test_task)
        
    async def handle_math_task(self, task: Task) -> Dict[str, Any]:
        """
        Handle mathematical operations
        
        Expected input:
        {
            "operation": "add|subtract|multiply|divide",
            "a": <number>,
            "b": <number>
        }
        """
        logger.info(f"Processing math task: {task.task_id}")
        
        # Extract input parameters
        operation = task.get_input('operation')
        a = task.get_input('a')
        b = task.get_input('b')
        
        if not all([operation, a is not None, b is not None]):
            raise ValueError("Missing required parameters: operation, a, b")
            
        # Send progress update
        await self.worker.send_task_progress(10.0, "Validating input parameters")
        
        # Perform operation
        await self.worker.send_task_progress(50.0, f"Performing {operation}")
        
        if operation == 'add':
            result = a + b
        elif operation == 'subtract':
            result = a - b
        elif operation == 'multiply':
            result = a * b
        elif operation == 'divide':
            if b == 0:
                raise ValueError("Division by zero")
            result = a / b
        else:
            raise ValueError(f"Unknown operation: {operation}")
            
        await self.worker.send_task_progress(90.0, "Finalizing result")
        
        # Simulate some processing time
        await asyncio.sleep(1)
        
        return {
            'operation': operation,
            'operands': [a, b],
            'result': result,
            'computed_at': time.time()
        }
        
    async def handle_text_task(self, task: Task) -> Dict[str, Any]:
        """
        Handle text processing operations
        
        Expected input:
        {
            "text": "<text to process>",
            "operation": "uppercase|lowercase|reverse|wordcount"
        }
        """
        logger.info(f"Processing text task: {task.task_id}")
        
        text = task.get_input('text', '')
        operation = task.get_input('operation', 'uppercase')
        
        await self.worker.send_task_progress(20.0, f"Processing text with {operation}")
        
        if operation == 'uppercase':
            result = text.upper()
        elif operation == 'lowercase':
            result = text.lower()
        elif operation == 'reverse':
            result = text[::-1]
        elif operation == 'wordcount':
            result = len(text.split())
        else:
            raise ValueError(f"Unknown text operation: {operation}")
            
        await self.worker.send_task_progress(80.0, "Formatting output")
        
        # Simulate processing time
        await asyncio.sleep(0.5)
        
        return {
            'original_text': text,
            'operation': operation,
            'result': result,
            'length': len(text),
            'processed_at': time.time()
        }
        
    async def handle_data_task(self, task: Task) -> Dict[str, Any]:
        """
        Handle data transformation tasks
        
        Expected input:
        {
            "data": [<array of numbers>],
            "transform": "sum|average|min|max|sort"
        }
        """
        logger.info(f"Processing data task: {task.task_id}")
        
        data = task.get_input('data', [])
        transform = task.get_input('transform', 'sum')
        
        if not isinstance(data, list):
            raise ValueError("Data must be a list")
            
        await self.worker.send_task_progress(30.0, f"Applying {transform} transformation")
        
        if transform == 'sum':
            result = sum(data)
        elif transform == 'average':
            result = sum(data) / len(data) if data else 0
        elif transform == 'min':
            result = min(data) if data else None
        elif transform == 'max':
            result = max(data) if data else None
        elif transform == 'sort':
            result = sorted(data)
        else:
            raise ValueError(f"Unknown transform operation: {transform}")
            
        await self.worker.send_task_progress(90.0, "Transformation complete")
        
        return {
            'original_data': data,
            'transform': transform,
            'result': result,
            'count': len(data),
            'processed_at': time.time()
        }
        
    async def handle_test_task(self, task: Task) -> Dict[str, Any]:
        """
        Handle test tasks for debugging and validation
        """
        logger.info(f"Processing test task: {task.task_id}")
        
        # Simulate various stages of processing
        stages = [
            (10.0, "Initializing test"),
            (30.0, "Loading test data"),
            (50.0, "Running test logic"),
            (70.0, "Validating results"),
            (90.0, "Finalizing test")
        ]
        
        for progress, message in stages:
            await self.worker.send_task_progress(progress, message)
            await asyncio.sleep(0.5)
            
        # Echo back all input data and metadata
        return {
            'task_id': task.task_id,
            'queue': task.queue,
            'input_data': task.input_data,
            'metadata': task.metadata,
            'files': task.files,
            'test_completed': True,
            'test_timestamp': time.time()
        }
        
    async def run(self):
        """Run the worker"""
        try:
            logger.info(f"Starting basic worker {self.worker_id}")
            logger.info(f"Connecting to: {self.controller_url}")

            await self.worker.start()
            
            # Keep running until interrupted
            while True:
                await asyncio.sleep(1)
                
        except KeyboardInterrupt:
            logger.info("Received shutdown signal")
        except Exception as e:
            logger.error(f"Worker error: {e}")
        finally:
            logger.info("Shutting down worker")
            await self.worker.stop()


async def main():
    """Main function"""
    worker = BasicWorker()
    await worker.run()


if __name__ == '__main__':
    asyncio.run(main())