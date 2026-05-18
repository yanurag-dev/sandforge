#!/usr/bin/env bash
# Build Linux guest images (vmlinuz + initrd.img) for the macos-vz backend.
# Requires: curl, cpio, gzip, find — all present on macOS with Xcode CLT.
set -euo pipefail

ALPINE_VERSION="3.21"
ALPINE_ARCH="x86_64"
ALPINE_MIRROR="https://dl-cdn.alpinelinux.org/alpine"
ROOTFS_URL="${ALPINE_MIRROR}/v${ALPINE_VERSION}/releases/${ALPINE_ARCH}/alpine-minirootfs-${ALPINE_VERSION}.0-${ALPINE_ARCH}.tar.gz"

IMAGES_DIR="$(cd "$(dirname "$0")/.." && pwd)/images"
WORK_DIR="$(mktemp -d)"
trap 'rm -rf "$WORK_DIR"' EXIT

echo "==> Work dir: $WORK_DIR"
echo "==> Output:   $IMAGES_DIR"
mkdir -p "$IMAGES_DIR"

# ── 1. Download Alpine minirootfs ──────────────────────────────────────────
ROOTFS_TAR="$WORK_DIR/alpine-minirootfs.tar.gz"
echo "==> Downloading Alpine ${ALPINE_VERSION} minirootfs..."
curl -fsSL "$ROOTFS_URL" -o "$ROOTFS_TAR"

# ── 2. Extract rootfs ──────────────────────────────────────────────────────
ROOTFS_DIR="$WORK_DIR/rootfs"
mkdir -p "$ROOTFS_DIR"
echo "==> Extracting rootfs..."
tar -xzf "$ROOTFS_TAR" -C "$ROOTFS_DIR"

# ── 3. Install guest agent placeholder ────────────────────────────────────
# The real guest agent binary will be built separately (cmd/guest-agent).
# For now, drop a stub so the initrd structure is correct.
AGENT_DIR="$ROOTFS_DIR/usr/local/bin"
mkdir -p "$AGENT_DIR"
cat > "$AGENT_DIR/sandforge-agent" <<'STUB'
#!/bin/sh
# Sandforge guest agent stub — replace with real binary before use.
echo "sandforge-agent: stub — not implemented" >&2
exit 1
STUB
chmod +x "$AGENT_DIR/sandforge-agent"

# ── 4. Create /init that starts the agent ─────────────────────────────────
cat > "$ROOTFS_DIR/init" <<'INIT'
#!/bin/sh
mount -t proc none /proc
mount -t sysfs none /sys
mount -t devtmpfs none /dev 2>/dev/null || mdev -s

# Start guest agent
exec /usr/local/bin/sandforge-agent
INIT
chmod +x "$ROOTFS_DIR/init"

# ── 5. Pack initrd ─────────────────────────────────────────────────────────
echo "==> Building initrd.img..."
(cd "$ROOTFS_DIR" && find . | cpio -H newc -o | gzip -9) > "$IMAGES_DIR/initrd.img"
echo "    initrd.img: $(du -sh "$IMAGES_DIR/initrd.img" | cut -f1)"

# ── 6. Fetch kernel from Alpine packages ──────────────────────────────────
# Download linux-virt (minimal kernel, no modules — ideal for VMs).
APKINDEX_URL="${ALPINE_MIRROR}/v${ALPINE_VERSION}/main/${ALPINE_ARCH}/APKINDEX.tar.gz"
echo "==> Fetching Alpine package index..."
curl -fsSL "$APKINDEX_URL" -o "$WORK_DIR/APKINDEX.tar.gz"
tar -xzf "$WORK_DIR/APKINDEX.tar.gz" -C "$WORK_DIR" APKINDEX 2>/dev/null || true

# Parse kernel package version from APKINDEX
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

# APK files are gzip'd tar; strip the first 512-byte pkg sig header.
echo "==> Extracting vmlinuz from apk..."
mkdir -p "$WORK_DIR/apk_extract"
# Alpine apks: outer gz stream is the package; inner tar has the files.
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
