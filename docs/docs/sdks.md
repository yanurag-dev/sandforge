---
sidebar_position: 8
title: Client SDK Bindings
description: TypeScript, Python, and Go SDK documentation for the Sandforge sandbox platform.
---

# Client SDK Bindings 📦

Sandforge provides native SDKs in TypeScript, Python, and Go. All three SDKs talk to the same HTTP control plane — choose the one that fits your agent framework.

| SDK | Package | Min runtime |
|-----|---------|-------------|
| TypeScript | `sandforge-sdk` (npm) | Node.js 18+ |
| Python | `sandforge-sdk` (PyPI) | Python 3.8+ |
| Go | `github.com/yanurag-dev/sandforge/sdks/go` | Go 1.21+ |

---

## 🟦 1. TypeScript SDK

### Installation

```bash
npm install sandforge-sdk
```

### Quick Start

```typescript
import { Client } from "sandforge-sdk";

const client = new Client("http://localhost:8080");

// Create a sandbox
const sandbox = await client.create({
  cpu: 2,
  memoryMb: 512,
  networkMode: "offline",
});

console.log("Sandbox:", sandbox.id);

// Run a command
const result = await sandbox.commands.run({
  command: ["echo", "Hello from Sandforge!"],
});

console.log("Exit code:", result.exitCode);
console.log("Stdout:", result.stdout);

// Get sandbox state
const info = await sandbox.info();
console.log("State:", info.state);

// Destroy
await sandbox.kill();
```

### API Reference

#### `new Client(baseURL, fetchImpl?)`

Creates a client pointing at a Sandforge control plane.

```typescript
const client = new Client("http://localhost:8080");
```

#### `client.create(spec?): Promise<Sandbox>`

Provisions a new sandbox. `spec` is optional — omit it to use server defaults.

```typescript
const sandbox = await client.create({
  cpu: 4,
  memoryMb: 2048,
  diskGb: 20,
  networkMode: "fetch",           // "offline" | "fetch" | "full"
  mounts: [
    { hostPath: "/my/project", guestPath: "/workspace", readOnly: false }
  ],
});
```

#### `sandbox.commands.run(request): Promise<ExecResult>`

Executes a command inside the sandbox.

```typescript
const result = await sandbox.commands.run({
  command: ["sh", "-c", "cd /workspace && npm test"],
  cwd: "/",
  env: { NODE_ENV: "test" },
  timeoutSec: 120,
});

console.log(result.exitCode);  // 0
console.log(result.stdout);
console.log(result.stderr);
```

#### `sandbox.files.read(path, opts?): Promise<string | Uint8Array>`

Reads a file from the sandbox. Returns text by default; pass `{ format: "bytes" }` for raw bytes.

```typescript
const text = await sandbox.files.read("/workspace/output.txt");
const raw = await sandbox.files.read("/workspace/data.bin", { format: "bytes" });
```

#### `sandbox.files.write(path, data): Promise<WriteFileResponse>`

Writes a string or `Uint8Array` to a path inside the sandbox.

```typescript
await sandbox.files.write("/workspace/hello.txt", "hello world");
await sandbox.files.write("/workspace/data.bin", new Uint8Array([1, 2, 3]));
```

#### `sandbox.files.list(path): Promise<EntryInfo[]>`

Lists directory contents.

```typescript
const entries = await sandbox.files.list("/workspace");
entries.forEach(e => console.log(e.name, e.isDir ? "DIR" : e.size));
```

#### `sandbox.files.stat(path): Promise<EntryInfo>`

Returns metadata for a single path.

```typescript
const info = await sandbox.files.stat("/workspace/hello.txt");
console.log(info.size, info.modTime);
```

#### `sandbox.files.exists(path): Promise<boolean>`

Returns `true` if the path exists, `false` otherwise (never throws).

```typescript
if (await sandbox.files.exists("/workspace/package.json")) { ... }
```

#### `sandbox.files.remove(path): Promise<ExecResult>`

Deletes a file or directory (`rm -rf`).

```typescript
await sandbox.files.remove("/workspace/node_modules");
```

#### `sandbox.git.clone(url, dest?, opts?): Promise<ExecResult>`

Clones a repository into the sandbox.

```typescript
await sandbox.git.clone("https://github.com/org/repo.git", "/workspace", { depth: 1 });
```

#### `sandbox.git.init(cwd): Promise<ExecResult>`

Initialises a new git repo at `cwd`.

#### `sandbox.git.add(paths, cwd): Promise<ExecResult>`

Stages files. `paths` may be a string or `string[]`.

#### `sandbox.git.commit(message, cwd): Promise<ExecResult>`

Creates a commit.

#### `sandbox.git.push(cwd, remote?, branch?): Promise<ExecResult>`

Pushes to a remote (default `origin HEAD`).

#### `sandbox.git.pull(cwd, remote?): Promise<ExecResult>`

Pulls from a remote.

#### `sandbox.git.status(cwd): Promise<GitStatus>`

Returns branch name, cleanliness, and raw porcelain output — single round-trip.

```typescript
const s = await sandbox.git.status("/workspace");
console.log(s.branch);  // "main"
console.log(s.clean);   // true / false
```

#### `sandbox.git.branches(cwd): Promise<string[]>`

Lists all local branches.

#### `sandbox.info(): Promise<SandboxInfo>`

Returns the current lifecycle state (`provisioning`, `ready`, `executing`, `destroyed`).

#### `sandbox.kill(): Promise<void>`

Destroys the sandbox and reclaims VM resources.

### Types

```typescript
interface SandboxSpec {
  backend?: string;           // "macos-vz" | "linux-kvm" | "linux-firecracker"
  cpu?: number;
  memoryMb?: number;
  diskGb?: number;
  timeoutSec?: number;
  networkMode?: string;       // "offline" | "fetch" | "full"
  mounts?: WorkspaceMount[];
}

interface WorkspaceMount {
  hostPath: string;
  guestPath: string;
  readOnly?: boolean;
}

interface ExecRequest {
  command: string[];
  cwd?: string;
  env?: Record<string, string>;
  timeoutSec?: number;
}

interface ExecResult {
  exitCode: number;
  stdout: string;
  stderr: string;
  artifacts?: string[];
}

interface SandboxInfo {
  id: string;
  state: string;
}

interface EntryInfo {
  name: string;
  path: string;
  size: number;
  isDir: boolean;
  modTime: string;
}

interface WriteFileResponse {
  size: number;
}

interface GitStatus {
  branch: string;
  clean: boolean;
  stdout: string;
}
```

### Error Handling

```typescript
import { Client, SandboxError } from "sandforge-sdk";

try {
  const sandbox = await client.create();
  // ...
} catch (err) {
  if (err instanceof SandboxError) {
    console.error(`API error ${err.statusCode}: ${err.message}`);
  }
}
```

---

## 🐍 2. Python SDK

### Installation

```bash
pip install sandforge-sdk
```

### Quick Start

```python
from sandforge import Client

client = Client("http://localhost:8080")

# Create a sandbox
sandbox = client.create_sandbox()

# Run a command
result = sandbox.commands.run(["echo", "Hello from Sandforge!"])
print(result.stdout)       # "Hello from Sandforge!\n"
print(result.exit_code)    # 0

# Get sandbox state
info = sandbox.info()
print(info.state)          # "ready"

# Destroy
sandbox.kill()
```

### API Reference

#### `Client(base_url, timeout=60)`

Creates a client pointing at a Sandforge control plane.

```python
client = Client("http://localhost:8080", timeout=30)
```

#### `client.create_sandbox(spec?) -> SandboxHandle`

Provisions a new sandbox. `spec` is optional.

```python
from sandforge import Client, SandboxSpec, WorkspaceMount

spec = SandboxSpec(
    cpu=4,
    memory_mb=2048,
    disk_gb=20,
    network_mode="fetch",        # "offline" | "fetch" | "full"
    mounts=[
        WorkspaceMount(
            host_path="/my/project",
            guest_path="/workspace",
            read_only=False,
        )
    ],
)

sandbox = client.create_sandbox(spec)
```

#### `sandbox.commands.run(command, cwd="/", env=None, timeout_sec=60) -> ExecResult`

Executes a command inside the sandbox.

```python
result = sandbox.commands.run(
    command=["sh", "-c", "cd /workspace && pytest"],
    cwd="/",
    env={"PYTHONUNBUFFERED": "1"},
    timeout_sec=300,
)

print(result.exit_code)
print(result.stdout)
print(result.stderr)
```

#### `sandbox.files.read(path, as_bytes=False) -> str | bytes`

Reads a file from the sandbox. Returns `str` by default; pass `as_bytes=True` for raw `bytes`.

```python
text = sandbox.files.read("/workspace/output.txt")
raw = sandbox.files.read("/workspace/data.bin", as_bytes=True)
```

#### `sandbox.files.write(path, data) -> int`

Writes a string or `bytes` to a path inside the sandbox. Returns bytes written.

```python
sandbox.files.write("/workspace/hello.txt", "hello world")
sandbox.files.write("/workspace/data.bin", b"\x01\x02\x03")
```

#### `sandbox.files.list(path) -> List[EntryInfo]`

Lists directory contents.

```python
for entry in sandbox.files.list("/workspace"):
    print(entry.name, "DIR" if entry.is_dir else entry.size)
```

#### `sandbox.files.stat(path) -> EntryInfo`

Returns metadata for a single path.

```python
info = sandbox.files.stat("/workspace/hello.txt")
print(info.size, info.mod_time)
```

#### `sandbox.files.exists(path) -> bool`

Returns `True` if the path exists, `False` otherwise (never raises).

```python
if sandbox.files.exists("/workspace/requirements.txt"):
    ...
```

#### `sandbox.files.remove(path) -> ExecResult`

Deletes a file or directory (`rm -rf`).

```python
sandbox.files.remove("/workspace/__pycache__")
```

#### `sandbox.git.clone(url, dest=".", depth=None) -> ExecResult`

Clones a repository into the sandbox.

```python
sandbox.git.clone("https://github.com/org/repo.git", "/workspace", depth=1)
```

#### `sandbox.git.init(cwd) -> ExecResult`

Initialises a new git repo at `cwd`.

#### `sandbox.git.add(paths, cwd) -> ExecResult`

Stages files. `paths` may be a string or list of strings.

#### `sandbox.git.commit(message, cwd) -> ExecResult`

Creates a commit.

#### `sandbox.git.push(cwd, remote="origin", branch="HEAD") -> ExecResult`

Pushes to a remote.

#### `sandbox.git.pull(cwd, remote="origin") -> ExecResult`

Pulls from a remote.

#### `sandbox.git.status(cwd) -> GitStatus`

Returns branch name, cleanliness, and raw porcelain output — single round-trip.

```python
s = sandbox.git.status("/workspace")
print(s.branch)  # "main"
print(s.clean)   # True / False
```

#### `sandbox.git.branches(cwd) -> List[str]`

Lists all local branches.

#### `sandbox.info() -> SandboxInfo`

Returns the current lifecycle state.

#### `sandbox.kill() -> None`

Destroys the sandbox and reclaims VM resources.

### Types

```python
from dataclasses import dataclass, field

@dataclass
class SandboxSpec:
    backend: str = "macos-vz"        # "macos-vz" | "linux-kvm" | "linux-firecracker"
    cpu: int = 2
    memory_mb: int = 512
    disk_gb: int = 10
    timeout_sec: int = 3600
    network_mode: str = "offline"    # "offline" | "fetch" | "full"
    mounts: List[WorkspaceMount] = field(default_factory=list)

@dataclass
class WorkspaceMount:
    host_path: str
    guest_path: str
    read_only: bool = False

@dataclass
class ExecResult:
    exit_code: int
    stdout: str
    stderr: str
    artifacts: List[str] = field(default_factory=list)

@dataclass
class SandboxInfo:
    id: str
    state: str

@dataclass
class EntryInfo:
    name: str
    path: str
    size: int
    is_dir: bool
    mod_time: str

@dataclass
class GitStatus:
    branch: str
    clean: bool
    stdout: str
```

### Error Handling

```python
from sandforge import Client, NetworkError, SandboxNotFoundError, SandforgeException

try:
    sandbox = client.create_sandbox()
    result = sandbox.commands.run(["false"])
    if result.exit_code != 0:
        print(f"Command failed: {result.stderr}")
    sandbox.kill()
except NetworkError as e:
    print(f"Network error: {e}")
except SandboxNotFoundError as e:
    print(f"Sandbox not found: {e}")
except SandforgeException as e:
    print(f"Sandforge error: {e}")
```

---

## 🐹 3. Go SDK

### Installation

```bash
go get github.com/yanurag-dev/sandforge/sdks/go@latest
```

### Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/yanurag-dev/sandforge/pkg/api"
    "github.com/yanurag-dev/sandforge/sdks/go"
)

func main() {
    c := client.NewClient("http://localhost:8080")

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    // Create a sandbox
    spec := api.SandboxSpec{
        CPU:         2,
        MemoryMb:    512,
        NetworkMode: "offline",
    }

    sb, err := c.CreateSandbox(ctx, spec)
    if err != nil {
        log.Fatalf("create failed: %v", err)
    }
    defer c.Destroy(context.Background(), sb.ID)

    fmt.Println("Sandbox:", sb.ID)

    // Run a command
    result, err := c.Exec(ctx, sb.ID, api.ExecRequest{
        Command: []string{"echo", "Hello from Sandforge!"},
    })
    if err != nil {
        log.Fatalf("exec failed: %v", err)
    }

    fmt.Printf("Exit code: %d\n", result.ExitCode)
    fmt.Printf("Stdout: %s\n", result.Stdout)
}
```

### API Reference

#### `NewClient(baseURL string) *Client`

Creates a new client.

#### `CreateSandbox(ctx, spec) (*Sandbox, error)`

Provisions a new sandbox. Returns a handle with the sandbox ID.

#### `Exec(ctx, id, req) (*api.ExecResult, error)`

Runs a command. Returns exit code, stdout, stderr, and artifacts.

#### `GetStatus(ctx, id) (string, error)`

Returns the current lifecycle state string.

#### `Destroy(ctx, id) error`

Tears down the sandbox.

### Types

```go
type SandboxSpec struct {
    CPU         int
    MemoryMb    int
    NetworkMode string           // "offline" | "fetch" | "full"
    Mounts      []WorkspaceMount
}

type WorkspaceMount struct {
    HostPath  string
    GuestPath string
    ReadOnly  bool
}

type ExecRequest struct {
    Command    []string
    Env        map[string]string
    TimeoutSec int
}

type ExecResult struct {
    ExitCode  int
    Stdout    string
    Stderr    string
    Artifacts []string
}
```

---

## What's not in v0

| Feature | Reason | Planned |
|---------|--------|---------|
| Streaming output | Protocol redesign needed | P3 |
