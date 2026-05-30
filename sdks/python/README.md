# Sandforge Python SDK

The Sandforge Python SDK provides a client library for interacting with the Sandforge hypervisor sandbox platform. It enables you to create, manage, and execute commands in isolated sandboxes programmatically.

## Installation

```bash
pip install sandforge-sdk
```

For development (from source):

```bash
pip install -e ".[dev]"
```

## Quick Start

### Basic Usage

```python
from sandforge import Client, SandboxSpec

# Create a client pointing to your Sandforge control plane
client = Client("http://localhost:8080")

# Create a sandbox with default configuration
sandbox = client.create_sandbox()

# Run a command
result = sandbox.commands.run(["echo", "Hello, Sandforge!"])
print(result.stdout)  # "Hello, Sandforge!\n"

# Clean up
sandbox.kill()
```

### Custom Sandbox Configuration

```python
from sandforge import Client, SandboxSpec, WorkspaceMount

spec = SandboxSpec(
    cpu=4,
    memory_mb=2048,
    disk_gb=20,
    timeout_sec=3600,
    network_mode="fetch",  # Allow package downloads
    mounts=[
        WorkspaceMount(
            host_path="/path/to/project",
            guest_path="/workspace",
            read_only=False,
        ),
    ],
)

client = Client("http://localhost:8080")
sandbox = client.create_sandbox(spec)

# Work with the mounted directory
result = sandbox.commands.run(["ls", "-la", "/workspace"])
print(result.stdout)

sandbox.kill()
```

### Command Execution with Environment Variables

```python
from sandforge import Client

client = Client("http://localhost:8080")
sandbox = client.create_sandbox()

# Run with custom environment variables
result = sandbox.commands.run(
    command=["python", "-c", "import os; print(os.environ.get('MY_VAR'))"],
    cwd="/",
    env={"MY_VAR": "Hello World"},
    timeout_sec=30,
)

print(result.stdout)   # "Hello World\n"
print(result.exit_code)  # 0

sandbox.kill()
```

### Error Handling

```python
from sandforge import Client, NetworkError, SandboxNotFoundError

client = Client("http://localhost:8080")

try:
    sandbox = client.create_sandbox()
    result = sandbox.commands.run(["false"])  # Command that fails
    
    if result.exit_code != 0:
        print(f"Command failed with exit code {result.exit_code}")
        print(f"stderr: {result.stderr}")
    
    sandbox.kill()
    
except NetworkError as e:
    print(f"Connection error: {e}")
except SandboxNotFoundError as e:
    print(f"Sandbox not found: {e}")
```

### Sandbox Information

```python
from sandforge import Client

client = Client("http://localhost:8080")
sandbox = client.create_sandbox()

# Get sandbox information
info = sandbox.info()
print(f"Sandbox ID: {info.id}")
print(f"State: {info.state}")  # "ready", "executing", "destroyed", etc.
```

## API Reference

### Client

The main entry point for interacting with Sandforge.

#### Constructor

```python
Client(base_url: str, timeout: int = 60)
```

- `base_url`: The control plane URL (e.g., "http://localhost:8080")
- `timeout`: Request timeout in seconds (default: 60)

#### Methods

##### `create_sandbox(spec: Optional[SandboxSpec] = None) -> SandboxHandle`

Create a new sandbox.

- Returns: A `SandboxHandle` to the created sandbox
- Raises: `NetworkError` or `SandforgeException`

##### `exec(sandbox_id: str, request: ExecRequest) -> ExecResult`

Execute a command in a sandbox.

- Returns: An `ExecResult` with exit code, stdout, and stderr
- Raises: `NetworkError` or `SandforgeException`

##### `get_status(sandbox_id: str) -> str`

Get the current state of a sandbox.

- Returns: The sandbox state as a string
- Raises: `NetworkError`

##### `get_info(sandbox_id: str) -> SandboxInfo`

Get detailed information about a sandbox.

- Returns: A `SandboxInfo` object
- Raises: `NetworkError`

##### `destroy(sandbox_id: str) -> None`

Destroy a sandbox.

- Raises: `NetworkError` or `SandforgeException`

### SandboxHandle

A handle to a created sandbox with convenience APIs.

#### Properties

- `id`: The sandbox ID (string)

#### Methods

##### `kill() -> None`

Destroy the sandbox.

```python
sandbox.kill()
```

##### `info() -> SandboxInfo`

Get sandbox information.

```python
info = sandbox.info()
```

#### Nested APIs

##### `commands`

The `CommandsAPI` for executing commands.

###### `run(command, cwd="/", env=None, timeout_sec=60) -> ExecResult`

Run a command in the sandbox.

- `command`: List of command and arguments
- `cwd`: Working directory (default: "/")
- `env`: Dictionary of environment variables (default: {})
- `timeout_sec`: Command timeout in seconds (default: 60)
- Returns: `ExecResult` with exit code, stdout, and stderr

```python
result = sandbox.commands.run(
    ["python", "script.py"],
    cwd="/workspace",
    env={"PYTHONUNBUFFERED": "1"},
    timeout_sec=300,
)
```

##### `files`

The `FilesAPI` for reading files from the sandbox.

###### `read(path: str) -> str`

Read a file from the sandbox.

**Note:** This method is currently not implemented and raises `NotImplementedError`. VSOCK copyout support is coming soon.

```python
try:
    content = sandbox.files.read("/etc/hostname")
except NotImplementedError:
    print("files.read() not yet supported")
```

### Types

#### SandboxSpec

Specification for creating a sandbox.

```python
SandboxSpec(
    backend: str = "macos-vz",        # "linux-kvm", "linux-firecracker", "macos-vz"
    cpu: int = 2,                      # Number of vCPUs
    memory_mb: int = 512,              # Memory in MB
    disk_gb: int = 10,                 # Disk size in GB
    timeout_sec: int = 3600,           # Sandbox lifetime in seconds
    network_mode: str = "offline",     # "offline", "fetch", "full"
    task_isolation: str = "container", # "container", "process"
    mounts: List[WorkspaceMount] = [], # Mounted directories
)
```

#### WorkspaceMount

A directory mount from host to guest.

```python
WorkspaceMount(
    host_path: str,   # Path on the host
    guest_path: str,  # Path in the sandbox
    read_only: bool = False,  # Whether the mount is read-only
)
```

#### ExecRequest

A request to execute a command.

```python
ExecRequest(
    command: List[str],              # Command and arguments
    cwd: str = "/",                  # Working directory
    env: Dict[str, str] = {},        # Environment variables
    timeout_sec: int = 60,           # Timeout in seconds
)
```

#### ExecResult

The result of command execution.

```python
ExecResult(
    exit_code: int,              # Command exit code
    stdout: str,                 # Standard output
    stderr: str,                 # Standard error
    artifacts: List[str] = [],   # Paths to generated artifacts
)
```

#### SandboxInfo

Information about a sandbox.

```python
SandboxInfo(
    id: str,     # Sandbox ID
    state: str,  # Current state (e.g., "ready", "executing", "destroyed")
)
```

### Exceptions

All exceptions inherit from `SandforgeException`.

- **SandforgeException**: Base exception for all Sandforge errors
- **NetworkError**: Network communication error with the control plane
- **SandboxNotFoundError**: Sandbox does not exist
- **ExecutionError**: Command execution failed
- **InvalidSpecError**: Invalid sandbox specification

## Error Handling

The SDK provides specific exception types for different error scenarios:

```python
from sandforge import Client, NetworkError, SandboxNotFoundError, SandforgeException

client = Client("http://localhost:8080")

try:
    sandbox = client.create_sandbox()
    result = sandbox.commands.run(["exit", "1"])
except NetworkError as e:
    print(f"Network error: {e}")
except SandboxNotFoundError as e:
    print(f"Sandbox not found: {e}")
except SandforgeException as e:
    print(f"Sandforge error: {e}")
```

## Running Tests

```bash
pip install -e ".[dev]"
pytest tests/
```

## Contributing

Contributions are welcome! Please ensure code passes linting and type checks:

```bash
black sandforge/
flake8 sandforge/
mypy sandforge/
```

## License

Apache License 2.0. See LICENSE in the repository root for details.
