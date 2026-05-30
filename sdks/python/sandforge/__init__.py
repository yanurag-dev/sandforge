"""Sandforge Python SDK.

A client library for interacting with the Sandforge hypervisor sandbox platform.

Example:
    from sandforge import Client, SandboxSpec

    # Create a client
    client = Client("http://localhost:8080")

    # Create a sandbox
    sandbox = client.create_sandbox(SandboxSpec(cpu=2, memory_mb=512))

    # Run a command
    result = sandbox.commands.run(["echo", "Hello, Sandforge!"])
    print(result.stdout)

    # Get sandbox info
    info = sandbox.info()
    print(f"Sandbox {info.id} is {info.state}")

    # Clean up
    sandbox.kill()
"""

from .client import Client, SandboxHandle, CommandsAPI, FilesAPI, GitAPI

# Alias for cleaner ergonomics
Sandbox = SandboxHandle
from .types import (
    SandboxSpec,
    WorkspaceMount,
    ExecRequest,
    ExecResult,
    SandboxInfo,
    EntryInfo,
    GitStatus,
    SandforgeException,
    SandboxNotFoundError,
    ExecutionError,
    NetworkError,
    InvalidSpecError,
)

__version__ = "0.1.1"

__all__ = [
    "Client",
    "Sandbox",
    "SandboxHandle",
    "CommandsAPI",
    "FilesAPI",
    "GitAPI",
    "SandboxSpec",
    "WorkspaceMount",
    "ExecRequest",
    "ExecResult",
    "SandboxInfo",
    "EntryInfo",
    "GitStatus",
    "SandforgeException",
    "SandboxNotFoundError",
    "ExecutionError",
    "NetworkError",
    "InvalidSpecError",
]
