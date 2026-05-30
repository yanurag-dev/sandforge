"""Type definitions for the Sandforge Python SDK."""

from dataclasses import dataclass, field, asdict
from typing import Dict, List, Optional, Any


@dataclass
class WorkspaceMount:
    """Represents a mount point for a workspace directory."""

    host_path: str
    guest_path: str
    read_only: bool = False

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        return asdict(self)


@dataclass
class SandboxSpec:
    """Specification for creating a sandbox."""

    backend: str = "macos-vz"  # "linux-kvm", "linux-firecracker", "macos-vz"
    cpu: int = 2
    memory_mb: int = 512
    disk_gb: int = 10
    timeout_sec: int = 3600
    network_mode: str = "offline"  # "offline", "fetch", "full"
    task_isolation: str = "container"  # "container", "process"
    mounts: List[WorkspaceMount] = field(default_factory=list)

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        return asdict(self)


@dataclass
class ExecRequest:
    """Request to execute a command in a sandbox."""

    command: List[str]
    cwd: str = "/"
    env: Dict[str, str] = field(default_factory=dict)
    timeout_sec: int = 60

    def to_dict(self) -> Dict[str, Any]:
        """Convert to dictionary for JSON serialization."""
        return asdict(self)


@dataclass
class ExecResult:
    """Result of executing a command in a sandbox."""

    exit_code: int
    stdout: str
    stderr: str
    artifacts: List[str] = field(default_factory=list)

    @staticmethod
    def from_dict(data: Dict[str, Any]) -> "ExecResult":
        """Create from dictionary returned by API."""
        return ExecResult(
            exit_code=data.get("exit_code", 0),
            stdout=data.get("stdout", ""),
            stderr=data.get("stderr", ""),
            artifacts=data.get("artifacts", []),
        )


@dataclass
class SandboxInfo:
    """Information about a sandbox's current state."""

    id: str
    state: str

    @staticmethod
    def from_dict(data: Dict[str, Any]) -> "SandboxInfo":
        """Create from dictionary returned by API."""
        return SandboxInfo(
            id=data.get("id", ""),
            state=data.get("state", ""),
        )


# Custom exceptions
class SandforgeException(Exception):
    """Base exception for Sandforge SDK."""

    pass


class SandboxNotFoundError(SandforgeException):
    """Raised when a sandbox is not found."""

    pass


class ExecutionError(SandforgeException):
    """Raised when command execution fails."""

    pass


class NetworkError(SandforgeException):
    """Raised when there's a network error communicating with the control plane."""

    pass


class InvalidSpecError(SandforgeException):
    """Raised when sandbox specification is invalid."""

    pass
