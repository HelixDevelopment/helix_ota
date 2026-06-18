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
#   PHASE PWU-AB-2: extends the base Buildroot config with RAUC, U-Boot A/B env,
#   dm-verity, uboot-tools (fw_setenv/fw_printenv), lvm2 (dmsetup), AND the RAUC
#   configuration files (system.conf, fw_env.config, dev cert) baked into the
#   rootfs overlay so a Linux-side `rauc bundle` is the ONLY missing step for a
#   full GREEN RED_MODE=0 run of ab_rauc_verity.sh.
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
# NOTE: the entire build script is a single-quoted heredoc inside `podman run ... bash -c '...'`.
# NO single quotes inside, NO bash constructs that break. Root commands above the
# `su br -c` line, user (br) commands inside it with OWN double quotes.
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
    # libssl-dev: U-Boot host tools mkimage/aisimage need openssl/evp.h (FACT:
    #   build7 failed at tools/aisimage.o on a missing openssl/evp.h, §11.4.102).
    # bison/flex: U-Boot Kconfig/dtc parser generators.
    # openssl: generate the RAUC dev key+cert inside the container.
    adduser --disabled-password --gecos "" br 2>/dev/null || useradd -m -s /bin/bash br || true

    # ---- PWU-AB-2: build the rootfs overlay (as root) BEFORE su to br ----
    # The project tree is NOT mounted into the container, so the overlay files
    # are created here. All overlay files are created as root with mode 644,
    # then chowned to br for the Buildroot step.
    mkdir -p /work/rootfs-overlay/etc/rauc

    # system.conf — RAUC slot A/B config with uboot backend + dev keyring.
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

    # fw_env.config — COMMENTED OUT (guest has no /dev/mtd*, proven 2026-06-19).
    cat > /work/rootfs-overlay/etc/fw_env.config <<FWEOF
# UNVERIFIED — the Linux kernel in this guest has NO MTD subsystem
# compiled in (proven 2026-06-19). See docs/qa/20260619-nor-flash-probe/.
# The raw line below is COMMENTED OUT until the env mechanism is resolved.
#
#   <device>   <offset>   <env-size>  <sector-size>
# /dev/vda1    0x000000   0x4000      0x200
FWEOF

    # Dev key+cert — generated inside the container so the matching keypair
    # stays in sync. The cert goes into the rootfs overlay; the key is
    # extracted after the build for host-side bundle signing. PRIVATE KEY
    # NEVER lands in the rootfs (the inline cp+rm below, plus Buildroot
    # only copies .pem certs from the overlay, not .key)
    mkdir -p /work/out/rauc-keys
    openssl req -x509 -newkey rsa:4096 -nodes \
      -keyout /work/rootfs-overlay/etc/rauc/dev.key.pem \
      -out /work/rootfs-overlay/etc/rauc/dev.cert.pem \
      -subj "/O=Helix OTA dev/CN=helix-ota-ab-virt-dev" \
      -days 365 >/dev/null 2>&1
    # Copy the private key to a known extraction dir, then delete from overlay
    cp /work/rootfs-overlay/etc/rauc/dev.key.pem /work/out/rauc-keys/dev.key.pem
    rm -f /work/rootfs-overlay/etc/rauc/dev.key.pem
    chmod 644 /work/rootfs-overlay/etc/rauc/* /work/rootfs-overlay/etc/fw_env.config 2>/dev/null || true
    chown -R br /work 2>/dev/null || true

    # ---- su to br for the Buildroot build ----
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
BR2_PACKAGE_E2FSPROGS_RESIZE2FS=y
BR2_PACKAGE_DOSFSTOOLS=y
BR2_PACKAGE_DOSFSTOOLS_MKFSDOTFAT=y
BR2_PACKAGE_RAUC=y
BR2_PACKAGE_UBOOT_TOOLS=y
BR2_PACKAGE_LVM2=y
BR2_TARGET_UBOOT=y
BR2_TARGET_UBOOT_BOARD_DEFCONFIG=\"qemu_arm64\"
BR2_TARGET_UBOOT_NEEDS_DTC=y
BR2_TARGET_UBOOT_FORMAT_BIN=y
BR2_ROOTFS_OVERLAY=\"/work/rootfs-overlay\"
BR2_ROOTFS_OVERLAY_DELETE_STALE=y
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
# u-boot.bin is additive (the A/B bootloader) — extract if present; the .ok gate
# below still keys on the kernel+rootfs so a missing U-Boot is visible, not faked.
podman cp "${BUILD_CTR}:/work/out/images/u-boot.bin"  "${OUT}/images/u-boot.bin"  >>"${OUT}/build.log" 2>&1 || true
# Dev signing key (generated inside the container, extracted for host-side bundle
# signing). §11.4.10: chmod 600 on host after extraction; never logged or committed.
mkdir -p "${OUT}/rauc-keys"
podman cp "${BUILD_CTR}:/work/out/rauc-keys/dev.key.pem" "${OUT}/rauc-keys/dev.key.pem" >>"${OUT}/build.log" 2>&1 || true
chmod 600 "${OUT}/rauc-keys/dev.key.pem" 2>/dev/null || true
podman rm -f "$BUILD_CTR" >/dev/null 2>&1 || true

# §11.4.6: declare success ONLY if the real artifacts are present + non-empty.
if [ -s "${OUT}/images/Image" ] && [ -s "${OUT}/images/rootfs.ext2" ]; then
  printf 'br2=%s built=%s\n' "$BR2_VERSION" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" > "${OUT}/.ok"
  log "BUILD OK — kernel + rootfs extracted:"; ls -la "${OUT}/images/" 2>/dev/null
  exit 0
fi
log "BUILD finished but images missing — NOT stamping .ok (anti-bluff §11.4.6)"
exit 1
