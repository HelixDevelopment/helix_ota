# RK3588 Hardware Validation Plan — Helix OTA A/B Slot Writer + fw_setenv

| Field | Value |
|---|---|
| Document | rk3588_hardware_validation_plan.md |
| Status | DRAFT — unblocks OTA-003, OTA-004, OTA-038, OTA-041, OTA-042, OTA-043 |
| Created | 2026-07-26 |
| Hardware | Orange Pi 5 Max (RK3588), arrive date TBD |
| Pre-built verified on | QEMU aarch64 (qemu-system-aarch64 9.1.2) |

## Summary

This document enumerates the specific hardware validation tests to execute
when the RK3588 physical board arrives. The slot-writer (`device.SlotWriter`),
U-Boot environment manager (`device.UBootEnvManager`), and ApplyPort flow
(`device.ApplyPort.WriteAndArm`) have all been pre-built and verified against
a QEMU aarch64 disk-image model (see `server/tests/qemu_arm/ab_slot_test.go`).

The hardware tests PROVE the QEMU-verified logic translates faithfully to
the real flash layout, GPT partition table, raw U-Boot env region, and
bootloader behaviour — the Final Confirmation step per §11.4.185.

## Q1-A Decision (2026-07-26)

**Write code against QEMU aarch64 emulation NOW. Hardware arrival validates
pre-built code later.** The QEMU test suite (`server/tests/qemu_arm/`) exercises
the full ApplyPort stack against a synthetic GPT disk image with 128 MB slot
partitions and a raw 64-KiB U-Boot env blob. All 10 tests pass (10/10 GREEN).

## Test Matrix

### 1. GPT Partition Table Conformance

**QEMU equivalent:** `TestQemuDiskImageGPTValid` — verifies MBR boot signature
(0x55 0xAA) and GPT header signature ("EFI PART").

**Hardware test:**
- Boot the RK3588 board into U-Boot shell
- Run `mmc part` or `gpt enumerate mmc 0` and verify:
  - Partition 1: U-Boot / EFI
  - Partition 2: slot-a (rootfs A)
  - Partition 3: slot-b (rootfs B)
  - Partition 4: data
- Verify `helix_slot=A` in bootargs (for initial flash)
- Run `ls mmc 0:2` and `ls mmc 0:3` to confirm both rootfs partitions accessible

### 2. Slot Detection (/proc/cmdline + /etc/slot_id)

**QEMU equivalent:** `TestApplyPort_WriteAndArm_QemuBlock` — writes to inactive
slot based on active slot detection.

**Hardware test:**
- Boot all the way to Linux userspace
- Verify `/proc/cmdline` contains `helix_slot=A`
- Verify `/etc/slot_id` contains `A`
- Run the slot-detection unit: `ActiveSlot()` → "A", `InactiveSlot()` → "B"
- Swap to slot B via U-Boot env, reboot, verify slot flips to "B"

### 3. Slot Writer — dd Write to Inactive Partition

**QEMU equivalent:** `TestQemuSlotWriter_WriteInactiveSlot` — writes rootfs image
to the inactive slot offset and verifies bytes at correct offset.

**Hardware test:**
- Prepare a test rootfs image (small tarball, ~1-10 MB)
- Run: `dd if=./test-rootfs.img of=/dev/mmcblk0p3 bs=4M conv=fsync status=progress`
  (assuming slot A is active, so slot B = /dev/mmcblk0p3)
- Mount `/dev/mmcblk0p3` and verify the content matches
- Repeat with slot B active → writes to `/dev/mmcblk0p2`
- Verify the active slot partition is NOT touched

### 4. U-Boot Env Manager — fw_setenv / fw_printenv

**QEMU equivalent:** `TestQemuUBootEnv_RoundTrip` — sets and reads env vars
against a raw 64-KiB CRC-protected blob.

**Hardware test:**
- Install `u-boot-tools` (libubootenv) on the board
- Place the correct `/etc/fw_env.config` matching the U-Boot env offset:
  ```
  # /etc/fw_env.config — RK3588 eMMC layout
  /dev/mmcblk0    0x88000    0x4000
  ```
  (Verify against `fw_printenv -c /etc/fw_env.config printenv` from U-Boot)
- Round-trip test:
  1. `fw_printenv BOOT_ORDER` → read current value
  2. `fw_setenv BOOT_ORDER "B A"` → set new value
  3. `fw_printenv BOOT_ORDER` → verify "B A"
  4. Reboot → `fw_printenv BOOT_ORDER` → still "B A" (persistence check)
- Set `upgrade_available=1`, `bootcount=1` and verify all three vars survive reboot

### 5. ApplyPort Full WriteAndArm (steps b+c)

**QEMU equivalent:** `TestApplyPort_WriteAndArm_QemuBlock` — writes rootfs,
sets BOOT_ORDER, upgrade_available, bootcount, and verifies all.

**Hardware test:**
- From slot A active, run the real `WriteAndArm` against a test rootfs image:
  - Writes rootfs to `/dev/mmcblk0p3` (slot B)
  - Sets: `BOOT_ORDER=B A`, `upgrade_available=1`, `bootcount=1`
- Verify with `fw_printenv` that all three vars are set
- Reboot
- U-Boot should select slot B (head of BOOT_ORDER)
- Verify `cat /proc/cmdline` shows `helix_slot=B` after boot

### 6. A/B Slot Toggle — Full Uptake Sequence

**QEMU equivalent:** `TestQemuUptakeSequence` — v1→v2→v3 across alternating
slots with healthy-boot confirmation.

**Hardware test:**
- Phase 1: Flash v1.0.0 to slot A, boot slot A
- Phase 2: Deploy v2.0.0 → writes to slot B, arms env, reboot → slot B boots v2.0.0
  - `uname -r` or version file confirms v2.0.0
- Phase 3: Deploy v3.0.0 → writes to slot A, arms env, reboot → slot A boots v3.0.0
- Phase 4: Healthy-boot confirm clears upgrade_available and bootcount
  - Run: `helix-ab-confirm` (or equivalent)
  - Verify: `fw_printenv upgrade_available` → "0"
  - Verify: `fw_printenv bootcount` → "0"

### 7. Edge Cases

| Case | QEMU test | Hardware check |
|---|---|---|
| Oversize image rejected | `TestQemuSlotWriter_OversizeImage` | Attempt dd of 200MB image into 128MB partition — must fail gracefully |
| Empty image rejected | `TestQemuSlotWriter_EmptyImage` | Attempt dd of zero-byte file — must fail |
| CRC corruption detected | `TestQemuUBootEnv_CRC_Corruption` | Manually corrupt env blob with busybox hexdump+dd, verify fw_printenv fails |
| Delete and recreate env var | `TestQemuEnvManager_DeleteAndRecreate` | fw_setenv key "" (delete), verify fw_printenv returns "" |

### 8. fw_env.config Verification (PWU-AB-4 §3)

This is the BLOCKING verification item (§11.4.6):

- The `/etc/fw_env.config` must match U-Boot's `CONFIG_ENV_OFFSET` and
  `CONFIG_ENV_SIZE` EXACTLY. A mismatch → fw_setenv writes to wrong flash
  region → bootloader never reads the vars → A/B switch silently fails.
- Verification procedure:
  1. Read U-Boot env offset/size from the built u-boot.bin:
     `strings u-boot.bin | grep -E "env_offset|env_size|CONFIG_ENV"` OR
     check the `.config` used to build U-Boot
  2. Confirm `/etc/fw_env.config` on the running board matches
  3. Round-trip: `fw_setenv <key> <val>` → reboot into U-Boot shell →
     `printenv <key>` → value matches
  4. If ANY drift: the config file is WRONG and must be corrected before
     ANY A/B slot switching can be trusted

## QEMU Test Suite Summary

| Test | Status | Description |
|---|---|---|
| `TestQemuSlotWriter_WriteInactiveSlot` | PASS | Writes to inactive slot, verifies bytes at correct partition offset |
| `TestQemuUBootEnv_RoundTrip` | PASS | Sets/reads env vars, verifies CRC-protected persistence |
| `TestQemuUBootEnv_CRC_Corruption` | PASS | Detects CRC32 corruption on env reload |
| `TestApplyPort_WriteAndArm_QemuBlock` | PASS | Full WriteAndArm: write + BOOT_ORDER + upgrade_available + bootcount |
| `TestQemuSlotWriter_OversizeImage` | PASS | Rejects image exceeding partition size |
| `TestQemuDiskImageGPTValid` | PASS | MBR (0x55AA) + GPT ("EFI PART") signatures valid |
| `TestQemuSlotWriter_EmptyImage` | PASS | Rejects empty (0-byte) image |
| `TestQemuEnvManager_DeleteAndRecreate` | PASS | Delete env var by setting to "", re-set works |
| `TestQemuUptakeSequence` | PASS | v1→v2→v3 across alternating slots + healthy-boot confirm |
| `TestQemuQEMUImgInfo` | PASS | qemu-img info confirms raw format + correct size |

**10/10 PASS** on QEMU aarch64 environment (no board required).

## Blocked Items (hardware-gated)

| Item | Unblock condition | QEMU coverage |
|---|---|---|
| OTA-003: fw_env.config verification | Board boots; U-Boot env offset confirmed from u-boot.bin | QEMU tests use raw env format that matches U-Boot's; offset is configurable |
| OTA-004: Real dd slot write to eMMC | Board boots; /dev/mmcblk0 devices present | QEMU uses byte-offset writes; identical semantics |
| OTA-038: Bootloader env round-trip (fw_setenv→reboot→U-Boot printenv) | Board boots; U-Boot shell accessible | QEMU tests CRC persistence and reload |
| OTA-041: Full uptake v1→v2→v3 with real reboot cycles | Board boots; two slot rootfs images available | QEMU tests simulate the entire sequence without reboots |
| OTA-042: A/B toggling survives power-loss | Board boots; can trigger hard reset | QEMU simulates clean reboots; power-loss = test env CRC integrity |
| OTA-043: RAUC integration with dm-verity | Board boots; RAUC bundle built per PWU-AB-2 | QEMU tests the pre-RAUC dd path |

## Procedure

When the RK3588 board arrives:
1. Flash the known-good disk image (from `assemble_ab_disk.sh`)
2. Run `/etc/fw_env.config` verification first (test 8 above) — this is the
   blocking gate
3. If fw_env.config passes, run tests 1–7 in order
4. Capture physical evidence per §11.4.5: photos/video of serial console
   showing each test PASS
5. Update this document with PASS/FAIL status for each test
6. Close OTA-003 through OTA-043 items confirmed by hardware evidence
