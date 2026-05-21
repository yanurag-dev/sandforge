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
Usage: sandforge [command] [options]

Commands:
  run          Execute a command inside an ephemeral guest microVM
  supervisor   Launch the background supervisor REST API daemon
  policy       Validate and audit host-side security policies

Global Options:
  -h, --help      Display system help instructions
  -v, --version   Print the active CLI binary build version
```

---

## 🚀 1. `sandforge run`

Launches a virtual guest VM, executes the specified command, streams stdout/stderr outputs, and terminates the guest.

```bash
sandforge run [options] "command"
```

### Options
| Flag | Short | Description | Default |
| :--- | :--- | :--- | :--- |
| `--cpu <n>` | `-c` | Number of vCPUs allocated to the VM guest | `1` |
| `--memory <mb>` | `-m` | Memory in Megabytes allocated to the VM guest | `1024` |
| `--network <mode>`| `-n` | Network mode: `offline`, `fetch`, or `full` | `offline` |
| `--dir <path>` | `-d` | Path to mount the host working workspace directory | `.` |
| `--env <k=v>` | `-e` | Pass environment variables into the guest task | (None) |

### Examples

#### Run off-line Python script inside local directory
```bash
./sandforge run -c 2 -m 2048 -d . "python3 main.py"
```

#### Run testing suite with custom environment variables
```bash
./sandforge run -e NODE_ENV="test" -e DEBUG="* " "npm run test"
```

---

## 🛰️ 2. `sandforge supervisor`

Launches the background Control Plane daemon, serving the REST HTTP API and managing the active hypervisor instances.

```bash
sandforge supervisor [options]
```

### Options
* `--port <port>` (Short `-p`): Port to bind the supervisor HTTP API (`default: 8585`).
* `--host <ip>`: Bind address for the daemon listener (`default: 127.0.0.1`).
* `--images <path>`: Local file path to search for kernel and initrd images (`default: ~/.config/sandforge/images`).

### Example
```bash
./sandforge supervisor --port 8585 --host 127.0.0.1
```

---

## 🛡️ 3. `sandforge policy`

Inspects, validates, and dry-runs host security configurations.

```bash
sandforge policy [command] [options]
```

### Sub-commands
* `validate <file>`: Parse a YAML policy configuration file and verify syntax correctness.
* `audit <cmd>`: Check if a terminal command is approved or blocked under current policies.

### Example
```bash
./sandforge policy validate /etc/sandforge/policy.yaml
```
```text
[Success] Policy file syntax is 100% correct.
[Rules Audit] 12 filesystem paths watched, network limits: FETCH.
```
