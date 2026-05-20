#!/usr/bin/env bash
# Build Linux guest images (vmlinuz + initrd.img) for the macos-vz backend.
# Requires: curl, cpio, gzip, find, go — all present on macOS with Xcode CLT.
set -euo pipefail

ALPINE_VERSION="3.21"
ALPINE_ARCH="x86_64"
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
echo "==> Building sandforge-agent (linux/amd64)..."
GOOS=linux GOARCH=amd64 go build \
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
mount -t proc none /proc
mount -t sysfs none /sys
mount -t devtmpfs none /dev 2>/dev/null || mdev -s

# Start guest agent (PID 2 — keeps VM alive)
exec /usr/local/bin/sandforge-agent
INIT
chmod +x "$ROOTFS_DIR/init"

# ── 6. Pack initrd ─────────────────────────────────────────────────────────
echo "==> Building initrd.img..."
(cd "$ROOTFS_DIR" && find . | cpio -H newc -o | gzip -9) > "$IMAGES_DIR/initrd.img"
echo "    initrd.img: $(du -sh "$IMAGES_DIR/initrd.img" | cut -f1)"

# ── 7. Fetch kernel from Alpine packages ──────────────────────────────────
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
echo "    vmlinuz:    $(du -sh "$IMAGES_DIR/vmlinuz" | cut -f1)"

echo ""
echo "==> Done. Images written to $IMAGES_DIR/"
echo "    $IMAGES_DIR/vmlinuz"
echo "    $IMAGES_DIR/initrd.img"
