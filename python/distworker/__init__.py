"""
DistWorker Python SDK

A Python SDK for connecting workers to the DistWorker distributed task processing system.
"""

__version__ = "1.0.0"
__author__ = "JC-Lab"

from .client.exceptions import (
    DistWorkerError,
    ConnectionError,
    AuthenticationError,
    TaskError,
)
from .client.request import Request
from .client.task import Task
from .client.worker import Worker

__all__ = [
    "Worker",
    "Task",
    "Request",
    "DistWorkerError",
    "ConnectionError",
    "AuthenticationError", 
    "TaskError",
]