---
sidebar_position: 8
title: Client SDK Bindings
description: Complete SDK documentation and library code examples for Go and JavaScript/Node.js.
---

# Client SDK Bindings 📦

Sandforge provides native library bindings in Go and JavaScript to make it easy to incorporate hypervisor sandboxes directly into autonomous agent frameworks.

---

## 🐹 1. Go SDK

The Go SDK provides two interfaces:
- **`pkg/client.Client`** — HTTP client for talking to a running Sandforge control plane (recommended for most users)
- **`pkg/api`** — Low-level types for sandbox specifications and execution requests

### Installation
```bash
go get github.com/yanurag-dev/sandforge@latest
```

### HTTP Client Example (Recommended)

This example connects to a running Sandforge control plane, creates a sandbox, executes a command, and retrieves the result.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yanurag-dev/sandforge/pkg/api"
	"github.com/yanurag-dev/sandforge/pkg/client"
)

func main() {
	// Connect to the control plane (assumes 'sandforge server' is running)
	c := client.NewClient("http://localhost:8080")

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Create a sandbox
	spec := api.SandboxSpec{
		CPU:         2,
		MemoryMb:    2048,
		NetworkMode: "offline",
		Mounts: []api.WorkspaceMount{
			{
				HostPath:  "/Users/anurag/Developer/app",
				GuestPath: "/workspace",
				ReadOnly:  true,
			},
		},
	}

	sb, err := c.CreateSandbox(ctx, spec)
	if err != nil {
		log.Fatalf("Failed to create sandbox: %v", err)
	}
	defer func() {
		// Clean up: destroy the sandbox when done
		if err := c.Destroy(context.Background(), sb.ID); err != nil {
			log.Printf("Error destroying sandbox: %v", err)
		}
	}()

	fmt.Printf("Created sandbox: %s\n", sb.ID)

	// 2. Check sandbox status
	state, err := c.GetStatus(ctx, sb.ID)
	if err != nil {
		log.Fatalf("Failed to get status: %v", err)
	}
	fmt.Printf("Sandbox state: %s\n", state)

	// 3. Execute a command inside the sandbox
	execReq := api.ExecRequest{
		Command: []string{"sh", "-c", "cd /workspace && go test ./..."},
	}

	result, err := c.Exec(ctx, sb.ID, execReq)
	if err != nil {
		log.Fatalf("Execution failed: %v", err)
	}

	// 4. Output results
	fmt.Printf("Exit Code: %d\n", result.ExitCode)
	fmt.Printf("Stdout:\n%s\n", result.Stdout)
	if len(result.Stderr) > 0 {
		fmt.Printf("Stderr:\n%s\n", result.Stderr)
	}
}
```

### Client API Reference

#### `NewClient(baseURL string) *Client`
Creates a new client pointed at a Sandforge control plane.

```go
c := client.NewClient("http://localhost:8080")
```

#### `CreateSandbox(ctx context.Context, spec api.SandboxSpec) (*Sandbox, error)`
Provisions a new sandbox and returns a handle. The SDK generates a unique ID automatically.

#### `Exec(ctx context.Context, id string, req api.ExecRequest) (*api.ExecResult, error)`
Runs a command inside a sandbox. Returns exit code, stdout, stderr, and any artifacts.

#### `GetStatus(ctx context.Context, id string) (string, error)`
Returns the current lifecycle state of a sandbox (`provisioning`, `ready`, `executing`, `destroyed`, etc.).

#### `Destroy(ctx context.Context, id string) error`
Tears down the sandbox and reclaims system resources.

### Sandbox Specification

The `api.SandboxSpec` type configures resources, networking, and mounts:

```go
type SandboxSpec struct {
	CPU         int                 // Number of vCPUs (e.g., 2)
	MemoryMb    int                 // RAM in megabytes (e.g., 2048)
	NetworkMode string              // "offline" or "fetch" (default: "offline")
	Mounts      []WorkspaceMount    // Host directories to share with the guest
}

type WorkspaceMount struct {
	HostPath  string // Path on the host machine
	GuestPath string // Mount point inside the sandbox
	ReadOnly  bool   // If true, mount is read-only to prevent escape
}
```

### Execution Requests

```go
type ExecRequest struct {
	Command    []string          // e.g., []string{"go", "test", "./..."}
	Env        map[string]string // Environment variables (optional)
	TimeoutSec int               // Execution timeout in seconds (optional)
}

type ExecResult struct {
	ExitCode  int    // Process exit code
	Stdout    string // Standard output
	Stderr    string // Standard error
	Artifacts []string // Paths to files copied out (future)
}
```

---

## 🟨 2. JavaScript / Node.js SDK

The JavaScript SDK is built for modern asynchronous Node.js platforms, enabling easy execution from LangChain, AutoGPT, or custom typescript frameworks.

### Installation
```bash
npm install @sandforge/sdk
```

### Complete Code Example
This example creates a sandbox using async/await syntax, injects environment secrets, and intercepts potential runtime errors.

```javascript
import { Sandforge } from '@sandforge/sdk';

async function main() {
  // 1. Initialize client and request hypervisor instance
  const sandbox = await Sandforge.create({
    cpu: 2,
    memoryMB: 2048,
    network: 'offline', // strict offline sandbox
    mounts: [
      {
        hostPath: '/Users/anurag/code',
        guestPath: '/workspace',
        readOnly: false
      }
    ],
    env: {
      NODE_ENV: 'sandbox'
    }
  });

  try {
    console.log(`Sandbox launched successfully with ID: ${sandbox.id}`);

    // 2. Safely run untrusted agent code
    const result = await sandbox.run('cd /workspace && npm install --dry-run');

    console.log(`Execution exited with code: ${result.exitCode}`);
    console.log(`Stdout: ${result.stdout}`);

    if (result.stderr) {
      console.warn(`Stderr: ${result.stderr}`);
    }

  } catch (error) {
    console.error('An error occurred during sandbox operation:', error);
  } finally {
    // 3. Terminate guest microVM and clean up resources
    await sandbox.close();
    console.log('Sandbox resources safely reclaimed.');
  }
}

main();
```
