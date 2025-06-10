"""
DistWorker Python SDK - Task Class
"""

from typing import Dict, Any, List, Optional
from dataclasses import dataclass


@dataclass
class Task:
    """
    Represents a task assigned to the worker
    """
    task_id: str
    queue: str
    timeout_ms: int
    metadata: Dict[str, Any]
    input_data: Dict[str, Any] 
    files: List[Dict[str, Any]]
    
    def __post_init__(self):
        """Initialize default values"""
        if self.metadata is None:
            self.metadata = {}
        if self.input_data is None:
            self.input_data = {}
        if self.files is None:
            self.files = []
            
    def get_file_by_id(self, file_id: str) -> Optional[Dict[str, Any]]:
        """
        Get file information by file ID
        
        Args:
            file_id: The file identifier
            
        Returns:
            File information dict or None if not found
        """
        for file_info in self.files:
            if file_info.get('file_id') == file_id:
                return file_info
        return None
        
    def get_files_by_name(self, filename: str) -> List[Dict[str, Any]]:
        """
        Get files by filename
        
        Args:
            filename: The filename to search for
            
        Returns:
            List of matching file information dicts
        """
        return [f for f in self.files if f.get('filename') == filename]
        
    def get_input(self, key: str, default: Any = None) -> Any:
        """
        Get input value by key
        
        Args:
            key: Input key
            default: Default value if key not found
            
        Returns:
            Input value or default
        """
        return self.input_data.get(key, default)
        
    def get_metadata(self, key: str, default: Any = None) -> Any:
        """
        Get metadata value by key
        
        Args:
            key: Metadata key
            default: Default value if key not found
            
        Returns:
            Metadata value or default
        """
        return self.metadata.get(key, default)
        
    @property
    def timeout_seconds(self) -> float:
        """Get timeout in seconds"""
        return self.timeout_ms / 1000.0
        
    def __str__(self) -> str:
        """String representation of the task"""
        return f"Task(id={self.task_id}, queue={self.queue}, files={len(self.files)})"
        
    def __repr__(self) -> str:
        """Detailed string representation"""
        return (f"Task(task_id='{self.task_id}', queue='{self.queue}', "
                f"timeout_ms={self.timeout_ms}, metadata={self.metadata}, "
                f"input_data={self.input_data}, files={self.files})")