---
sidebar_position: 8
title: Client SDK Bindings
description: Complete SDK documentation and library code examples for Go and JavaScript/Node.js.
---

# Client SDK Bindings 📦

Sandforge provides native library bindings in Go and JavaScript to make it easy to incorporate hypervisor sandboxes directly into autonomous agent frameworks.

---

## 🐹 1. Go SDK

The Go SDK (`pkg/api`) provides interface-driven abstractions to spawn, query, and run commands in microVM sandboxes on the host machine.

### Installation
```bash
go get github.com/yanurag-dev/sandforge@latest
```

### Complete Code Example
This example initializes an isolated sandbox, mounts the local workspace directory read-only, executes a terminal command, and cleans up resources.

```go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yanurag-dev/sandforge/pkg/api"
)

func main() {
	// Create context with timeout for guest setup
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Configure sandbox
	cfg := &api.Config{
		CPU:      2,
		MemoryMB: 2048,
		Network:  api.NetworkModeOffline, // Drop external sockets
		Mounts: []api.Mount{
			{
				HostPath:  "/Users/anurag/Developer/app",
				GuestPath: "/workspace",
				ReadOnly:  true, // Safe read-only mount
			},
		},
		Env: map[string]string{
			"GO_ENV": "sandbox-test",
		},
	}

	// 1. Provision Guest VM
	sb, err := api.NewSandbox(ctx, cfg)
	if err != nil {
		log.Fatalf("Failed to spin up sandbox microVM: %v", err)
	}
	defer func() {
		// 3. Reclaim system resources
		if err := sb.Close(); err != nil {
			log.Printf("Error reclaiming resources: %v", err)
		}
	}()

	// 2. Execute command
	res, err := sb.Run(ctx, "cd /workspace && go test ./...")
	if err != nil {
		log.Fatalf("Execution failure: %v", err)
	}

	// 4. Output results
	fmt.Printf("Exit Code: %d\n", res.ExitCode)
	fmt.Printf("Stdout:\n%s\n", res.Stdout)
	if len(res.Stderr) > 0 {
		fmt.Printf("Stderr:\n%s\n", res.Stderr)
	}
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
