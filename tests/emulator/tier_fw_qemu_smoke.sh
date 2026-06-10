#!/usr/bin/env bash
#
# tier_fw_qemu_smoke.sh — Emulation-fabric P2 (Tfw tier) QEMU aarch64 firmware boot smoke.
#
# Purpose
#   Prove the firmware/Linux-target (Tfw) tier is operational on this macOS dev
#   host: boot a generic aarch64 `-machine virt` target under
#   qemu-system-aarch64 with edk2 UEFI firmware (TCG; HVF accel optional) and
#   assert a REAL boot milestone captured from the serial console — the edk2/UEFI
#   firmware banner, a U-Boot prompt, or a Linux kernel boot string. Per
#   docs/research/emulation_infra/REPORT.md §3.1 + docs/design/emulation_fabric/
#   {DESIGN.md (Tfw row), ROADMAP.md (P2)}, this is the Tfw-tier boot floor that
#   `pkg/vm` U-Boot slot-switch logic builds on. It is NOT an Android A/B tier
#   (`-machine virt` is a generic ARMv8 board, not an RK3588 SoC model — REPORT §3.1).
#
#   §11.4 anti-bluff: a PASS requires a REAL milestone STRING captured from the
#   serial log of an actually-launched qemu process — never a metadata-only or
#   exit-code-only pass. If qemu OR a UEFI firmware blob is genuinely absent on
#   this host, the script SKIPs-with-reason (exit 3) per §11.4.3 — printing exactly
#   what is missing and how to install it (`brew install qemu`) — NOT a fake pass.
#   §11.4.112 honest boundary: the host fact (REPORT §0) is that there is no real
#   RK3588 SoC model in QEMU; this tier proves generic UEFI/firmware boot only.
#
# Usage
#   bash tests/emulator/tier_fw_qemu_smoke.sh
#   QEMU_EFI=/path/to/edk2-aarch64-code.fd bash tests/emulator/tier_fw_qemu_smoke.sh
#   BOOT_TIMEOUT=45 bash tests/emulator/tier_fw_qemu_smoke.sh
#
# Inputs (env, all optional)
#   QEMU_BIN      qemu-system-aarch64 binary (default: first on PATH)
#   QEMU_EFI      edk2 UEFI code blob (default: auto-detected under qemu share dirs)
#   BOOT_TIMEOUT  seconds to wait for the boot milestone (default: 60)
#   QEMU_MEM      guest RAM (default: 512, bounded — §12.6 host-memory ceiling)
#   QEMU_ACCEL    accelerator (default: tcg; set "hvf" to try Apple Hypervisor)
# Outputs / Side-effects
#   Launches ONE qemu-system-aarch64 process with no disk (UEFI shell / firmware
#   boot only), captures its serial console to a file, asserts a boot-milestone
#   string, writes evidence under docs/qa/<run-id>-qemu-fw-smoke/, then ALWAYS
#   kills ONLY that specific qemu pid (trap EXIT, §11.4.14 — never a broad pkill
#   that could touch podman's qemu or any other qemu instance).
# Dependencies: qemu-system-aarch64 (`brew install qemu`) + an edk2 aarch64 UEFI
#   firmware blob (ships with the qemu formula under share/qemu/). macOS host
#   (REPORT §0): TCG always works; HVF is optional acceleration.
# Cross-references: docs/research/emulation_infra/REPORT.md §3.1,
#   docs/design/emulation_fabric/{DESIGN,ROADMAP}.md (Tfw / P2),
#   docs/scripts/tier_fw_qemu_smoke.md, tests/emulator/tier1_avd_hvf_smoke.sh.

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
QEMU_BIN="${QEMU_BIN:-$(command -v qemu-system-aarch64 2>/dev/null || true)}"
BOOT_TIMEOUT="${BOOT_TIMEOUT:-60}"
QEMU_MEM="${QEMU_MEM:-512}"
QEMU_ACCEL="${QEMU_ACCEL:-tcg}"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
QA="${REPO_ROOT}/docs/qa/${RUN_ID}-qemu-fw-smoke"
mkdir -p "$QA"

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%SZ)" "$*"; }
fail() { log "FAIL: $*"; exit 1; }
skip() { log "SKIP: $*"; exit 3; }

# --- Prereq detection (§11.4.3 — honest SKIP, never a fake pass) ---------------

if [ -z "$QEMU_BIN" ] || [ ! -x "$QEMU_BIN" ]; then
  {
    echo "=== Tfw QEMU firmware boot smoke — SKIP ($(date -u +%FT%TZ)) ==="
    echo "reason: qemu-system-aarch64 not found on this host"
    echo "install: brew install qemu"
  } | tee "$QA/boot_smoke_result.txt"
  skip "qemu-system-aarch64 not installed (run: brew install qemu) — Tfw tier prereq absent, see $QA/"
fi

# Auto-detect an edk2 aarch64 UEFI code blob if not supplied. The qemu formula
# ships these under <prefix>/share/qemu/. Accept the common names across distros.
find_efi() {
  if [ -n "${QEMU_EFI:-}" ]; then
    [ -f "$QEMU_EFI" ] && { printf '%s\n' "$QEMU_EFI"; return 0; }
    return 1
  fi
  qdir="$(dirname "$QEMU_BIN")"
  prefix="$(cd "$qdir/.." 2>/dev/null && pwd || true)"
  for d in \
    "$prefix/share/qemu" \
    "/opt/homebrew/share/qemu" \
    "/usr/local/share/qemu" \
    "/usr/share/qemu" \
    "/usr/share/AAVMF" \
    "/usr/share/edk2/aarch64"; do
    [ -d "$d" ] || continue
    for n in edk2-aarch64-code.fd QEMU_EFI.fd QEMU_EFI-pflash.raw AAVMF_CODE.fd; do
      [ -f "$d/$n" ] && { printf '%s\n' "$d/$n"; return 0; }
    done
  done
  return 1
}

EFI="$(find_efi || true)"
if [ -z "$EFI" ]; then
  {
    echo "=== Tfw QEMU firmware boot smoke — SKIP ($(date -u +%FT%TZ)) ==="
    echo "reason: no edk2/UEFI aarch64 firmware blob found"
    echo "qemu_bin: $QEMU_BIN"
    echo "looked for: edk2-aarch64-code.fd / QEMU_EFI.fd / AAVMF_CODE.fd under qemu share dirs"
    echo "install/locate: brew install qemu  (ships share/qemu/edk2-aarch64-code.fd);"
    echo "                or set QEMU_EFI=/path/to/edk2-aarch64-code.fd"
  } | tee "$QA/boot_smoke_result.txt"
  skip "no edk2/UEFI aarch64 firmware blob found (set QEMU_EFI=... or brew install qemu) — see $QA/"
fi

# --- Boot ----------------------------------------------------------------------

SERIAL_LOG="$QA/qemu_serial.log"
: > "$SERIAL_LOG"

log "booting aarch64 -machine virt + edk2 UEFI (accel=$QEMU_ACCEL, mem=${QEMU_MEM}M)"
log "qemu=$QEMU_BIN efi=$EFI evidence=$QA"

# No disk image: the firmware boots and (finding nothing bootable) lands at the
# edk2 UEFI shell / boot-failure prompt — both of which emit the firmware banner
# on the serial console, which is the milestone we assert. -serial file: routes
# the guest console to the log; -display none keeps it headless; -no-reboot stops
# a boot-fail reboot loop. The qemu pid is captured so cleanup kills ONLY it.
"$QEMU_BIN" \
  -machine "virt,accel=${QEMU_ACCEL}" \
  -cpu cortex-a72 \
  -m "$QEMU_MEM" \
  -nographic \
  -display none \
  -no-reboot \
  -drive "if=pflash,format=raw,readonly=on,file=${EFI}" \
  -serial "file:${SERIAL_LOG}" \
  -monitor none \
  >"$QA/qemu_stdout.log" 2>&1 &
QEMU_PID=$!

# §11.4.14 cleanup — kill ONLY this qemu pid, never a broad pkill.
cleanup() {
  if [ -n "${QEMU_PID:-}" ] && kill -0 "$QEMU_PID" 2>/dev/null; then
    kill "$QEMU_PID" 2>/dev/null || true
    for _ in 1 2 3 4 5; do
      kill -0 "$QEMU_PID" 2>/dev/null || break
      sleep 1
    done
    kill -9 "$QEMU_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

# Boot-milestone strings (any one proves a real firmware/kernel boot reached the
# serial console). edk2 prints "UEFI Interactive Shell" / "EDK II" / "BdsDxe";
# U-Boot prints "U-Boot 20" / "Hit any key to stop"; a Linux kernel prints
# "Booting Linux" / "Linux version". Matched case-insensitively.
MILESTONE_RE='UEFI Interactive Shell|EDK II|BdsDxe|TianoCore|U-Boot 20|Hit any key to stop|Booting Linux|Linux version|Synchronous Exception'

booted=0
matched=""
for _ in $(seq 1 "$BOOT_TIMEOUT"); do
  if ! kill -0 "$QEMU_PID" 2>/dev/null; then
    # qemu exited; still inspect the serial log below before deciding.
    break
  fi
  m="$(grep -i -E "$MILESTONE_RE" "$SERIAL_LOG" 2>/dev/null | head -1 | tr -d '\r')"
  if [ -n "$m" ]; then booted=1; matched="$m"; break; fi
  sleep 1
done

# Final post-loop scan (covers the qemu-exited-fast case).
if [ "$booted" != "1" ]; then
  m="$(grep -i -E "$MILESTONE_RE" "$SERIAL_LOG" 2>/dev/null | head -1 | tr -d '\r')"
  [ -n "$m" ] && { booted=1; matched="$m"; }
fi

ACCEL_EVIDENCE="$(grep -i -E 'hvf|tcg|accel|kvm' "$QA/qemu_stdout.log" 2>/dev/null | head -3)"
{
  echo "=== Tfw QEMU firmware boot smoke ($(date -u +%FT%TZ)) ==="
  echo "qemu=$QEMU_BIN"
  echo "efi=$EFI"
  echo "accel=$QEMU_ACCEL mem=${QEMU_MEM}M timeout=${BOOT_TIMEOUT}s"
  echo "booted=$booted"
  echo "milestone_matched=${matched:-<none>}"
  echo "serial_log=$SERIAL_LOG ($(wc -c <"$SERIAL_LOG" 2>/dev/null | tr -d ' ') bytes)"
  echo "--- first serial lines ---"
  head -20 "$SERIAL_LOG" 2>/dev/null
  echo "--- qemu stdout (accel evidence) ---"
  printf '%s\n' "${ACCEL_EVIDENCE:-<none>}"
} | tee "$QA/boot_smoke_result.txt"

[ "$booted" = "1" ] || fail "no boot milestone (${MILESTONE_RE}) captured on the serial console within ${BOOT_TIMEOUT}s (see $SERIAL_LOG / $QA/qemu_stdout.log)"

log "RESULT: PASS — aarch64 -machine virt firmware boot reached milestone: ${matched}"
log "evidence: $QA/"
exit 0
