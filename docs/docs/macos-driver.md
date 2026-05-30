---
sidebar_position: 4
title: macOS Virtualization Driver
---

# macOS Virtualization Driver 🍏

On macOS, Sandforge integrates natively with Apple's **Virtualization Framework** (`Virtualization.framework`, introduced in macOS 11/12) using the Go **`github.com/Code-Hex/vz/v3`** bindings. This creates incredibly lightweight, hardware-accelerated Linux virtual machines directly on Apple Silicon or Intel chips without requiring QEMU, VirtualBox, or heavy third-party hypervisors.

The entire macOS driver implementation resides in `internal/backend/vz/vz.go`.

---

## 🏗️ VM Configuration & Device Mapping

The driver configures the hypervisor at the low-level API, explicitly attaching essential virtualized hardware:

### 1. The Bootloader
We boot our minimal guest kernel and Alpine RAM disk using the native Linux bootloader configuration:

```go
bootLoader, err := vz.NewLinuxBootLoader(
    kernelPath,
    vz.WithCommandLine("console=hvc0 root=/dev/ram0"),
    vz.WithInitrd(initrdPath),
)
```
*   `console=hvc0`: Routes kernel console output to a Hypervisor Virtual Console.
*   `root=/dev/ram0`: Tells the kernel to mount the RAM disk loaded via `initrd` as its root filesystem.

### 2. Serial Console Routing
To enable interactive kernel tracing, the VM's serial console port is mapped directly to standard host pipes:

```go
attachment, err := vz.NewFileHandleSerialPortAttachment(os.Stdin, os.Stdout)
serial, err := vz.NewVirtioConsoleDeviceSerialPortConfiguration(attachment)
```

### 3. Memory Ballooning & Entropy
*   **Memory Ballooning:** Configures traditional memory balloons (`vz.NewVirtioTraditionalMemoryBalloonDeviceConfiguration()`) so the host hypervisor can dynamically reclaim unused memory from the guest VM.
*   **Virtio Entropy:** Maps `/dev/urandom` from the macOS host to `/dev/random` in the guest VM via a hardware entropy generator (`vz.NewVirtioEntropyDeviceConfiguration()`). This ensures the sandboxed guest never runs out of cryptographic entropy.

### 4. Shared Directories (Virtio-fs)
Host directories (like our workspace root) are exposed to the VM using high-performance **Virtio-fs** filesystem sharing:

```go
dir, err := vz.NewSharedDirectory(m.HostPath, m.ReadOnly)
share, err := vz.NewSingleDirectoryShare(dir)
fsCfg.SetDirectoryShare(share)
```
Inside the guest, these directories are mounted using the `mount` utility targeting the designated Virtio tag.

---

## 🔌 VSOCK Communication Protocol

Since Sandforge operates in a secure, segmented network environment, standard TCP/IP networking cannot be relied upon to run commands or transfer files. Instead, host-guest communication is multiplexed over **Virtual Sockets (VSOCK)**.

VSOCK provides a standard socket interface (similar to TCP) but operates directly over the hypervisor bus without network routing overhead or firewall exposure.

```
┌──────────────────────────────────────┐
│             MAC OS HOST              │
│  Dial Guest VM via socket.Connect()  │
└──────────────────┬───────────────────┘
                   │  VSOCK Port 2222
                   ▼
┌──────────────────────────────────────┐
│           GUEST LINUX VM             │
│  In-Guest Agent listens on Port 2222  │
└──────────────────────────────────────┘
```

### Payload Structure

All messages traveling across the VSOCK interface are encoded as newline-delimited JSON envelopes.

#### The Envelope Structure
```go
type agentEnvelope struct {
	Op      string          `json:"op"`      // "exec" or "copyout"
	Payload json.RawMessage `json:"payload"` // Payload structure corresponding to op
}
```

#### Command Execution (`op == "exec"`)
*   **Request:** Contains command tokens, working directories, custom environment variables, and execution timeouts.
    ```json
    {
      "op": "exec",
      "payload": {
        "command": ["go", "test", "./..."],
        "cwd": "/workspace",
        "env": {"GOOS": "linux"},
        "timeout_sec": 30
      }
    }
    ```
*   **Response:** Carries exit status, stdout, and stderr output.
    ```json
    {
      "exit_code": 0,
      "stdout": "PASS\nok  github.com/yanurag-dev/sandforge/internal/policy\n",
      "stderr": ""
    }
    ```

#### File Retrieval (`op == "copyout"`)
*   **Request:** Requests a specific absolute path from the guest.
    ```json
    {
      "op": "copyout",
      "payload": {
        "guest_path": "/workspace/log.txt"
      }
    }
    ```
*   **Response:** Returns base64-encoded raw bytes of the file, or an error string.
    ```json
    {
      "data": "aGVsbG8gd29ybGQ=", // Base64 for "hello world"
      "error": ""
    }
    ```
