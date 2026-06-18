# RAUC dm-verity PWU-AB-2 — Install attempt + RED_MODE=1 baseline

**Revision:** 1
**Last modified:** 2026-06-19T00:00:00Z

## 1. rauc CLI installation on macOS host

`brew install rauc` — **FAILED**. No Homebrew formula exists for `rauc` (suggestion was `runc`, which is unrelated).

`rauc` build from source (https://github.com/rauc/rauc) — **FAILED on macOS**:
- `meson setup build` failed with: `Dependency "libnl-genl-3.0" not found`
- `libnl` is a Linux-specific netlink library; the Homebrew `libnl` formula exists but does NOT install a `libnl-genl-3.0.pc` pkg-config file on macOS (macOS has no netlink).
- **Root cause:** `rauc` depends on Linux kernel interfaces (netlink/genl, dbus, fdisk) and cannot build natively on macOS.

**Conclusion:** `rauc` host binary is not buildable on macOS. The project's CI/build infrastructure (inside the Podman aarch64 Linux container where Buildroot builds) or a dedicated Linux host is needed for bundle building.

`u-boot-tools` (`fw_setenv`/`fw_printenv`):

| Package | Status |
|---|---|
| `u-boot-tools` | Installed via `brew install u-boot-tools` (v2026.04) |

### What works on macOS host

All the tools needed to run the test driver itself are available:
- `qemu-system-aarch64` — installed (QEMU 11.0.1)
- `expect` — available at `/usr/bin/expect`
- `openssl` — available (key generation)

## 2. Dev keys generation

**PASS.** `bash tests/emulator/ab_virt/rauc/gen_dev_keys.sh` succeeded:

| Artifact | Path | Status |
|---|---|---|
| Private key | `tests/emulator/ab_virt/out/rauc-keys/dev.key.pem` | `chmod 600`, gitignored |
| Public cert | `tests/emulator/ab_virt/out/rauc-keys/dev.cert.pem` | `chmod 644`, gitignored |
| Fingerprint | `CB:2D:F8:80:75:13:B9:46:A2:AC:5A:3D:89:02:E1:D0:78:61:20:01:2A:27:7E:3E:70:08:70:15:AC:D8:57:06` | Self-signed dev, 365d |

## 3. Build artifacts

Preconditions met — all artifacts present and intact:

| Artifact | Size | Status |
|---|---|---|
| `out/.ok` | 42 B | Present |
| `out/.disk_ok` | 51 B | Present |
| `out/images/u-boot.bin` | 1.0 MB | U-Boot 2024.01 (Jun 11 2026) |
| `out/images/Image` | 11 MB | Linux 6.1.44 |
| `out/images/rootfs.ext2` | 60 MB | Buildroot ext4 |
| `out/images/ab_disk.img` | 1.0 GB | GPT with vda1 (fat boot), vda2 (slot A root), vda3 (slot B root) |
| `out/images/boot.scr` | 5.8 KB | U-Boot boot script |

## 4. ab_rauc_verity.sh RED_MODE=1 run

### QEMU acceleration workaround

QEMU 11.0.1 on Apple Silicon has a serial-output issue with `-accel hvf` — the UART produces no output and expect times out waiting for the autoboot prompt. Prior runs (PWU-AB-1/AB-3) which also used HVF had the same issue but were run with an older QEMU or different configuration.

**Workaround applied:** The script was extended with `QEMU_ACCEL` environment variable support (default `hvf`). The run used `QEMU_ACCEL=tcg` which works correctly (serial output intact, though ~30x slower).

### Run details

| Field | Value |
|---|---|
| Run ID | `20260618T210924Z-ab-rauc-verity` |
| RED_MODE | 1 (defect-present baseline) |
| Acceleration | `tcg` |
| CPU | `max` |
| Guest root PW | `helixota` |
| Bundle path (guest) | `/root/update.raucb` (does NOT exist — expected) |

### Console evidence

**540 lines** captured at `/Volumes/T7/Projects/helix_ota/docs/qa/20260618T210924Z-ab-rauc-verity/console.log`

### Verdict: PASS (RED baseline)

**Exit code:** 0
**expect rc:** 0

All 6 assertions correct:

| # | Assertion | Expected (RED_MODE=1) | Actual | Result |
|---|---|---|---|---|
| 1 | First boot slot A | PASS (baseline must hold) | `HELIX_PRESLOT=A` | PASS |
| 2 | rauc install driven | PASS (binary invoked) | `Failed to initialize context` | PASS |
| 3 | Post-reboot shell live | PASS (post-login sentinel) | `HELIX_DONE_RAUC_MARK` | PASS |
| 4 | rauc install rc=0 | Must FAIL (no bundle) | `HELIX_RAUC_INSTALL_RC=1` | PASS (expected fail) |
| 5 | dm-verity active | Must FAIL (not wired) | `HELIX_DMVERITY=0` | PASS (expected fail) |
| 6 | Slot switched to B | Must FAIL (apply unproven) | `HELIX_POSTSLOT=A` | PASS (expected fail) |

## 5. Analysis of RED baseline findings

The test correctly proves the defect-present state. Here is what the RED baseline reveals about what is missing for a GREEN `RED_MODE=0` run:

### Blocker 1: `/etc/rauc/system.conf` not installed in rootfs (HIGHEST PRIORITY)

`rauc status` and `rauc install` both fail with:
```
Failed to initialize context: Failed to load system config (/etc/rauc/system.conf):
Failed to open file ?/etc/rauc/system.conf?: No such file or directory
```

The `system.conf` at `tests/emulator/ab_virt/rauc/system.conf` exists in the repo but is NOT wired into the Buildroot rootfs overlay. **Fix:** Add a Buildroot rootfs overlay (`board/helix/ab_virt/rootfs_overlay/etc/rauc/system.conf`) in `build_image.sh` that copies the config artifact and dev cert into both slot root filesystems.

### Blocker 2: No dm-verity userspace tools in rootfs

`dmsetup: not found` — the `device-mapper` userspace tools are missing from the Buildroot config. While the kernel has dm-verity support compiled in, the userspace `dmsetup` binary is needed to probe verity status. **Fix:** Add `BR2_PACKAGE_LVM2=y` (provides `dmsetup`) to the Buildroot config in `build_image.sh`.

### Blocker 3: No `fw_env.config` — fw_setenv fails

`HELIX_FWSET_RC=1` — `fw_setenv` cannot operate because `/etc/fw_env.config` does not exist in the guest. The `fw_env.config` at `tests/emulator/ab_virt/rauc/fw_env.config` exists in the repo but is commented out and not installed. **Fix:** (a) Determine the actual U-Boot env storage config for the `qemu_arm64` defconfig, (b) uncomment/correct the offset/size values in `fw_env.config`, (c) install it via rootfs overlay.

### Blocker 4: No RAUC bundle (`update.raucb`) exists

`rauc install` cannot operate because no bundle was built. **Fix:** Requires `rauc` CLI on a Linux host (Podman container or CI) to run `rauc bundle` with the dev key+cert and a fresh rootfs image.

## 6. Next steps to flip GREEN (RED_MODE=0)

Ordered by dependency:

1. **Build `rauc` inside the Podman aarch64 container** (where Buildroot builds) and use it to call `rauc bundle` with the dev keys from `out/rauc-keys/` and the rendered `manifest.raucm.in`.
2. **Add `BR2_PACKAGE_LVM2=y`** to the Buildroot config to get `dmsetup` in the rootfs.
3. **Add rootfs overlay** wiring `system.conf` + `fw_env.config` + `dev.cert.pem` into `/etc/rauc/` and `/etc/` for BOTH slots.
4. **Round-trip-verify fw_setenv/fw_printenv** against U-Boot's env storage.
5. **Rebuild** guest image with the overlay, **re-run** `ab_rauc_verity.sh` with `RED_MODE=0` to prove slot switch + dm-verity active.

**Canonical authority for the bundle-build procedure:** `tests/emulator/ab_virt/rauc/README.md` + `docs/design/rk3588_ab_virt/PWU_AB_2_RAUC_VERITY.md`.
