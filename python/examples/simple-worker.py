"""
Basic DistWorker Example

This example demonstrates how to create a simple worker that can process
different types of tasks from multiple queues.
"""

import asyncio
import logging
import os
from typing import Dict, Any

from distworker import Worker, Task

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

worker = Worker(
    controller_url=os.getenv('DISTWORKER_CONTROLLER_URL', 'http://localhost:9802/'),
    provisioner=os.getenv('DISTWORKER_PROVISIONER', 'example'),
    worker_id=os.getenv('DISTWORKER_WORKER_ID', ''),
    worker_token=os.getenv('DISTWORKER_WORKER_TOKEN', 'demo-token-123'),
    reconnect_interval=5.0,
    heartbeat_interval=5.0
)

async def task_handler(task: Task) -> Dict[str, Any]:
    print("task_handler: ", task)
    output_message = task.get_input("message") * 10
    #
    # await worker.send_task_progress(30.0, "processing")
    #
    # # Simulate some processing time
    # await asyncio.sleep(1)
    #
    # await worker.send_task_progress(90.0, "finishing")
    #
    # # Simulate some processing time
    # await asyncio.sleep(1)

    return {
        "message": output_message,
    }

async def main():
    worker.task_handler = task_handler
    await worker.run()

if __name__ == '__main__':
    asyncio.run(main())