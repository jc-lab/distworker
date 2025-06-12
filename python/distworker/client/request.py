from typing import Callable, Optional, Dict

from .task import Task


class Request:
    _send_progress: Callable[[float, str, Optional[Dict]], None]

    task: Task

    def __init__(self, task: Task):
        self.task = task

    def progress(self, progress: float = 0, message: str = "", data: Optional[Dict] = None):
        return self._send_progress(progress, message, data)