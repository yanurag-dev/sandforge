---
sidebar_position: 5
title: Development & Building
---

# Development & Building 🛠️

This guide outlines how to sync dependencies, run tests, compile the CLI binary, and build the custom Linux guest operating system images required to spin up sandboxes locally.

---

## 📋 Prerequisites

Ensure you have the following installed on your host machine:

*   **Go Compiler:** Go Version `1.25` or higher.
*   **Operating System Requirements:**
    *   **macOS:** macOS 12+ (Apple Silicon recommended for native hypervisor acceleration).
    *   **Linux:** Kernel 4.8+ with virtual socket and KVM kernel modules active.
*   **Build Tools:** Standard utilities (`curl`, `cpio`, `gzip`, `find`, `make`, `tar`) available under shell environments. On macOS, these are bundled by default with Xcode Command Line Tools (`xcode-select --install`).

---

## 🛠️ Developer Lifecycle Commands

Sandforge includes a simple, standardized **Makefile** to coordinate dev cycles:

### 1. Synchronize Go Modules
Fetch Go dependencies and compile system-level virtualization bindings:

```bash
go mod tidy
```

### 2. Run the Test Suites
Run the entire unit and integration test suite (covering Policy Engine symlink traps, supervisor state transitions, and backend simulation flows):

```bash
go test -v ./...
```

### 3. Compile the CLI Binary
Builds the standalone host CLI binary and outputs it under the `bin/` directory:

```bash
make build
```

---

## 💾 Building the Guest OS Image

To run a real hypervisor VM sandbox on macOS, you must construct the lightweight Linux guest files (`images/vmlinuz` and `images/initrd.img`). Sandforge automates this packaging process inside `./scripts/build-images.sh`.

```bash
# Build bootable kernel and RAM disk images
make images
```

### Under the Hood: What `build-images.sh` Does

The packaging script operates through a series of automated compilation and extraction stages:

```
1. Fetch Alpine Minirootfs  ➔  2. Extract & Inject /init Script
                                          │
4. Write images/vmlinuz     ◀── 3. Fetch linux-virt kernel apk
5. Write images/initrd.img
```

1.  **Downloads Alpine Linux:** Fetches the minimal, unprivileged Alpine Linux root filesystem (`minirootfs`, ~3MB) directly from official package mirrors.
2.  **Configures the Boot Initialization Sequence:** 
    *   Creates a custom `/init` startup shell script in the root directory.
    *   Sets up essential virtual filesystems (`proc`, `sysfs`, `devtmpfs`).
    *   Prepares folders for mounting the `Virtio-fs` shared workspace.
    *   Launches the in-guest agent binary (`sandforge-agent`).
3.  **Packages the RAM Disk (`initrd.img`):** Re-compresses the customized Alpine root filesystem into an initrd archive using `cpio` and `gzip`:
    ```bash
    (cd "$ROOTFS_DIR" && find . | cpio -H newc -o | gzip -9) > "./images/initrd.img"
    ```
4.  **Fetches the Optimized Hypervisor Kernel:** 
    *   Downloads the official Alpine `linux-virt` kernel package index.
    *   Extracts the minimal `vmlinuz-virt` binary (a stripped Linux kernel with all unused hardware drivers removed, resulting in a ~7MB footprint that boots in under 1 second).
5.  **Saves to Workspace Assets:** Copies the resulting binaries under `images/`.

---

## ⚙️ Overriding Image Paths

By default, the Sandforge macOS hypervisor looks for guest files inside the local `./images/` directory. You can customize or override these paths in your development shell using environment variables:

```bash
# Set custom path overrides
export SANDFORGE_KERNEL_PATH="/var/lib/sandforge/vmlinuz"
export SANDFORGE_INITRD_PATH="/var/lib/sandforge/initrd.img"

# Boot Sandforge with customized image assets
./bin/sandforge
```
