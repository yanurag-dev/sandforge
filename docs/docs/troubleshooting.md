---
sidebar_position: 12
title: Troubleshooting
description: Debug VSOCK dial timeouts, host file permissions, and mock driver fallbacks.
---

# Troubleshooting 🔍

This guide outlines common technical errors encountered when developing with or hosting Sandforge and explains how to resolve them.

---

## 🛰️ 1. VSOCK Dial Timeouts

### Error Symptom
```text
Failed to establish VSOCK socket handshake: dial vsock: timeout after 10000ms.
```

### Explanation
The supervisor successfully boots the guest virtual machine kernel, but the host fails to dial port `2222` inside the VM. The guest VSOCK daemon failed to start or fell into a kernel panic.

### Resolving Steps
1. **Check Kernel Architecture Compat:**
   Ensure the guest Linux kernel matches your host CPU architecture. spwaning an `x86_64` kernel on an ARM M1/M2 Mac (or vice-versa) will fail silently.
2. **Inspect VM Serial Logs:**
   Launch the sandbox in debug console mode to capture kernel panic readouts:
   ```bash
   ./sandforge run --debug "uname -a"
   ```
3. **Verify Host Port Bindings:**
   Ensure there are no competing virtual socket listeners on port `2222`.

---

## 🔒 2. Mount Path Access Denied

### Error Symptom
```text
[Policy Block] Access denied: Directory mount path '/etc' violates policy rules.
```
Or inside guest VM:
```text
permission denied: writing to /workspace/test.txt
```

### Explanation
1. **Host-Side Policy Block:** The Policy Engine blocked the folder mount *before* launching the VM because it violates the standard path scopes defined in `policy.yaml`.
2. **Guest-Side Permission Conflict:** The host folder was mounted, but the guest's rootless container container process runs as uid `1000`, which lack write permissions to that folder on your host machine.

### Resolving Steps
1. **Adjust Host Security Scopes:**
   Update your local `policy.yaml` configuration to allow the mounted path:
   ```yaml
   allowed_mounts:
     - /Users/anurag/Developer/my-app
   ```
2. **Fix Guest User Write Rights:**
   Ensure the folder is owned or writable by standard local users on the host (e.g. `chmod -R 775 /my-folder`), or mount the path with the `read_only: true` configuration if the agent does not need to edit it.

---

## 🧪 3. Falls Back to "Mock" Backend

### Error Symptom
```text
[Warn] Hypervisor engine unavailable. Falling back strictly to Mock Backend sandbox simulator.
```

### Explanation
Sandforge requires native system hardware virtualization. If CGO is disabled or hypervisor services cannot be initialized, it falls back to a simulated mock mode to prevent code compiles from crashing.

### Resolving Steps
1. **On macOS:**
   * Make sure you compiled the CLI using `CGO_ENABLED=1`.
   * Verify Xcode Command Line Tools are active (`xcode-select -p`).
2. **On Linux:**
   * Verify KVM modules are loaded (`lsmod | grep kvm`).
   * Add your user to the kvm group: `sudo usermod -aG kvm $USER`.
   * Test direct KVM access: `kvm-ok` (requires `cpu-checker` package).
