#!/usr/bin/env bash
# =============================================================================
# build_image.sh — RK3588 A/B-virt emulator guest-image builder (PWU-AB-*)
# -----------------------------------------------------------------------------
# Purpose:
#   Build a bootable aarch64 Linux guest image for the dev-host RK3588 / Orange
#   Pi 5 Max A/B emulator (docs/research/rk3588_emulator/REPORT.md). The image
#   boots under QEMU `-machine virt` + HVF on this Apple-Silicon host and is the
#   foundation the A/B (U-Boot bootcount + RAUC dm-verity) layers build on.
#
#   PHASE PWU-AB-0 (this script's current target): a BASE Buildroot aarch64
#   rootfs + kernel with dropbear SSH + a known root credential. Later phases
#   extend the Buildroot config with a 2-slot GPT, U-Boot A/B env, RAUC, and
#   dm-verity (reuse per §11.4.74 — RAUC + U-Boot, never reimplemented).
#
#   Runs INSIDE a podman aarch64 Linux container (this host is macOS; Buildroot
#   needs a Linux build host) — native arm64, no emulation (§11.4.76 spirit).
#
# macOS/podman portability (FACT): the project lives on /Volumes/T7 (an external
#   volume NOT shared into the podman machine VM), and macOS podman rejects the
#   `:Z` SELinux relabel. So we do NOT bind-mount the project tree: the build
#   runs in a NAMED container's own filesystem with a NAMED volume for the
#   Buildroot download cache, and the two image artifacts are extracted to the
#   host out/ dir via `podman cp` (a host-side write, works on /Volumes/T7).
#
# Usage:
#   tests/emulator/ab_virt/build_image.sh            # build
#   tests/emulator/ab_virt/build_image.sh --clean    # wipe out/ + dl volume
#   Env: BR2_VERSION (default 2024.02.10), HELIX_AB_ROOT_PW (default helixota)
#
# Outputs (gitignored, §11.4.30/§11.4.77 regen mechanism):
#   tests/emulator/ab_virt/out/images/{Image,rootfs.ext2}
#   tests/emulator/ab_virt/out/build.log   (full build transcript)
#   tests/emulator/ab_virt/out/.ok         (stamp written ONLY on real success)
#
# Dependencies: podman (aarch64 Linux container + a named volume), ~10-20 GB
#   podman-machine disk, network.
# Cross-references: §11.4.74 (reuse RAUC+U-Boot), §11.4.76, §11.4.77, §11.4.30,
#   §11.4.6 (only stamp .ok on real artifacts — never claim a phantom build).
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

# Build in the container's own FS (/work); persist the Buildroot DL cache in the
# named volume so re-runs don't re-download. Buildroot refuses to build as root,
# so a dedicated 'br' user owns the tree.
podman run --name "$BUILD_CTR" --arch arm64 \
  -v "${DL_VOL}:/dl" \
  -e BR2_VERSION="$BR2_VERSION" -e ROOT_PW="$ROOT_PW" \
  docker.io/library/debian:bookworm-slim bash -euo pipefail -c '
    export DEBIAN_FRONTEND=noninteractive
    apt-get -qq update >/dev/null
    apt-get -qq install -y --no-install-recommends \
      build-essential git wget cpio unzip rsync bc python3 file \
      libncurses-dev sed make binutils gcc g++ patch perl tar which \
      ca-certificates xz-utils >/dev/null
    useradd -m -s /bin/bash br || true
    mkdir -p /work && chown -R br /work /dl
    su br -c "set -euo pipefail
      cd /work
      wget -q https://buildroot.org/downloads/buildroot-${BR2_VERSION}.tar.gz
      tar xf buildroot-${BR2_VERSION}.tar.gz
      cd buildroot-${BR2_VERSION}
      export BR2_DL_DIR=/dl
      make O=/work/out qemu_aarch64_virt_defconfig
      cat >> /work/out/.config <<CFG
BR2_TARGET_GENERIC_ROOT_PASSWD=\"${ROOT_PW}\"
BR2_PACKAGE_DROPBEAR=y
BR2_PACKAGE_DROPBEAR_CLIENT=y
BR2_PACKAGE_UTIL_LINUX=y
BR2_PACKAGE_UTIL_LINUX_BINARIES=y
BR2_PACKAGE_E2FSPROGS=y
CFG
      make O=/work/out olddefconfig
      make O=/work/out -j\$(nproc)
      ls -la /work/out/images/
    "
  ' > "${OUT}/build.log" 2>&1
RC=$?

if [ "$RC" -ne 0 ]; then
  log "BUILD FAILED (rc=$RC) — see out/build.log (tail):"; tail -25 "${OUT}/build.log" 2>/dev/null
  podman rm -f "$BUILD_CTR" >/dev/null 2>&1 || true
  exit 1
fi

# Extract the two artifacts host-side via podman cp (no bind-mount needed).
log "extracting images via podman cp ..."
podman cp "${BUILD_CTR}:/work/out/images/Image"       "${OUT}/images/Image"       >>"${OUT}/build.log" 2>&1
podman cp "${BUILD_CTR}:/work/out/images/rootfs.ext2" "${OUT}/images/rootfs.ext2" >>"${OUT}/build.log" 2>&1
podman rm -f "$BUILD_CTR" >/dev/null 2>&1 || true

# §11.4.6: declare success ONLY if the real artifacts are present + non-empty.
if [ -s "${OUT}/images/Image" ] && [ -s "${OUT}/images/rootfs.ext2" ]; then
  printf 'br2=%s built=%s\n' "$BR2_VERSION" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${OUT}/.ok"
  log "BUILD OK — kernel + rootfs extracted:"; ls -la "${OUT}/images/" 2>/dev/null
  exit 0
fi
log "BUILD finished but images missing — NOT stamping .ok (anti-bluff §11.4.6)"
exit 1
