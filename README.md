# Sandforge 🛠️

[![Go Version](https://img.shields.io/github/go-mod/go-version/yanurag-dev/sandforge)](https://github.com/yanurag-dev/sandforge/blob/main/go.mod)
[![CI](https://github.com/yanurag-dev/sandforge/actions/workflows/ci.yml/badge.svg)](https://github.com/yanurag-dev/sandforge/actions/workflows/ci.yml)
[![License](https://img.shields.io/github/license/yanurag-dev/sandforge)](https://github.com/yanurag-dev/sandforge/blob/main/LICENSE)
[![Issues](https://img.shields.io/github/issues/yanurag-dev/sandforge)](https://github.com/yanurag-dev/sandforge/issues)
[![Pull Requests](https://img.shields.io/github/issues-pr/yanurag-dev/sandforge)](https://github.com/yanurag-dev/sandforge/pulls)
[![npm](https://img.shields.io/npm/v/sandforge-sdk)](https://www.npmjs.com/package/sandforge-sdk)
[![PyPI](https://img.shields.io/pypi/v/sandforge-sdk)](https://pypi.org/project/sandforge-sdk/)

**Sandforge** is a portable, secure, and robust sandbox architecture designed to run AI coding agents (such as Codex, Claude Code, and others) in a highly restricted, isolated environment. 

By enforcing the core principle of **"Control Plane Outside, Execution Inside"**, Sandforge protects your host machine from untrusted command execution, malicious third-party builds, and unsafe repository code.

---

## 🌟 Core Pillars

1. **Hypervisor-Level Isolation**: Strong virtual machine boundaries utilizing the **Apple Virtualization Framework** (`Virtualization.framework`) on macOS and **KVM** on Linux hosts.
2. **Per-Task Boundaries**: Rootless task containers inside the guest Linux worker to isolate individual agent commands.
3. **Deny-by-Default Policy**: Deep path validation (resolving symlinks to prevent escapes), restricted network execution modes (`offline`, `fetch`), and vCPU/RAM resource enforcement before compute begins.
4. **Clean Abstractions**: A swappable, interface-driven driver architecture (`SandboxBackend`) that enables mocking and platform parity without altering supervisor orchestration.

---

## 🏗️ Architecture Overview

```mermaid
flowchart LR
    U[User / CLI / UI]
    A[Agent Adapter]
    CP[Control Plane / HTTP Server]
    PE[Policy Engine]
    SS[Sandbox Supervisor]
    BD[Backend Driver<br/>macos-vz / linux-kvm / mock]
    WK[Isolated Linux Worker VM]
    TR[Task Runtime<br/>Rootless Container]

    U --> A
    A --> CP
    CP --> PE
    CP --> SS
    SS --> BD
    BD --> WK
    WK --> TR
```

### System Boundaries

*   **Host OS (Highly Trusted):** Runs the HTTP Control Plane Server, Policy Engine, and Sandbox Supervisor. None of the agent's code can touch these components directly.
*   **Virtual Machine (Low Trust):** Runs a minimal Linux guest standard. Receives commands via virtual socket (`VSOCK`) and interacts solely with explicitly mounted directories.

For a comprehensive architectural specification, read [ARCHITECTURE.md](ARCHITECTURE.md).

---

## 📦 Client SDKs

Sandforge ships native SDKs for TypeScript, Python, and Go.

| Language | Package | Install |
|----------|---------|---------|
| TypeScript / Node.js | [`sandforge-sdk`](https://www.npmjs.com/package/sandforge-sdk) | `npm install sandforge-sdk` |
| Python | [`sandforge-sdk`](https://pypi.org/project/sandforge-sdk/) | `pip install sandforge-sdk` |
| Go | `github.com/yanurag-dev/sandforge/sdks/go` | `go get github.com/yanurag-dev/sandforge/sdks/go@latest` |

### TypeScript Quick Start

```typescript
import { Client } from "sandforge-sdk";

const client = new Client("http://localhost:8080");
const sandbox = await client.create({ cpu: 2, memoryMb: 512 });

const result = await sandbox.commands.run({ command: ["echo", "hello"] });
console.log(result.stdout); // "hello\n"

await sandbox.files.write("/workspace/hello.txt", "hello world");
const text = await sandbox.files.read("/workspace/hello.txt");

await sandbox.git.clone("https://github.com/org/repo.git", "/workspace");
await sandbox.kill();
```

### Python Quick Start

```python
from sandforge import Client

client = Client("http://localhost:8080")
sandbox = client.create_sandbox()

result = sandbox.commands.run(["echo", "hello"])
print(result.stdout)  # "hello\n"

sandbox.files.write("/workspace/hello.txt", "hello world")
text = sandbox.files.read("/workspace/hello.txt")

sandbox.git.clone("https://github.com/org/repo.git", "/workspace")
sandbox.kill()
```

---

## 📂 Project Anatomy

```text
├── cmd/
│   └── sandforge/        # CLI Entry point & server bootstrapping
├── internal/
│   ├── adapter/          # Integrations for specific coding agents (Codex, Claude)
│   ├── backend/          # Hypervisor backends (macOS Apple VZ, Mock driver)
│   ├── controlplane/     # HTTP REST Server & API handlers (/v1/sandboxes)
│   ├── policy/           # Secure Policy Engine (Path whitelisting & limits)
│   └── supervisor/       # Thread-safe Lifecycle Supervisor & state machine
├── pkg/
│   └── api/              # Public interfaces & API structs (SandboxBackend)
├── scripts/              # Guest OS builder utilities
└── images/               # [Generated] Bootable kernels & initrd files
```

---

## 🛠️ Getting Started

### Prerequisites

*   **Go**: Version `1.25` or higher.
*   **Host OS**: 
    *   macOS (Apple Silicon recommended for native hypervisor support)
    *   Linux (with KVM virtualization support)

---

### 1. Synchronize Dependencies

Ensure you have all module definitions and bindings fetched:

```bash
go mod tidy
```

### 2. Compile and Run Unit Tests

Sandforge features an extensive suite of concurrent tests, policy validations, and sandbox simulation flows:

```bash
go test -v ./...
```

### 3. Build the CLI

Compile the binary into a local path:

```bash
make build
```

---

### 4. Build Guest OS Images (macOS hosts)

If you are developing on a macOS host and wish to construct the lightweight bootable Linux guest environment, run the automatic image packager script. 

> [!NOTE]
> This requires `curl`, `cpio`, `gzip`, and `find` (all standard on macOS with Xcode Command Line Tools). It automatically fetches a minimal Alpine rootfs and extracts the optimized `linux-virt` hypervisor kernel.

```bash
# Build the guest VM image assets
make images
```

The script will generate these two files under the `images/` directory:
*   `images/vmlinuz` (Minimal Linux hypervisor kernel)
*   `images/initrd.img` (Minimal Alpine rootfs containing stubs for guest communication)

---

## 🤝 Contributing

We welcome collaborations and ideas! 

*   Since we are in early active development, please read through [ROADMAP.md](ROADMAP.md) to see active frontiers.
*   To propose significant architectural deviations, please open an Issue first so we can align on design principles.

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
