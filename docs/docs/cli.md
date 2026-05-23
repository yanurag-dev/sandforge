---
sidebar_position: 6
title: CLI Reference
description: Complete CLI reference for the sandforge command-line interface tool.
---

# CLI Reference 💻

The `sandforge` command-line utility provides terminal commands to start secure hypervisor control servers and run transient sandboxed tasks.

---

## 🛠️ CLI Global Options

```text
Usage:
  sandforge [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  run         Run a transient command in an isolated sandbox
  server      Run the API control plane server

Flags:
  -h, --help   help for sandforge
```

---

## 🚀 1. `sandforge run`

Launches an isolated virtual guest VM, executes the specified command, streams stdout/stderr outputs, and terminates the guest environment upon exit.

```bash
sandforge run [flags] <cmd> [args...]
```

### Flags
| Long Flag | Short Flag | Description | Default |
| :--- | :--- | :--- | :--- |
| `--cpu <n>` | `-c` | Number of virtual CPU cores | `2` |
| `--mem <mb>` | `-m` | Memory size in Megabytes | `2048` |
| `--dir <path>` | `-d` | Host workspace directory to mount into guest | `.` |
| `--network <mode>`| `-n` | Network mode: `offline` or `fetch` | `offline` |
| `--timeout <secs>`| `-t` | Maximum execution timeout in seconds | `300` |
| `--mock` | *(None)* | Use in-memory Mock backend instead of real hypervisor | `false` |
| `--help` | `-h` | Display detailed subcommand help | |

### Examples

#### Run offline Python script inside local directory (using short flags)
```bash
./sandforge run -c 2 -m 2048 -d . python3 main.py
```

#### Run with network access and custom timeout (using POSIX flags)
```bash
./sandforge run --network fetch --timeout 600 go build ./...
```

#### Test using the in-memory Mock backend
```bash
./sandforge run --mock -- echo "Hello from sandbox"
```

---

## 🛰️ 2. `sandforge server`

Launches the background Control Plane daemon, serving the REST HTTP API and managing the active hypervisor instances.

```bash
sandforge server [flags]
```

### Flags
| Long Flag | Short Flag | Description | Default |
| :--- | :--- | :--- | :--- |
| `--addr <address>`| `-a` | TCP address for the API server to listen on | `:8080` |
| `--help` | `-h` | Display detailed subcommand help | |

### Example
```bash
./sandforge server -a :8080
```
