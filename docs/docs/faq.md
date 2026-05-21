---
sidebar_position: 13
title: FAQ
description: Technical frequently asked questions about the Sandforge hypervisor sandbox.
---

# FAQ ❓

Frequently asked technical questions regarding the design, security, and operation of Sandforge.

---

## 🔒 1. Is it safe to execute active malware or destructive scripts inside Sandforge?

**Yes.** Unlike traditional Docker container runtimes that share the host machine's macOS or Linux kernel, Sandforge provisions a hard hypervisor boundary. 

* The guest virtual machine runs a completely separate guest kernel. Even if a script exploits a kernel vulnerability (privilege escalation), it remains locked inside the VM memory context.
* File write and network access are validated on the host *prior* to transmission. Standard directory escapes (e.g. symlink climbing) are caught by the host-side Policy Engine before the guest handles the request.

---

## 🏎️ 2. How is Sandforge faster than standard virtual machines (e.g., VirtualBox, VMware)?

Traditional virtualization hypervisors boot an entire operating system, initialize graphical systems, and load heavy hardware drivers, taking 15 to 45 seconds.

Sandforge is optimized for autonomous micro-lifecycles:
* **Minimal Initrd Core:** The RAM disk guest root contains only standard package runtimes and a secure VSOCK server (no heavy systemd service clusters).
* **Direct Direct Kernel Booting:** Sandforge instructs Apple Virtualization / KVM to boot the kernel directly from host memory, bypassing expensive virtual BIOS / bootloader stages.
* **Result:** ephemerally spawning and booting a sandbox guest in less than **250ms**.

---

## 🪟 3. Can I run Sandforge on Microsoft Windows hosts?

**Yes, via two paths:**
1. **WSL2:** Windows Subsystem for Linux (WSL2) runs a native Linux kernel. If you enable nested virtualization in your Windows Hyper-V settings, Sandforge's KVM backend will run flawlessly inside WSL2.
2. **Mock Backend:** You can develop control planes on Windows using the built-in `Mock` driver, simulating task transitions and REST responses without hypervisor hardware.

---

## 🐳 4. Can my autonomous agent run standard Docker or Podman commands inside the guest sandbox VM?

**Yes.** The guest Linux image is pre-packaged with a rootless installation of **Podman** (which is CLI-compatible with Docker). 

Coding agents can execute standard container workflows:
```bash
./sandforge run "podman run --rm alpineecho 'Hello from Container-in-VM'"
```
The nested container is fully isolated inside the microVM kernel, keeping your host completely safe.
