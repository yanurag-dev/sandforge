# Sandforge Implementation Roadmap

This roadmap tracks the progress of the Sandforge Agent Sandbox based on [ARCHITECTURE.md](ARCHITECTURE.md).

## Phase 1: Foundation & Policy (Security First)
*Goal: Establish the core interfaces and the "Deny by Default" security layer.*

- [x] **1.1 Project Scaffolding**: Go workspace, directory structure, and `go.mod`.
- [x] **1.2 Core API Contracts**: Define `SandboxSpec`, `ExecRequest`, and `SandboxBackend` interfaces.
- [x] **1.3 Policy Engine**:
    - [x] Filesystem path validation (whitelist logic) [#1](https://github.com/yanurag-dev/sandforge/issues/1).
    - [x] Network mode enforcement (Offline/Fetch/Full) [#2](https://github.com/yanurag-dev/sandforge/issues/2).
    - [x] Resource limit validation (CPU/Memory/Disk) [#2](https://github.com/yanurag-dev/sandforge/issues/2).
    - [x] Command family filtering [#3](https://github.com/yanurag-dev/sandforge/issues/3).
- [x] **1.4 Testing**: Unit tests for policy enforcement.

## Phase 2: Orchestration & Mocking
*Goal: Build the state machine that manages sandbox lifecycles.*

- [ ] **2.1 Sandbox Supervisor**:
    - [ ] Implementation of the Lifecycle State Machine (Requested -> Provisioning -> Ready -> ...).
    - [ ] Concurrent session management.
- [ ] **2.2 Mock Backend Driver**:
    - [ ] An in-memory/process-based driver for testing the supervisor without a VM.
- [ ] **2.3 Artifact Manager**: Basic logic to handle "CopyOut" for logs and files.

## Phase 3: macOS Execution Plane (macos-vz)
*Goal: Boot a real Linux VM on macOS using the Apple Virtualization Framework.*

- [ ] **3.1 Worker Image Preparation**: Create a minimal Linux kernel + initrd/disk image.
- [ ] **3.2 VZ Driver Implementation**:
    - [ ] VM configuration (vCPU, Memory).
    - [ ] Virtio-fs or Virtio-9p for workspace mounting.
    - [ ] Virtio-serial or VSOCK for command transport.
- [ ] **3.3 Networking**: Implement `offline` and `fetch` (NAT) modes using VZ.

## Phase 4: Linux Execution Plane (linux-kvm)
*Goal: Parity for Linux hosts.*

- [ ] **4.1 KVM/QEMU Driver**:
    - [ ] Implementation of the `SandboxBackend` using KVM.
    - [ ] Shared filesystem setup (Virtio-fs).
- [ ] **4.2 (Optional) Firecracker**: MicroVM support for ultra-fast boot.

## Phase 5: Task Runtime (Inside the Worker)
*Goal: The boundary between the VM and the Agent's code.*

- [ ] **5.1 Rootless Container Setup**: Pre-installing and configuring a container runtime (e.g., Podman/Docker) in the worker image.
- [ ] **5.2 Task Runner Agent**: A small Go binary inside the VM that receives commands via VSOCK and runs them in a container.
- [ ] **5.3 Cleanup Logic**: Ensuring the task container is destroyed immediately after execution.

## Phase 6: Control Plane & Adapters
*Goal: The external interface for Coding Agents.*

- [ ] **6.1 Control Plane API**: REST/gRPC server to manage tasks and sessions.
- [ ] **6.2 Agent Adapters**:
    - [ ] Generic Tool-Calling Adapter.
    - [ ] (Optional) Specific adapters for Claude/Codex.
- [ ] **6.3 Secret Manager**: Injection of scoped secrets into the task environment.

## Phase 7: CLI & Experience
*Goal: Making it usable.*

- [ ] **7.1 Sandforge CLI**: Commands like `sandforge run --dir . "npm test"`.
- [ ] **7.2 Logging & Streaming**: Real-time stdout/stderr streaming from the sandbox to the terminal.
- [ ] **7.3 Audit Logs**: Persisting execution history for review.

---
## Progress Legend
- [ ] To Do
- [x] Done
