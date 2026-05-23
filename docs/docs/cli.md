---
sidebar_position: 6
title: CLI Reference
description: Complete CLI reference for the sandforge command-line interface tool.
---

# CLI Reference 💻

The `sandforge` command-line utility provides terminal commands to start hypervisor daemons, execute sandboxed tasks, and manage host security policies.

---

## 🛠️ CLI Global Options

```text
Usage: sandforge [command] [flags]

Commands:
  server       Run the API control plane server
  run          Run a transient command in an isolated sandbox

Global Options:
  -h, --help   Display system help instructions
```

---

## 🚀 1. `sandforge run`

Launches a virtual guest VM, executes the specified command, streams stdout/stderr outputs, and terminates the guest.

```bash
sandforge run [options] "command"
```

### Flags
| Flag | Description | Default |
| :--- | :--- | :--- |
| `--cpu <n>` | Number of virtual CPU cores | `2` |
| `--mem <mb>` | Memory size in MB | `2048` |
| `--network <mode>` | Network mode: `offline` or `fetch` | `offline` |
| `--dir <path>` | Host workspace directory to mount | `.` |
| `--timeout <secs>` | Maximum execution timeout in seconds | `300` |
| `--mock` | Use in-memory Mock backend instead of real hypervisor | `false` |

### Examples

#### Run offline Python script inside local directory
```bash
./sandforge run --cpu 2 --mem 2048 --dir . python3 main.py
```

#### Run with network access for package fetching
```bash
./sandforge run --network fetch --timeout 600 go build ./...
```

#### Test with mock backend (no real VM)
```bash
./sandforge run --mock bash -c "echo Hello from sandbox"
```

---

## 🛰️ 2. `sandforge server`

Launches the background Control Plane daemon, serving the REST HTTP API and managing the active hypervisor instances.

```bash
sandforge server [flags]
```

### Flags
* `--addr <address>`: TCP address for the API server to listen on (`default: :8080`).

### Example
```bash
./sandforge server --addr :8080
```

