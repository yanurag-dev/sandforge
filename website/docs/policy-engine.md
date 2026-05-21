---
sidebar_position: 3
title: The Policy Engine
---

# The Policy Engine 🛡️

The **Policy Engine** is Sandforge's gatekeeper. It acts as a static validation layer on the host OS, intercepting requests *before* any CPU cycles or storage are allocated for a virtual machine. If an agent's request violates security constraints, it is rejected instantly.

---

## 📂 Filesystem Mount Policies

Filesystem mounting is one of the highest risk areas in sandbox orchestration. If a container or hypervisor mounts an overly broad host directory, a malicious script can read or overwrite critical files.

Sandforge solves this by enforcing three levels of verification inside `internal/policy/engine.go`:

### 1. Absolute Path Enforcement
All host paths requested for mounting must be fully qualified absolute paths. Relative paths (e.g. `../etc`) are rejected immediately to prevent directory traversal attempts.

### 2. Symlink Resolution (Preventing Traversal Escapes)
If a user creates a symbolic link inside an allowed workspace pointing to `/Users/username/.ssh`, standard path checking might allow it because the symlink itself resides in an allowed folder. 

Sandforge prevents this exploit by calling `filepath.EvalSymlinks(path)` *prior* to evaluation. The engine resolves the actual physical target on the host filesystem:

```go
// From internal/policy/engine.go
resolved, err := filepath.EvalSymlinks(path)
if err != nil {
    // If the target file doesn't exist yet, resolve its parent directory
    dir := filepath.Dir(path)
    resolvedDir, errDir := filepath.EvalSymlinks(dir)
    if errDir != nil {
        return errDir
    }
    path = filepath.Join(resolvedDir, filepath.Base(path))
} else {
    path = resolved
}
```

### 3. Precise Segment Whitelisting and Blocklisting
*   **Whitelisting:** The resolved path must start with one of the configured `AllowedHostPrefixes` (e.g., your designated workspace root).
*   **Segment Blocklisting:** The path is tokenized by the OS filepath separator. If any individual segment matches blocked keywords (such as `.ssh`, `secrets`, `.aws`, or `credentials`), the mount is denied:

```go
segments := strings.Split(path, string(filepath.Separator))
for _, pattern := range e.BlockedHostPatterns {
    for _, segment := range segments {
        if segment == pattern {
            return ErrForbiddenHostPath
        }
    }
}
```

---

## 🌐 Network Isolation Modes

Sandforge supports granular egress policies to prevent data exfiltration, secret leakage, or downloading malicious payloads:

| Mode | Egress Access | Typical Use Case |
| :--- | :--- | :--- |
| **`offline`** | **Blocked completely** | Default task execution, testing, and file editing. |
| **`fetch`** | **Restricted egress** | Installing project dependencies (`npm install`, `pip install`) or fetching from remote Git repositories. |
| **`full`** | **Standard outbound** | Explicitly authorized tasks requiring broad network capabilities. |

---

## 🎛️ Resource and Execution Limits

The Policy Engine checks hardware allocations to prevent denial-of-service (DoS) attacks on the host machine:

*   **vCPUs:** Evaluates sandbox requests to ensure they do not exceed `MaxCPU` boundaries.
*   **Memory:** Blocks VM boots that request more than `MaxMemoryMb`.
*   **Disk Allocations:** Enforces the `MaxDiskGb` quota.
*   **Command Whitelisting:** When executing shell commands inside the VM, the Policy Engine inspects the primary binary. Only explicitly allowed commands (e.g., `go`, `node`, `python3`, `npm`, `ls`, `cat`) are permitted; administrative or high-risk binaries (like `sudo` or raw `iptables`) are blocked at the host level.
