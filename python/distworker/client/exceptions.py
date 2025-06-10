"""
DistWorker Client Exceptions
"""


class DistWorkerError(Exception):
    """Base exception for all DistWorker errors"""
    pass


class ConnectionError(DistWorkerError):
    """Raised when connection to the controller fails"""
    pass


class AuthenticationError(DistWorkerError):
    """Raised when worker authentication fails"""
    pass


class TaskError(DistWorkerError):
    """Raised when task processing fails"""
    pass


class ProtocolError(DistWorkerError):
    """Raised when protocol message parsing fails"""
    pass


class ConfigurationError(DistWorkerError):
    """Raised when worker configuration is invalid"""
    pass