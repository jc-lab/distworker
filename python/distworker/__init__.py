"""
DistWorker Python SDK

A Python SDK for connecting workers to the DistWorker distributed task processing system.
"""

__version__ = "1.0.0"
__author__ = "JC-Lab"

from .client.worker import Worker
from .client.task import Task
from .client.exceptions import (
    DistWorkerError,
    ConnectionError,
    AuthenticationError,
    TaskError,
)

__all__ = [
    "Worker",
    "Task", 
    "DistWorkerError",
    "ConnectionError",
    "AuthenticationError", 
    "TaskError",
]