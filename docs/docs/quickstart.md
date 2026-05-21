---
sidebar_position: 3
title: Quickstart
description: Get up and running with Sandforge microVMs and CLI command execution in 5 minutes.
---

# Quickstart ⚡

Learn how to initialize the Sandforge supervisor, spin up an isolated guest microVM, and safely execute untrusted commands in under 5 minutes.

---

## 🚀 1. Verify Prerequisites

Ensure your host machine supports hypervisor virtualization:

```bash
# On macOS - check if Virtualization is active
sysctl kern.hv_support

# On Linux - check if KVM is enabled
lsmod | grep kvm
```

---

## 🏗️ 2. Run your First Sandbox

Use the `sandforge run` command to launch an ephemeral sandbox microVM, run a command inside it, and fetch the standard output instantly.

```bash
# Launch a sandbox and print the guest OS hostname
./sandforge run "hostname && uname -a"
```

### What happened behind the scenes?
1. The **Policy Engine** inspected the command and approved it.
2. The **Supervisor** requested the configured `vz` (macOS) or `kvm` (Linux) driver to initialize.
3. The host booted a minimal Guest Linux kernel in **220ms**.
4. The host dialed the guest over Virtual Socket (`VSOCK`) port `2222`.
5. The guest executed the `hostname && uname -a` command inside a rootless container.
6. The sandbox captured stdout/stderr, streamed them to your terminal, and gracefully purged the VM.

---

## 🔒 3. Verify Isolation

Let's test if the sandbox successfully isolates execution. We will attempt to write a file to the host machine's home directory.

```bash
./sandforge run "echo 'hacked' > /host/Users/test.txt"
```

### Result:
```text
[Policy Block] Access denied: Write attempt to host mount directory '/host' violates sandbox policy rules.
exit status 1
```

The Policy Engine intercepts filesystem calls *before* they are sent to the guest, failing fast and protecting the host.

---

## 🌐 4. Manage Network Modes

By default, Sandforge sandboxes run in **offline** mode. You can explicitly configure network access for commands that need to download dependencies (e.g., `npm install`, `go get`):

```bash
# Run with FETCH network capability
./sandforge run --network=fetch "curl -I https://github.com"
```

> [!TIP]
> **Network Policies:**
> The `fetch` network mode only allows outgoing HTTPS connections on port `443` to pre-approved domains. All other ports and protocols are strictly dropped. See [Guides & Policy](./guides) for details.
