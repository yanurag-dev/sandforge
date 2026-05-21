---
sidebar_position: 1
slug: /
title: Welcome to Sandforge
---

# Welcome to Sandforge 🛠️

**Sandforge** is a portable, secure, and robust sandbox architecture designed to run AI coding agents (such as Codex, Claude Code, and others) in a highly restricted, isolated environment.

By enforcing the core principle of **"Control Plane Outside, Execution Inside"**, Sandforge protects your host machine from untrusted command execution, malicious third-party builds, and unsafe repository code.

---

## 🌟 Core Pillars

1. **Hypervisor-Level Isolation**: Strong virtual machine boundaries utilizing the **Apple Virtualization Framework** (`Virtualization.framework`) on macOS and **KVM** on Linux hosts.
2. **Per-Task Boundaries**: Rootless task containers inside the guest Linux worker to isolate individual agent commands.
3. **Deny-by-Default Policy**: Deep path validation (resolving symlinks to prevent escapes), restricted network execution modes (`offline`, `fetch`), and vCPU/RAM resource enforcement before compute begins.
4. **Clean Abstractions**: A swappable, interface-driven driver architecture (`SandboxBackend`) that enables mocking and platform parity without altering supervisor orchestration.

---

## 🆚 Comparison to Alternatives

When running autonomous coding agents that write and execute arbitrary terminal commands, standard container setups are often insufficient or insecure. The table below outlines how Sandforge compares to traditional approaches:

| Approach | Isolation Level | Docker Daemon Access | Target Use Case |
| :--- | :--- | :--- | :--- |
| **Sandboxes (microVMs / Sandforge)** | **Full (hypervisor-level)** | **Isolated inside VM** | **Autonomous AI Agents (Untrusted code)** |
| Container with socket mount | Partial (namespaces) | Shared host daemon (Dangerous!) | Trusted local developer tools |
| Docker-in-Docker | Partial (privileged) | Nested daemon (Heavy & complex) | CI/CD pipelines and runners |
| Host execution | None | Host daemon | Manual, local software development |

---

### The Sandbox Advantage

*   **Virtual Machine Boundary:** Instead of sharing the host machine's macOS/Linux kernel, Sandforge boots a distinct guest kernel. Any kernel panic, resource leak, or privilege escalation is entirely trapped inside the VM.
*   **VSOCK Communication Protocol:** Standard microVM controllers communicate via HTTP or SSH, exposing complex interfaces. Sandforge dials the VM using raw, hardware-level Virtual Sockets (`VSOCK`) over port `2222`, sending minimal length-prefixed JSON envelopes.
*   **Fail-Fast Host Policies:** Our host-side Policy Engine validates commands and directory mounts *before* spinning up VM resources or transmitting execution payloads, conserving CPU cycles.
