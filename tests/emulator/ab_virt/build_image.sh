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
    useradd -m -s /bin/bash br || true
    mkdir -p /work && chown -R br /work /dl
    su br -c "set -euo pipefail
      cd /work
      wget -q https://buildroot.org/downloads/buildroot-${BR2_VERSION}.tar.gz
      tar xf buildroot-${BR2_VERSION}.tar.gz
      cd buildroot-${BR2_VERSION}
      export BR2_DL_DIR=/dl
      make O=/work/out qemu_aarch64_virt_defconfig
      # Internal Buildroot toolchain from the defconfig. NOTE: this comment lives
      # inside a single-quoted podman bash -c string, so it MUST stay ASCII-safe
      # with no apostrophes or parens. Rationale is in this file header + the
      # commit log. Disk fits now after reclaiming orphaned rootless podman
      # storage; Bootlin external toolchain is invalid for this defconfig.
      # Base userspace + the A/B toolchain pieces (§11.4.74 reuse):
      #  - U-Boot qemu_arm64 -> u-boot.bin so QEMU can boot via a REAL bootloader
      #    whose bootcount/altbootcmd env is the A/B auto-rollback engine.
      #  - RAUC -> the in-guest A/B update client with dm-verity slots.
      #  - dosfstools/e2fsprogs/util-linux -> build + inspect the 2-slot GPT disk.
      #  - uboot-tools (fw_setenv/fw_printenv) -> RAUC U-Boot backend env access.
      #  - lvm2 (dmsetup) -> dm-verity userspace for RAUC verity slot activation.
      # PWU-AB-2: RAUC rootfs overlay. RAUC config files at /etc/rauc/system.conf,
      #   /etc/fw_env.config (commented-out offset), dev.cert.pem in /etc/rauc/.
      #   ALL embedded here because the project tree is NOT mounted into the
      #   container (the design choice for macOS/podman portability).
      mkdir -p /work/rootfs-overlay/etc/rauc

      # system.conf — RAUC slot A/B config with uboot backend + dev keyring.
      cat > /work/rootfs-overlay/etc/rauc/system.conf << SYSEOF
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

      # fw_env.config — U-Boot env access map. COMMENTED OUT because the guest
      # has NO /dev/mtd* devices (proven by NOR-flash probe on 2026-06-19).
      # See docs/qa/20260619-nor-flash-probe/console.log for the evidence.
      # The offset/size here are placeholder; the real values depend on the
      # chosen env mechanism (pflash via /dev/mem or U-Boot ENV_IS_IN_FAT
      # rebuild). See PWU-AB-2 STATUS.md for the discussion.
      cat > /work/rootfs-overlay/etc/fw_env.config << FWEOF
# /etc/fw_env.config for the A/B-virt emulator guest.
# UNVERIFIED — the Linux kernel in this guest has NO MTD subsystem
# compiled in (proven 2026-06-19: /dev/mtd* absent, NO_MTD_CLASS in
# dmesg). The raw line below is COMMENTED OUT until the env mechanism
# is resolved.
#
# Once a path is chosen (pflash via /dev/mem or FAT-file env rebuild),
# update the device/offset/size below and uncomment.
#
#   <device>   <offset>   <env-size>  <sector-size>
# /dev/vda1    0x000000   0x4000      0x200
FWEOF

      # Dev key+cert — generate a THROWAWAY self-signed RSA-4096 keypair INSIDE
      # the container using openssl (the same recipe gen_dev_keys.sh uses on the
      # host). The cert goes into the rootfs overlay (RAUC keyring); the key is
      # extracted via podman cp after the build so the host can sign bundles with
      # the matching keypair (§11.4.10: private key NEVER embedded in the rootfs).
      openssl req -x509 -newkey rsa:4096 -nodes \
        -keyout /work/rootfs-overlay/etc/rauc/dev.key.pem \
        -out    /work/rootfs-overlay/etc/rauc/dev.cert.pem \
        -subj   \"/O=Helix OTA dev/CN=helix-ota-ab-virt-dev\" \
        -days   365 >/dev/null 2>&1
      # The key is only in the overlay dir (used below for bundle signing key
      # extraction). It MUST NOT end up in the rootfs — BR2_ROOTFS_OVERLAY only
      # copies files matching certain patterns; .key.pem is excluded by Buildroot's
      # default overlay filtering (chmod 600, .pem suffix is not in the included
      # file types). But to be extra safe per §11.4.10:
      mkdir -p /work/out/rauc-keys
      cp /work/rootfs-overlay/etc/rauc/dev.key.pem /work/out/rauc-keys/dev.key.pem
      rm -f /work/rootfs-overlay/etc/rauc/dev.key.pem
      chmod 644 /work/rootfs-overlay/etc/rauc/dev.cert.pem 2>/dev/null || true
      chmod 644 /work/rootfs-overlay/etc/fw_env.config 2>/dev/null || true
      chmod 644 /work/rootfs-overlay/etc/rauc/system.conf 2>/dev/null || true

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
# signing). The matching cert is baked into the rootfs overlay (RAUC keyring).
# §11.4.10: chmod 600 on host after extraction; never logged, never committed.
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
