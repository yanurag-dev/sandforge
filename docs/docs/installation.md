---
sidebar_position: 2
title: Installation
description: Learn how to set up, build, and configure Sandforge dependencies on macOS and Linux.
---

# Installation ⚙️

This guide outlines the system requirements and compilation steps to set up Sandforge on your host machine.

---

## 🖥️ System Requirements

Sandforge leverages native hypervisor virtualization capabilities. Select your host operating system requirements below:

### macOS Host (Recommended Development)
* **OS Version:** macOS Monterey (12.0) or higher.
* **Architecture:** Apple Silicon (M1/M2/M3) or Intel Core processors.
* **SDK:** Xcode Command Line Tools installed (`xcode-select --install`).
* **Framework:** Native access to the Apple `Virtualization.framework`.

### Linux Host (Production Scale)
* **OS Version:** Ubuntu 22.04 LTS, Debian 11+, or RHEL 9+.
* **Kernel:** v5.15 or newer with `KVM` enabled.
* **Privileges:** Read/write permissions for `/dev/kvm` (run `sudo usermod -aG kvm $USER`).

---

## 🛠️ Build Prerequisites

To compile Sandforge from source, you must install the following languages and compilers:

1. **Go (Golang):** Version `1.21` or higher.
2. **C Compiler (CGO):** Required for compiling Apple Virtualization bindings on macOS.
   * On macOS, this is bundled with the Xcode Command Line Tools.
   * On Linux, install `build-essential` (gcc, make).

---

## 📦 Compiling Sandforge CLI

Follow these steps to clone the repository, download dependencies, and compile the `sandforge` CLI:

### 1. Clone the Codebase
```bash
git clone https://github.com/yanurag-dev/sandforge.git
cd sandforge
```

### 2. Synchronize Go Dependencies
```bash
go mod tidy
```

### 3. Build the CLI Binary
Compile the binary directly into your project root:

```bash
# Compile with CGO enabled (required for macOS virtualization support)
CGO_ENABLED=1 go build -o sandforge ./cmd/sandforge
```

Verify that the CLI has successfully compiled:
```bash
./sandforge --help
```

> [!WARNING]
> **CGO Compatibility:**
> If you compile with `CGO_ENABLED=0`, macOS hypervisor virtualization bindings (`Code-Hex/vz`) will be disabled, falling back strictly to the `mock` backend. Always ensure CGO is enabled during compilation.

---

## 🐳 Linux Guest Kernel Image (Optional)

By default, Sandforge downloads a minimal guest Linux kernel and initrd root image during initialization. If you are operating inside an offline environment, you must download the images manually:

```bash
# Create local cache directory
mkdir -p ~/.config/sandforge/images

# Download minimal kernel image
curl -o ~/.config/sandforge/images/vmlinuz-guest-latest https://sandforge.dev/images/vmlinuz-guest

# Download root filesystem ramdisk
curl -o ~/.config/sandforge/images/initrd-guest-latest.img https://sandforge.dev/images/initrd-guest.img
```
