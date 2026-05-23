#!/usr/bin/env bash
# Build Linux guest images (vmlinuz + initrd.img) for the macos-vz backend.
# Requires: curl, cpio, gzip, find, go — all present on macOS with Xcode CLT.
set -euo pipefail

HOST_ARCH="$(uname -m)"
if [ "$HOST_ARCH" = "arm64" ]; then
    ALPINE_ARCH="aarch64"
    GO_ARCH="arm64"
elif [ "$HOST_ARCH" = "x86_64" ]; then
    ALPINE_ARCH="x86_64"
    GO_ARCH="amd64"
else
    echo "ERROR: Unsupported host architecture: $HOST_ARCH" >&2
    exit 1
fi

ALPINE_VERSION="3.21"
ALPINE_MIRROR="https://dl-cdn.alpinelinux.org/alpine"
ROOTFS_URL="${ALPINE_MIRROR}/v${ALPINE_VERSION}/releases/${ALPINE_ARCH}/alpine-minirootfs-${ALPINE_VERSION}.0-${ALPINE_ARCH}.tar.gz"

REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
IMAGES_DIR="$REPO_DIR/images"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

echo "==> Work dir: $WORK_DIR"
echo "==> Output:   $IMAGES_DIR"
mkdir -p "$IMAGES_DIR"

# ── 1. Cross-compile guest agent ───────────────────────────────────────────
echo "==> Building sandforge-agent (linux/${GO_ARCH})..."
GOOS=linux GOARCH="$GO_ARCH" go build \
    -o "$WORK_DIR/sandforge-agent" \
    "$REPO_DIR/cmd/guest-agent"
echo "    agent: $(du -sh "$WORK_DIR/sandforge-agent" | cut -f1)"

# ── 2. Download Alpine minirootfs ──────────────────────────────────────────
ROOTFS_TAR="$WORK_DIR/alpine-minirootfs.tar.gz"
echo "==> Downloading Alpine ${ALPINE_VERSION} minirootfs..."
curl -fsSL "$ROOTFS_URL" -o "$ROOTFS_TAR"

# ── 3. Extract rootfs ──────────────────────────────────────────────────────
ROOTFS_DIR="$WORK_DIR/rootfs"
mkdir -p "$ROOTFS_DIR"
echo "==> Extracting rootfs..."
tar -xzf "$ROOTFS_TAR" -C "$ROOTFS_DIR"

# ── 4. Install guest agent binary ─────────────────────────────────────────
AGENT_DIR="$ROOTFS_DIR/usr/local/bin"
mkdir -p "$AGENT_DIR"
cp "$WORK_DIR/sandforge-agent" "$AGENT_DIR/sandforge-agent"
chmod +x "$AGENT_DIR/sandforge-agent"

# ── 5. Create /init ────────────────────────────────────────────────────────
cat > "$ROOTFS_DIR/init" <<'INIT'
#!/bin/sh
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
echo "[init] boot start"
mount -t proc none /proc
echo "[init] proc mounted"
mount -t sysfs none /sys
echo "[init] sysfs mounted"
mount -t devtmpfs none /dev 2>/dev/null && echo "[init] devtmpfs mounted" || echo "[init] devtmpfs failed (ok)"

# Mount virtiofs share(s) — host VZ backend exposes them with tags mount0, mount1, ...
# Convention: mount0 → /workspace (used by sandforge run transient mode).
if [ -d /sys/fs/virtiofs ] || grep -q virtiofs /proc/filesystems 2>/dev/null; then
    mkdir -p /workspace
    mount -t virtiofs mount0 /workspace 2>/dev/null && echo "[init] mount0 -> /workspace" || echo "[init] mount0 not present (ok)"
fi

echo "[init] starting sandforge-agent"
exec /usr/local/bin/sandforge-agent
echo "[init] ERROR: exec failed" >&2
INIT
chmod +x "$ROOTFS_DIR/init"

# ── 6. Pack initrd ─────────────────────────────────────────────────────────
echo "==> Building initrd.img..."
(cd "$ROOTFS_DIR" && find . | cpio -H newc -o | gzip -9) > "$IMAGES_DIR/initrd.img"
echo "    initrd.img: $(du -sh "$IMAGES_DIR/initrd.img" | cut -f1)"

# ── 7. Fetch kernel ────────────────────────────────────────────────────────
# Apple VZ NewLinuxBootLoader requires a raw uncompressed kernel Image, not an
# EFI stub. Alpine linux-virt for aarch64 ships only the EFI stub (vmlinuz-virt),
# so on arm64 we use puipui-linux which ships a raw Image.gz built for VZ.
# On x86_64 Alpine's vmlinuz-virt is a bzImage which VZ accepts directly.

if [ "$HOST_ARCH" = "arm64" ]; then
    PUIPUI_VERSION="1.0.3"
    PUIPUI_URL="https://github.com/Code-Hex/puipui-linux/releases/download/v${PUIPUI_VERSION}/puipui_linux_v${PUIPUI_VERSION}_aarch64.tar.gz"
    echo "==> Fetching puipui-linux v${PUIPUI_VERSION} kernel (arm64 VZ-compatible)..."
    curl -fsSL "$PUIPUI_URL" -o "$WORK_DIR/puipui.tar.gz"
    tar -xzf "$WORK_DIR/puipui.tar.gz" -C "$WORK_DIR" ./Image.gz
    gunzip -f "$WORK_DIR/Image.gz"
    cp "$WORK_DIR/Image" "$IMAGES_DIR/vmlinuz"
else
    APKINDEX_URL="${ALPINE_MIRROR}/v${ALPINE_VERSION}/main/${ALPINE_ARCH}/APKINDEX.tar.gz"
    echo "==> Fetching Alpine package index..."
    curl -fsSL "$APKINDEX_URL" -o "$WORK_DIR/APKINDEX.tar.gz"
    tar -xzf "$WORK_DIR/APKINDEX.tar.gz" -C "$WORK_DIR" APKINDEX 2>/dev/null || true

    KERNEL_VER=$(awk '/^P:linux-virt$/{found=1} found && /^V:/{print $0; exit}' \
        "$WORK_DIR/APKINDEX" | sed 's/^V://')

    if [ -z "$KERNEL_VER" ]; then
        echo "ERROR: Could not find linux-virt version in APKINDEX" >&2
        exit 1
    fi
    echo "==> Kernel package: linux-virt-${KERNEL_VER}"

    KERNEL_APK_URL="${ALPINE_MIRROR}/v${ALPINE_VERSION}/main/${ALPINE_ARCH}/linux-virt-${KERNEL_VER}.apk"
    echo "==> Downloading linux-virt apk..."
    curl -fsSL "$KERNEL_APK_URL" -o "$WORK_DIR/linux-virt.apk"

    echo "==> Extracting vmlinuz from apk..."
    mkdir -p "$WORK_DIR/apk_extract"
    (cd "$WORK_DIR/apk_extract" && tar -xzf "$WORK_DIR/linux-virt.apk" 2>/dev/null || true)

    VMLINUZ=$(find "$WORK_DIR/apk_extract" -name "vmlinuz-virt" | head -1)
    if [ -z "$VMLINUZ" ]; then
        echo "ERROR: vmlinuz-virt not found in apk" >&2
        exit 1
    fi
    cp "$VMLINUZ" "$IMAGES_DIR/vmlinuz"
fi

echo "    vmlinuz:    $(du -sh "$IMAGES_DIR/vmlinuz" | cut -f1)"

echo ""
echo "==> Done. Images written to $IMAGES_DIR/"
echo "    $IMAGES_DIR/vmlinuz"
echo "    $IMAGES_DIR/initrd.img"
