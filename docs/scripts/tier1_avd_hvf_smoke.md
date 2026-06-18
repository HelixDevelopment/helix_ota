# tier1_avd_hvf_smoke.sh — AVD-on-HVF boot smoke (emulation-fabric P1 / T1 tier)

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-10T14:45:00Z |
| Status | active |

## Overview

First deliverable of emulation-fabric **P1 (T1 tier)** per
[`../design/emulation_fabric/ROADMAP.md`](../design/emulation_fabric/ROADMAP.md):
prove the only hardware-accelerated Android path on the Apple-Silicon dev host
(REPORT §2.2) is operational by booting an **arm64-v8a** Android AVD headless on
Apple **Hypervisor (HVF)** and asserting a completed boot.

## Prerequisites

- `ANDROID_HOME` Android SDK with `emulator/` + `platform-tools/adb`.
- An installed **arm64-v8a** system image and an AVD (e.g. `Pixel_8`, API 35).
- macOS on Apple Silicon (HVF; there is no `/dev/kvm` — REPORT §0).

## Usage

```bash
bash tests/emulator/tier1_avd_hvf_smoke.sh
AVD=Pixel_8 PORT=5584 BOOT_TIMEOUT=200 bash tests/emulator/tier1_avd_hvf_smoke.sh
```

## Anti-bluff contract (§11.4)

PASS requires BOTH:
1. `sys.boot_completed == 1` (the device actually finished booting), AND
2. `ro.product.cpu.abi == arm64-v8a` — the **acceleration proof**: an x86/x86_64
   abi on this host would mean slow TCG emulation, not HVF, so the tier would not
   be the accelerated path P1 requires.

Evidence (under `docs/qa/<run-id>-avd-hvf-smoke/`): the emulator boot log +
`boot_smoke_result.txt` (avd/serial/booted/abi/sdk + accel log lines).

## Edge cases / internal behaviour

- **`emulator` not on PATH:** the script uses the absolute `$ANDROID_HOME/emulator/emulator`
  (the non-interactive shell does NOT have `emulator` on PATH — the lesson from the
  first attempt; §11.4.6 captured as FACT).
- **Cleanup (§11.4.14 / §12):** ALWAYS tears down via `adb -s emulator-<port> emu kill`
  (serial-targeted, graceful — NEVER a broad `pkill qemu.*`, which could touch
  podman's qemu). The graceful shutdown takes ~20s and continues after the script
  exits; the emulator self-terminates (verified: no orphan, podman untouched).
- **Boot timeout:** `BOOT_TIMEOUT` (default 200s); a non-boot FAILs with the log path.

## Related

- `docs/research/emulation_infra/REPORT.md` (§0 host fact, §2.2 AVD-HVF verdict)
- `docs/design/emulation_fabric/{DESIGN,ROADMAP,TEST_COVERAGE_PLAN}.md`
- Next P1 steps (NOT yet done): build/install the `ota-android-agent` harness APK on
  the booted AVD + drive its decision logic; resolve the `UNCONFIRMED:` GSI-A/B
  real-`update_engine` question (REPORT §2.2 / §8).

Last verified: 2026-06-10 (booted=1, abi=arm64-v8a, sdk=35; clean teardown).
