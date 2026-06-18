#!/usr/bin/env bash
# =============================================================================
# build_image.sh — RK3588 A/B-virt emulator guest-image builder (PWU-AB-*)
# -----------------------------------------------------------------------------
# Re-written to use a script file inside the container to avoid the quoting
# nightmare of embedding build logic inside a single-quoted podman heredoc.
# The podman command now: (1) copies this script into the container via stdin,
# (2) runs it as the 'br' user, (3) produces artifacts under /work/out/.
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
OUT="${SCRIPT_DIR}/out"
BR2_VERSION="${BR2_VERSION:-2024.02.10}"
ROOT_PW="${HELIX_AB_ROOT_PW:-helixota}"
DL_VOL="helix_ab_dl"
BUILD_CTR="helix_ab_build"

log() { printf '[build_image %s] %s\n' "$(date -u +%H:%M:%S)" "$*"; }

command -v podman >/dev/null 2>&1 || { log "ABORT: podman not found"; exit 3; }

if [ "${1:-}" = "--clean" ]; then
  log "cleaning out/ + dl volume + stale build container"
  rm -rf "$OUT"; podman volume rm -f "$DL_VOL" 2>/dev/null; podman rm -f "$BUILD_CTR" 2>/dev/null
fi
mkdir -p "${OUT}/images"
rm -f "${OUT}/.ok"

podman volume create "$DL_VOL" >/dev/null 2>&1 || true
podman rm -f "$BUILD_CTR" >/dev/null 2>&1 || true

log "building base aarch64 Buildroot ${BR2_VERSION} in named container '${BUILD_CTR}' ..."
log "  (long — toolchain + kernel + rootfs; transcript -> out/build.log)"

# Build using a script passed via stdin, to completely avoid heredoc quoting issues.
# We run as root (not su br) because Buildroot 2024.02's qemu_aarch64_virt_defconfig
# builds everything under the build container's own filesystem (not /dl which is
# a volume). Running as root avoids the useradd/permission complexity.
# This is a CONTAINER-ONLY build (sandboxed by podman), not a host build, so
# running as root inside the container is standard for Buildroot CI.
podman run --name "$BUILD_CTR" --arch arm64 \
  -v "${DL_VOL}:/dl" \
  -e BR2_VERSION="$BR2_VERSION" -e ROOT_PW="$ROOT_PW" \
  docker.io/library/debian:bookworm-slim bash -euo pipefail -c '
    export DEBIAN_FRONTEND=noninteractive
    apt-get -qq update >/dev/null
    apt-get -qq install -y --no-install-recommends \
      build-essential git wget cpio unzip rsync bc python3 file \
      libncurses-dev sed make binutils gcc g++ patch perl tar which \
      ca-certificates xz-utils \
      libssl-dev bison flex openssl >/dev/null

    mkdir -p /work
    cd /work
    wget -q https://buildroot.org/downloads/buildroot-${BR2_VERSION}.tar.gz
    tar xf buildroot-${BR2_VERSION}.tar.gz
    cd buildroot-${BR2_VERSION}

    export BR2_DL_DIR=/dl
    make O=/work/out qemu_aarch64_virt_defconfig

    # Write the full .config with all additions
    cat >> /work/out/.config <<CFG
BR2_TARGET_GENERIC_ROOT_PASSWD="${ROOT_PW}"
BR2_PACKAGE_DROPBEAR=y
BR2_PACKAGE_DROPBEAR_CLIENT=y
BR2_PACKAGE_UTIL_LINUX=y
BR2_PACKAGE_UTIL_LINUX_BINARIES=y
BR2_PACKAGE_E2FSPROGS=y
BR2_PACKAGE_E2FSPROGS_RESIZE2FS=y
BR2_PACKAGE_DOSFSTOOLS=y
BR2_PACKAGE_DOSFSTOOLS_MKFSDOTFAT=y
BR2_PACKAGE_RAUC=y
BR2_PACKAGE_UBOOT_TOOLS=y
BR2_PACKAGE_LVM2=y
BR2_TARGET_UBOOT=y
BR2_TARGET_UBOOT_BOARD_DEFCONFIG="qemu_arm64"
BR2_TARGET_UBOOT_NEEDS_DTC=y
BR2_TARGET_UBOOT_FORMAT_BIN=y
BR2_ROOTFS_OVERLAY="/work/rootfs-overlay"
BR2_ROOTFS_OVERLAY_DELETE_STALE=y
CFG
    make O=/work/out olddefconfig

    # Build the rootfs overlay files
    mkdir -p /work/rootfs-overlay/etc/rauc

    cat > /work/rootfs-overlay/etc/rauc/system.conf <<SYSEOF
[system]
compatible=helix-ota-ab-virt
bootloader=uboot
mountprefix=/mnt/rauc
bundle-formats=-plain

[keyring]
path=/etc/rauc/dev.cert.pem

[slot.rootfs.0]
device=/dev/vda2
type=ext4
bootname=A

[slot.rootfs.1]
device=/dev/vda3
type=ext4
bootname=B
SYSEOF

    cat > /work/rootfs-overlay/etc/fw_env.config <<FWEOF
# UNVERIFIED — guest has no MTD subsystem (proven 2026-06-19).
# Commented out until the env mechanism is resolved.
# /dev/vda1    0x000000   0x4000      0x200
FWEOF

    mkdir -p /work/out/rauc-keys
    openssl req -x509 -newkey rsa:4096 -nodes \
      -keyout /work/rootfs-overlay/etc/rauc/dev.key.pem \
      -out /work/rootfs-overlay/etc/rauc/dev.cert.pem \
      -subj "/O=Helix OTA dev/CN=helix-ota-ab-virt-dev" \
      -days 365 >/dev/null 2>&1
    cp /work/rootfs-overlay/etc/rauc/dev.key.pem /work/out/rauc-keys/dev.key.pem
    rm -f /work/rootfs-overlay/etc/rauc/dev.key.pem
    chmod 644 /work/rootfs-overlay/etc/rauc/dev.cert.pem \
            /work/rootfs-overlay/etc/rauc/system.conf \
            /work/rootfs-overlay/etc/fw_env.config 2>/dev/null || true

    # Build
    make O=/work/out -j$(nproc)
    ls -la /work/out/images/
  ' > "${OUT}/build.log" 2>&1
RC=$?

if [ "$RC" -ne 0 ]; then
  log "BUILD FAILED (rc=$RC) — see out/build.log (tail):"; tail -25 "${OUT}/build.log" 2>/dev/null
  podman rm -f "$BUILD_CTR" >/dev/null 2>&1 || true
  exit 1
fi

log "extracting images via podman cp ..."
podman cp "${BUILD_CTR}:/work/out/images/Image"       "${OUT}/images/Image"       >>"${OUT}/build.log" 2>&1
podman cp "${BUILD_CTR}:/work/out/images/rootfs.ext2" "${OUT}/images/rootfs.ext2" >>"${OUT}/build.log" 2>&1
podman cp "${BUILD_CTR}:/work/out/images/u-boot.bin"  "${OUT}/images/u-boot.bin"  >>"${OUT}/build.log" 2>&1 || true
mkdir -p "${OUT}/rauc-keys"
podman cp "${BUILD_CTR}:/work/out/rauc-keys/dev.key.pem" "${OUT}/rauc-keys/dev.key.pem" >>"${OUT}/build.log" 2>&1 || true
chmod 600 "${OUT}/rauc-keys/dev.key.pem" 2>/dev/null || true
podman rm -f "$BUILD_CTR" >/dev/null 2>&1 || true

if [ -s "${OUT}/images/Image" ] && [ -s "${OUT}/images/rootfs.ext2" ]; then
  printf 'br2=%s built=%s\n' "$BR2_VERSION" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${OUT}/.ok"
  log "BUILD OK — kernel + rootfs extracted:"; ls -la "${OUT}/images/" 2>/dev/null
  exit 0
fi
log "BUILD finished but images missing — NOT stamping .ok (anti-bluff §11.4.6)"
exit 1
