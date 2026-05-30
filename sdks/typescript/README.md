# sandforge-sdk

TypeScript SDK for the **Sandforge** hypervisor sandbox platform. Create isolated execution environments, run commands, and manage their lifecycle.

## Installation

```bash
npm install sandforge-sdk
```

## Quick Start

```typescript
import { Client } from "sandforge-sdk";

// Create a client pointing to the control plane
const client = new Client("http://localhost:8080");

// Create a sandbox
const sandbox = await client.create({
  cpu: 2,
  memoryMb: 512,
  networkMode: "offline",
});

console.log("Created sandbox:", sandbox.id);

// Run a command
const result = await sandbox.commands.run({
  command: ["echo", "Hello from the sandbox!"],
});

console.log("Exit code:", result.exitCode);
console.log("Stdout:", result.stdout);
console.log("Stderr:", result.stderr);

// Get sandbox info
const info = await sandbox.info();
console.log("Sandbox state:", info.state);

// Clean up
await sandbox.kill();
```

## API Reference

### Client

The main entry point for the SDK.

```typescript
const client = new Client(baseURL: string, fetchImpl?: typeof fetch)
```

#### Methods

- **`create(spec?: SandboxSpec): Promise<Sandbox>`**
  - Provisions a new sandbox from the given spec.
  - Returns a `Sandbox` instance.
  - If `spec` is omitted, defaults are used.

### Sandbox

A handle to a created sandbox instance.

#### Properties

- **`id: string`** — The sandbox identifier.
- **`commands: CommandsNamespace`** — Command execution API.
- **`files: FilesNamespace`** — File operations API.

#### Methods

- **`async info(): Promise<SandboxInfo>`**
  - Returns the current state of the sandbox.

- **`async kill(): Promise<void>`**
  - Destroys the sandbox.

### CommandsNamespace

Provides command execution within a sandbox.

#### Methods

- **`async run(request: ExecRequest): Promise<ExecResult>`**
  - Executes a command inside the sandbox.
  - Returns stdout, stderr, exit code, and any artifacts.

### FilesNamespace

Provides file I/O within a sandbox.

#### Methods

- **`async read(path: string): Promise<string>`**
  - ⚠️ Not yet implemented. Requires VSOCK copyout support.

## Types

### SandboxSpec

Configuration for a new sandbox.

```typescript
interface SandboxSpec {
  backend?: string;          // "linux-kvm", "linux-firecracker", "macos-vz"
  cpu?: number;              // CPU cores
  memoryMb?: number;         // Memory in megabytes
  diskGb?: number;           // Disk space in gigabytes
  timeoutSec?: number;       // Default timeout in seconds
  networkMode?: string;      // "offline", "fetch", "full"
  taskIsolation?: string;    // "container", "process"
  mounts?: WorkspaceMount[]; // Host-to-guest mounts
}
```

### ExecRequest

A command to execute.

```typescript
interface ExecRequest {
  command: string[];              // Command + arguments
  cwd?: string;                   // Working directory
  env?: Record<string, string>;   // Environment variables
  timeoutSec?: number;            // Command timeout in seconds
}
```

### ExecResult

Result of running a command.

```typescript
interface ExecResult {
  exitCode: number;       // Process exit code
  stdout: string;         // Standard output
  stderr: string;         // Standard error
  artifacts?: string[];   // Output file paths (optional)
}
```

### SandboxInfo

Status response from the control plane.

```typescript
interface SandboxInfo {
  id: string;      // Sandbox identifier
  state: string;   // Current state (e.g., "ready", "executing")
}
```

### SandboxError

Custom error class for API errors.

```typescript
class SandboxError extends Error {
  statusCode: number;
  message: string;
}
```

## Error Handling

The SDK throws `SandboxError` for API errors with a `statusCode` and descriptive message:

```typescript
import { Client, SandboxError } from "sandforge-sdk";

const client = new Client("http://localhost:8080");

try {
  const sandbox = await client.create();
  // ...
} catch (err) {
  if (err instanceof SandboxError) {
    console.error(`API error ${err.statusCode}:`, err.message);
  } else {
    console.error("Unexpected error:", err);
  }
}
```

## Building

```bash
npm install
npm run build
```

The compiled JavaScript and TypeScript definitions are in `dist/`.

## Development

Watch for changes and rebuild automatically:

```bash
npm run build:watch
```

## License

MIT
