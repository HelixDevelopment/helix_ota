#!/usr/bin/env bash
#
# tier1_avd_hvf_smoke.sh — Emulation-fabric P1 (T1 tier) AVD-on-HVF boot smoke.
#
# Purpose
#   Prove the ONLY hardware-accelerated Android path on the Apple-Silicon dev host
#   is operational: boot an arm64-v8a Android AVD headless on Apple Hypervisor
#   (HVF) and assert it reaches a completed boot. Per docs/research/emulation_infra/
#   REPORT.md §2.2 + docs/design/emulation_fabric/ROADMAP.md P1, this is the first
#   T1-tier deliverable (the agent-APK + GSI-A/B real-apply steps build on it).
#
#   §11.4 anti-bluff: PASS requires BOTH a real `sys.boot_completed=1` AND
#   `ro.product.cpu.abi == arm64-v8a` — the abi check is the acceleration proof
#   (an x86/x86_64 abi on this host would mean slow TCG, not HVF). Evidence is the
#   captured emulator boot log + the getprop snapshot under docs/qa/<run-id>/.
#
# Usage
#   bash tests/emulator/tier1_avd_hvf_smoke.sh
#   AVD=Pixel_8 PORT=5584 bash tests/emulator/tier1_avd_hvf_smoke.sh
#
# Inputs (env, all optional)
#   ANDROID_HOME  Android SDK root (default: ~/Library/Android/sdk)
#   AVD           AVD name (default: Pixel_8, else the first ~/.android/avd/*.ini)
#   PORT          emulator console port (default: 5584)
#   BOOT_TIMEOUT  seconds to wait for boot_completed (default: 200)
# Outputs / Side-effects
#   Boots a headless emulator, asserts boot+abi, writes evidence under
#   docs/qa/<run-id>-avd-hvf-smoke/, then ALWAYS kills the emulator by its serial
#   (trap EXIT, §11.4.14 — never a broad pkill that could touch podman's qemu).
# Dependencies: ANDROID_HOME/emulator/emulator + platform-tools/adb, an installed
#   arm64-v8a system image + AVD. macOS HVF (no /dev/kvm) — REPORT §0.
# Cross-references: docs/research/emulation_infra/REPORT.md,
#   docs/design/emulation_fabric/{DESIGN,ROADMAP}.md, docs/scripts/tier1_avd_hvf_smoke.md.

set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ANDROID_HOME="${ANDROID_HOME:-$HOME/Library/Android/sdk}"
EMU="${ANDROID_HOME}/emulator/emulator"
ADB="${ANDROID_HOME}/platform-tools/adb"
AVD="${AVD:-Pixel_8}"
PORT="${PORT:-5584}"
SER="emulator-${PORT}"
BOOT_TIMEOUT="${BOOT_TIMEOUT:-200}"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
QA="${REPO_ROOT}/docs/qa/${RUN_ID}-avd-hvf-smoke"
mkdir -p "$QA"

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%SZ)" "$*"; }
fail() { log "FAIL: $*"; exit 1; }

[ -x "$EMU" ] || fail "emulator not found at $EMU (set ANDROID_HOME)"
[ -x "$ADB" ] || fail "adb not found at $ADB"
[ -f "$HOME/.android/avd/${AVD}.ini" ] || AVD="$(ls "$HOME/.android/avd"/*.ini 2>/dev/null | head -1 | xargs -n1 basename 2>/dev/null | sed 's/\.ini$//')"
[ -n "$AVD" ] || fail "no AVD found under ~/.android/avd"

cleanup() { "$ADB" -s "$SER" emu kill >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM

log "booting AVD=$AVD on HVF (headless), serial=$SER, evidence=$QA"
"$EMU" -avd "$AVD" -port "$PORT" -no-window -no-audio -no-snapshot -no-boot-anim \
  -gpu swiftshader_indirect > "$QA/emulator_boot.log" 2>&1 &

booted=0
iters=$(( BOOT_TIMEOUT / 2 ))
for _ in $(seq 1 "$iters"); do
  [ "$("$ADB" -s "$SER" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ] && { booted=1; break; }
  sleep 2
done

ABI="$("$ADB" -s "$SER" shell getprop ro.product.cpu.abi 2>/dev/null | tr -d '\r')"
SDK="$("$ADB" -s "$SER" shell getprop ro.build.version.sdk 2>/dev/null | tr -d '\r')"
{
  echo "=== AVD-HVF boot smoke ($(date -u +%FT%TZ)) ==="
  echo "avd=$AVD serial=$SER booted=$booted abi=$ABI sdk=$SDK"
  echo "accel evidence (emulator boot log):"
  grep -iE 'hvf|hypervisor|accel|Added library|cpuvulkan' "$QA/emulator_boot.log" 2>/dev/null | head -4
} | tee "$QA/boot_smoke_result.txt"

[ "$booted" = "1" ] || fail "AVD did not reach sys.boot_completed within ${BOOT_TIMEOUT}s (see $QA/emulator_boot.log)"
case "$ABI" in
  arm64-v8a) : ;;
  *) fail "abi=$ABI is not arm64-v8a — NOT HVF-accelerated on this host (would be slow TCG); P1 requires the accelerated path" ;;
esac

log "RESULT: PASS — arm64-v8a AVD booted on HVF (booted=1, abi=$ABI, sdk=$SDK)."
log "evidence: $QA/"
exit 0
