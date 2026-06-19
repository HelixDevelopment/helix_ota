# Buildroot Rebuild Report (2026-06-19)

## Build Overview

- **Task:** 40-minute Buildroot rebuild with CONFIG_ENV_IS_IN_FAT, RAUC bundle, libubootenv
- **Script:** `tests/emulator/ab_virt/build_image.sh`
- **Buildroot version:** 2024.02.10
- **Target:** qemu_aarch64_virt_defconfig (aarch64)
- **Build method:** Podman container (Debian bookworm-slim, --arch arm64)

## Attempt History

### Attempt 1 (Build PID 78145)
- **Started:** 2026-06-19 05:55:44 UTC
- **Result:** FAILED after ~35 minutes
- **Root cause:** U-Boot 2024.01 FAT env compilation errors. The config fragment passed `CONFIG_ENV_FAT_INTERFACE`, `CONFIG_ENV_FAT_DEVICE_AND_PART`, and `CONFIG_ENV_FAT_FILE` as Kconfig make options, but these are NOT Kconfig symbols in U-Boot 2024.01 -- they are legacy #define values that belong in the board config header.
- **Error:** `env/fat.c:38:30: error: 'CONFIG_ENV_FAT_INTERFACE' undeclared`

### Attempt 2 (Build PID 69269, worktree)
- **Started:** 2026-06-19 06:49:39 UTC
- **Result:** FAILED at container start (script syntax error)
- **Root cause:** Apostrophe in comment broke single-quoted podman heredoc

### Attempt 3 (Build PID 13465)
- **Started:** 2026-06-19 ~07:44 UTC
- **Duration:** ~18 minutes (until U-Boot build failure)
- **Root cause:** uboot-configure + cat >> patch worked for the configure step, but `make -j$(nproc)` re-extracted U-Boot (configure stamp existed, but build clean triggered re-extract), wiping the manual patch.

### Attempt 4 (Build PID 27988)
- **Started:** 2026-06-19 08:50 UTC
- **Result:** FAILED at config validation
- **Root cause:** `BR2_TARGET_UBOOT_CUSTOM_PATCH_DIR` is legacy/unknown in Buildroot 2024.02

## Current Build (Attempt 5 - Build PID 31171)

- **Started:** 2026-06-19 approximately 09:30 UTC
- **Approach:** uboot-configure extracts U-Boot and runs its kconfig, then the board header is patched before the full build runs. The key fix: uboot-configure NEEDS to create the configure stamp BEFORE the manual patch is applied, so make -j does not re-extract.

## Key Findings

1. **U-Boot 2024.01 Kconfig limitation:** The qemu_arm64 defconfig does not declare CONFIG_ENV_FAT_INTERFACE, CONFIG_ENV_FAT_DEVICE_AND_PART, or CONFIG_ENV_FAT_FILE as Kconfig symbols. They must be legacy C defines in the board config header.
2. **Buildroot stamp system:** Manual patching must happen between uboot-configure and uboot-build stamps.
3. **Heredoc quoting:** build_image.sh uses bash -euo pipefail -c '...' (single-quoted), so all content inside must have no apostrophes.

## Build Status

In progress (PID 31171). Checking build.log.

## Build 5 (Current - PID 33798)
- **Started:** ~09:35 UTC
- **Status:** In progress - passing host-m4 phase at T+2:25
- **U-Boot approach:** uboot-configure + cat >> board header patch
- **Key fixes vs attempt 4:** removed orphaned UOOTPATCH heredoc block, removed BR2_GLOBAL_PATCH_DIR (unknown config), reverted to uboot-configure approach with proper heredoc handling
