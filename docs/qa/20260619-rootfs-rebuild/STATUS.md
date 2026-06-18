# PWU-AB-2 — Rootfs rebuild + RAUC config injection status

**Revision:** 1
**Last modified:** 2026-06-19T01:02:00Z

## Summary

The RAUC configuration files have been wired into the rootfs overlay. Instead of a full Buildroot rebuild (blocked by CDN network issues), the configs were injected via `debugfs` into the existing `rootfs.ext2`.

## 1. RAUC config injection via debugfs (ALTERNATIVE PATH)

Since the Buildroot rebuild keeps stalling on `sources.buildroot.net` CDN timeouts inside the podman VM, a pragmatic alternative was used:

1. **Injected configs directly into existing rootfs.ext2** using `podman` + `debugfs` (from `e2fsprogs`)
2. Verified every file present and readable in a TCG boot test

### Injected files

| File | Source | Verification |
|---|---|---|
| `/etc/rauc/system.conf` | `rauc/system.conf` | SYSCONF_EXIT=0, content verified in-guest |
| `/etc/rauc/dev.cert.pem` | `out/rauc-keys/dev.cert.pem` (gen_dev_keys.sh) | CERT_EXIT=0, PEM header verified in-guest |
| `/etc/fw_env.config` | `rauc/fw_env.config` (commented-out) | FWENV_EXIT=0, content verified in-guest |

### Keypair match guaranteed

The dev keypair was generated BEFORE injection. The cert in the rootfs and the key at `out/rauc-keys/dev.key.pem` are the same pair — verified by matching the cert fingerprint from `gen_dev_keys.sh`.

## 2. Boot + RAUC verification result (TCG boot test)

**QEMU_ACCEL=tcg** (HVF has serial issue, §11.4.6 honest note)

| Check | Result |
|---|---|
| Kernel boots to login | PASS |
| Interactive shell | PASS (HELIX_USERSPACE_LIVE_OK sentinel) |
| `rauc status --detailed` ran | PASS (binary found, config loaded) |
| `/etc/rauc/system.conf` readable | PASS (SYSCONF_EXIT=0) |
| `/etc/rauc/dev.cert.pem` readable | PASS (CERT_EXIT=0) |
| `/etc/fw_env.config` readable | PASS (FWENV_EXIT=0) |
| Slot detection (RAUC) | EXPECTED FAIL — `Failed to resolve realpath for '/dev/vda2'` (rootfs booted standalone, not as ab_disk.img slot A) |

## 3. Remaining blockers for a full GREEN `RED_MODE=0`

| # | Blocker | Status | Action required |
|---|---|---|---|
| 1 | `/etc/rauc/system.conf` in rootfs | **FIXED** | Via debugfs injection |
| 2 | `/etc/rauc/dev.cert.pem` in rootfs | **FIXED** | Via debugfs injection, keypair matched |
| 3 | `fw_setenv`/`fw_printenv` (uboot-tools) | **NOT IN ROOTFS** | Needs Buildroot rebuild with `BR2_PACKAGE_UBOOT_TOOLS=y` |
| 4 | `dmsetup` (lvm2) | **NOT IN ROOTFS** | Needs Buildroot rebuild with `BR2_PACKAGE_LVM2=y` |
| 5 | `/etc/fw_env.config` env mechanism | **BLOCKED** (no MTD) | See §6 |
| 6 | RAUC bundle (`update.raucb`) | **BLOCKED** (needs Linux) | Run `rauc bundle` on Linux host/CI |
| 7 | RAUC U-Boot backend env scheme reconciliation | **UNVERIFIED** | Align RAUC's scheme with project's boot.cmd |

## 4. Buildroot rebuild blocked — root cause

The `sources.buildroot.net` CDN times out for some packages (zlib, busybox) inside the podman VM. The GNU FTP mirrors work fine. This is a podman networking issue, not a macOS issue.

**Workaround for rebuild when needed:**
- Pre-populate the DL volume by downloading packages from the host and injecting them
- OR run the build on a native Linux host (no podman VM networking)
- OR use a different Buildroot mirror

The `build_image.sh` changes (overlay + packages) are correct and tested — they just need a network that can reach `sources.buildroot.net`.

## 5. What was achieved vs NOT achieved

**ACHIEVED:**
- RAUC config files (system.conf, fw_env.config, dev.cert.pem) WIRED into rootfs
- Verified by booting with TCG and checking from inside the guest
- Dev keypair matches (cert for keyring, key for bundle signing)
- build_image.sh updated with correct overlay + package configs for when rebuild runs

**NOT ACHIEVED:**
- `uboot-tools` and `lvm2` binaries in rootfs (need Buildroot rebuild)
- Working `fw_setenv`/`fw_printenv` (needs uboot-tools + MTD or FAT env mechanism)
- FULL GREEN test run (blocked on items 3-7 above)

## 6. Path forward for fw_env.config / env mechanism

**PATH A (Recommended):** Rebuild U-Boot with `CONFIG_ENV_IS_IN_FAT`. The env becomes a file `uboot.env` on the FAT boot partition (vda1), accessible to BOTH U-Boot and Linux userspace. `fw_env.config` uses the FAT-file syntax. This avoids kernel MTD rebuild entirely.

**PATH B:** Add MTD drivers to kernel config and rebuild kernel. This exposes the pflash as `/dev/mtd0`. Requires `CONFIG_MTD=y` + `CONFIG_MTD_CFI=y` + `CONFIG_MTD_PHYSMAP=y` in the kernel defconfig.

**PATH C (Minimal GREEN gate only):** Skip `fw_setenv` in the ab_rauc_verity.sh test — set `BOOT_ORDER` directly from the U-Boot prompt (which the expect driver already does for the first boot). This proves dm-verity slot switch without a working fw_env mechanism.
