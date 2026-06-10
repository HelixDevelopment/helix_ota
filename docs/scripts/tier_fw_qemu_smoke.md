# tier_fw_qemu_smoke.sh — QEMU aarch64 firmware boot smoke (emulation-fabric P2 / Tfw tier)

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-06-10T15:35:00Z |
| Status | active |

## Overview

First deliverable of emulation-fabric **P2 (Tfw tier)** per
[`../design/emulation_fabric/ROADMAP.md`](../design/emulation_fabric/ROADMAP.md)
(and the Tfw row of [`../design/emulation_fabric/DESIGN.md`](../design/emulation_fabric/DESIGN.md)):
prove the firmware/Linux-target tier is operational on the macOS dev host by
booting a generic **aarch64 `-machine virt`** target under
`qemu-system-aarch64` with **edk2 UEFI** firmware (TCG by default; HVF accel
optional) and asserting a **real boot milestone string** captured from the
serial console — the edk2/UEFI firmware banner, a U-Boot prompt, or a Linux
kernel boot string. This is the boot floor that the `containers/pkg/vm` U-Boot
slot-switch logic builds on (REPORT §3.1).

It is **not** an Android A/B tier: `-machine virt` is a generic ARMv8 board, not
an RK3588 SoC model (no Rockchip clocks/PMIC/Mali) — REPORT §3.1 + §8 honest gap.

## Prerequisites

- `qemu-system-aarch64` — `brew install qemu`.
- An **edk2 aarch64 UEFI** firmware blob. The qemu Homebrew formula ships one at
  `$(brew --prefix qemu)/share/qemu/edk2-aarch64-code.fd`; the script auto-detects
  it (and the common `QEMU_EFI.fd` / `AAVMF_CODE.fd` names) under the qemu share
  dirs, or accepts `QEMU_EFI=/path/...`.
- macOS host (REPORT §0): TCG always works; HVF is optional acceleration
  (`QEMU_ACCEL=hvf`).

## Usage

```bash
bash tests/emulator/tier_fw_qemu_smoke.sh
QEMU_EFI=/opt/homebrew/share/qemu/edk2-aarch64-code.fd bash tests/emulator/tier_fw_qemu_smoke.sh
BOOT_TIMEOUT=45 QEMU_ACCEL=hvf bash tests/emulator/tier_fw_qemu_smoke.sh
```

Env inputs (all optional): `QEMU_BIN`, `QEMU_EFI`, `BOOT_TIMEOUT` (default 60s),
`QEMU_MEM` (default 512M — bounded per §12.6), `QEMU_ACCEL` (default `tcg`).

## Anti-bluff contract (§11.4 / §11.4.3)

- **PASS (exit 0)** requires a **real milestone string** captured from the serial
  log of an actually-launched qemu process — matched case-insensitively against
  `UEFI Interactive Shell | EDK II | BdsDxe | TianoCore | U-Boot 20 | Hit any key
  to stop | Booting Linux | Linux version | Synchronous Exception`. Never a
  metadata-only or exit-code-only pass.
- **SKIP (exit 3)** when a prereq is **genuinely absent** (qemu not installed, or
  no UEFI firmware blob) — prints exactly what is missing + the install command.
  This is the honest §11.4.3 fallback, NOT a fake pass.
- **FAIL (exit 1)** when qemu launched but no milestone appeared within the
  timeout (cites the serial + stdout log paths).

Evidence (under `docs/qa/<run-id>-qemu-fw-smoke/`): `qemu_serial.log` (the guest
console capture), `qemu_stdout.log` (accel evidence), and `boot_smoke_result.txt`
(qemu/efi/accel/booted/milestone + first serial lines).

## Edge cases / internal behaviour

- **`qemu --prefix` is not `qemu installed`:** `brew --prefix qemu` reports the
  formula's would-be location even when qemu is NOT installed; the script keys off
  the actual binary (`command -v` / `$QEMU_BIN` executable check), not the prefix —
  §11.4.6 captured as FACT (this host: qemu absent → SKIP).
- **No disk image:** the firmware boots and (finding nothing bootable) lands at the
  edk2 UEFI shell / boot-failure prompt — both emit the firmware banner on the
  serial console, which is the asserted milestone. `-no-reboot` stops a boot-fail
  reboot loop.
- **Cleanup (§11.4.14 / §12):** the script captures the qemu pid (`$!`) and ALWAYS
  kills **only that pid** on exit (graceful `kill`, then `kill -9` after a bounded
  wait) — NEVER a broad `pkill qemu.*`, which could touch podman's qemu or any
  other qemu instance.
- **Bounded resource use:** 512M guest RAM by default (§12.6 host-memory ceiling);
  single short-lived process.

## Related

- `docs/research/emulation_infra/REPORT.md` (§0 host fact, §3.1 QEMU virt+edk2 verdict, §8 RK3588-SoC gap)
- `docs/design/emulation_fabric/{DESIGN,ROADMAP}.md` (Tfw row / P2 milestone)
- `tests/emulator/tier1_avd_hvf_smoke.sh` (sibling P1/T1 AVD-on-HVF smoke)
- `containers/pkg/vm` (QEMU aarch64 `virt`+UEFI orchestration the Tfw tier feeds)

Last verified: 2026-06-10 — SKIP (exit 3) on this host (qemu not installed; honest
prereq-absent skip, evidence written). Boot/milestone/cleanup path independently
exercised via a fake-qemu harness emitting a real edk2 banner: PASS on the captured
milestone + the live pid killed with no leak (harness probe-dir not committed).
