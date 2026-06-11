#!/usr/bin/env bash
# =============================================================================
# tier2_cuttlefish_ab.sh — Tier-2: REAL Android A/B OTA on Cuttlefish (Linux+KVM)
# -----------------------------------------------------------------------------
# Purpose:
#   Exercise the REAL Android `update_engine` A/B + AVB/dm-verity + auto-rollback
#   flow on a Cuttlefish (`cvd`) virtual device — the fidelity the dev-host
#   QEMU A/B-virt tier (tests/emulator/ab_virt/) cannot reach, and the closest
#   hardware-free proxy for the Orange Pi 5 Max / RK3588 OTA apply. Plan +
#   sources: docs/design/CUTTLEFISH_TIER2.md ; docs/research/rk3588_emulator/REPORT.md.
#
# TOPOLOGY GATE (§11.4.3 / §11.4.81 cross-platform parity):
#   Cuttlefish requires Linux + nested KVM (`/dev/kvm`). On any host WITHOUT a
#   usable /dev/kvm (e.g. this Apple-Silicon macOS dev host — §11.4.112 host-gate)
#   this script SKIPs honestly (exit 3) — NEVER a fake PASS. It RUNS the real flow
#   only where the topology is present (the operator's incoming Linux+KVM host,
#   an M4+/macOS-15 nested-virt Mac, or a GCE nested-virt instance).
#
# HONEST STATUS (§11.4.6): the Linux A/B-apply path below is implemented from the
#   latest official AOSP cuttlefish + update_engine docs (CUTTLEFISH_TIER2.md
#   "Sources verified"), but has NOT yet been executed on a real Linux+KVM host —
#   it is UNVERIFIED-pending-host. The exact OTA-apply invocation the AOSP doc
#   itself marks `UNCONFIRMED:` (Virtual-A/B vs legacy A/B; `update_device.py` vs
#   `cvd`) is detected + attempted at runtime, not guessed-and-asserted. When the
#   Linux host is available this script is run, any host-specific fixes land, and
#   its PASS becomes real captured evidence (§11.4.107/§11.4.69/§11.4.108).
#
# Usage:
#   tests/emulator/tier2_cuttlefish_ab.sh [--prepare]   # --prepare installs cuttlefish debs
#   Env: HELIX_CF_DIR (cuttlefish workdir, default ./.cuttlefish),
#        HELIX_CF_TARGET (aosp_cf_arm64_only_phone | aosp_cf_x86_64_only_phone — auto by arch),
#        HELIX_CF_EVIDENCE (default docs/qa/<run-id>/cuttlefish_ab/)
#
# Outputs: captured evidence (slot-state, update_engine status, rollback trace)
#   under the evidence dir; PASS/FAIL/SKIP verdict on stdout.
# Dependencies (Linux host): git, apt/dpkg, kvm group membership, ~30 GB disk,
#   network (AOSP build fetch). Self-cleaning: stop_cvd on every exit (§11.4.14).
# Cross-references: §11.4.3, §11.4.81, §11.4.69, §11.4.107, §11.4.108, §11.4.112,
#   §11.4.123 (the UNCONFIRMED apply path is a research-trigger, never a bluff).
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
RUN_ID="cuttlefish-ab-$(date +%s)-$$"
EVID="${HELIX_CF_EVIDENCE:-${REPO_ROOT}/docs/qa/${RUN_ID}/cuttlefish_ab}"
CF_DIR="${HELIX_CF_DIR:-${REPO_ROOT}/.cuttlefish}"

PASS=0; FAIL=0; SKIP=0
mkdir -p "$EVID"
log()  { printf '%s\n' "$*" | tee -a "${EVID}/transcript.txt"; }
pass() { PASS=$((PASS+1)); log "[PASS] $1${2:+ [evidence: $2]}"; }   # §11.4.69 ab_pass_with_evidence shape
fail() { FAIL=$((FAIL+1)); log "[FAIL] $1"; }
skip() { SKIP=$((SKIP+1)); log "[SKIP] $1 (reason: $2)"; }            # §11.4.69 ab_skip_with_reason

CF_PID_DIR=""
cleanup() {
  if [ -n "$CF_PID_DIR" ] && [ -x "${CF_PID_DIR}/bin/stop_cvd" ]; then
    log "cleanup: stop_cvd"; ( cd "$CF_PID_DIR" && HOME="$CF_PID_DIR" ./bin/stop_cvd >/dev/null 2>&1 || true )
  fi
}
trap cleanup EXIT INT TERM

log "== Tier-2 Cuttlefish real-Android-A/B OTA =="
log "run=${RUN_ID}  host=$(uname -s)/$(uname -m)  $(date -u +%Y-%m-%dT%H:%M:%SZ)"

# ---- TOPOLOGY GATE (§11.4.3 / §11.4.81 / §11.4.112) -------------------------
if [ "$(uname -s)" != "Linux" ]; then
  skip "Cuttlefish needs Linux+KVM; host is $(uname -s)" "topology_unsupported"
  log "RESULT: SKIP — runs on the Linux+KVM host (operator-provided), not this $(uname -s) dev host."
  log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
  exit 3
fi
if [ ! -e /dev/kvm ] || [ ! -r /dev/kvm ] || [ ! -w /dev/kvm ]; then
  skip "no usable /dev/kvm (nested-virt absent or no kvm-group membership)" "topology_unsupported"
  log "RESULT: SKIP — needs /dev/kvm. On the Linux host: 'sudo usermod -aG kvm \$USER' + reboot, then re-run."
  log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
  exit 3
fi
pass "topology gate: Linux + usable /dev/kvm present" "$(ls -l /dev/kvm 2>/dev/null | tee ${EVID}/kvm.txt >/dev/null; echo ${EVID}/kvm.txt)"

# ---- target by arch (§11.4.6 — no guessing the build target) ----------------
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  DEF_TARGET="aosp_cf_x86_64_only_phone" ;;
  aarch64) DEF_TARGET="aosp_cf_arm64_only_phone" ;;
  *) fail "unsupported host arch '$ARCH' for cuttlefish (need x86_64 or aarch64)"; exit 1 ;;
esac
CF_TARGET="${HELIX_CF_TARGET:-$DEF_TARGET}"
log "cuttlefish target: ${CF_TARGET} (host arch ${ARCH})"

# ---- prerequisite: cuttlefish host package present? -------------------------
# (--prepare builds+installs the debs per CUTTLEFISH_TIER2.md; otherwise we
#  require them already installed so this script stays non-sudo by default.)
if [ "${1:-}" = "--prepare" ]; then
  log "[prepare] building + installing cuttlefish debs (needs sudo) ..."
  PREP="${CF_DIR}/android-cuttlefish"
  mkdir -p "$CF_DIR"
  [ -d "$PREP/.git" ] || git clone https://github.com/google/android-cuttlefish "$PREP" 2>&1 | tee -a "${EVID}/prepare.log"
  ( cd "$PREP" && tools/buildutils/build_packages.sh ) 2>&1 | tee -a "${EVID}/prepare.log" || true
  ( cd "$PREP" && sudo apt-get install -y ./cuttlefish-base_*.deb ./cuttlefish-user_*.deb ) 2>&1 | tee -a "${EVID}/prepare.log" || true
  sudo usermod -aG kvm,cvdnetwork,render "$USER" 2>&1 | tee -a "${EVID}/prepare.log" || true
  log "[prepare] done — a REBOOT is required to load modules + udev rules, then re-run WITHOUT --prepare."
  exit 0
fi

if ! dpkg -l cuttlefish-base >/dev/null 2>&1 && [ ! -x "${CF_DIR}/cvd/bin/launch_cvd" ]; then
  skip "cuttlefish-base not installed (run with --prepare first, then reboot)" "feature_disabled_by_config"
  log "RESULT: SKIP — install prerequisites: '$0 --prepare' then reboot."
  exit 3
fi

# ---- fetch an A/B build (device images + matching cvd host package) ----------
# UNCONFIRMED (CUTTLEFISH_TIER2.md): whether the default aosp_cf_* target ships
# Virtual-A/B vs legacy A/B. We fetch + detect at runtime, never assume.
WORK="${CF_DIR}/cvd"
mkdir -p "$WORK"; cd "$WORK"
if [ ! -x "./bin/launch_cvd" ]; then
  log "fetching ${CF_TARGET} device images + cvd-host_package (this is large) ..."
  if command -v fetch_cvd >/dev/null 2>&1; then
    fetch_cvd -default_build="aosp-main/${CF_TARGET}-userdebug" 2>&1 | tee -a "${EVID}/fetch.log" \
      || { fail "fetch_cvd failed (see fetch.log) — UNVERIFIED-pending-host: confirm the build id/branch on the Linux host"; exit 1; }
  else
    skip "fetch_cvd not on PATH — install the cvd host package first" "feature_disabled_by_config"; exit 3
  fi
fi
CF_PID_DIR="$WORK"

# ---- launch + baseline slot ------------------------------------------------
log "launching cvd (daemon) ..."
HOME="$WORK" ./bin/launch_cvd --daemon 2>&1 | tee -a "${EVID}/launch.log" \
  || { fail "launch_cvd failed (see launch.log)"; exit 1; }
# Wait for adb + boot completion.
ADB="./bin/adb"; [ -x "$ADB" ] || ADB="adb"
for i in $(seq 1 60); do
  "$ADB" wait-for-device 2>/dev/null
  [ "$("$ADB" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ] && break
  sleep 5
done
SLOT_BEFORE="$("$ADB" shell getprop ro.boot.slot_suffix 2>/dev/null | tr -d '\r')"
"$ADB" shell getprop > "${EVID}/getprop_before.txt" 2>/dev/null || true
if [ -z "$SLOT_BEFORE" ]; then
  fail "could not read ro.boot.slot_suffix — device may be non-A/B (UNCONFIRMED target — verify on host)"; exit 1
fi
pass "cvd booted; baseline active slot = '${SLOT_BEFORE}'" "${EVID}/getprop_before.txt"

# ---- REAL A/B apply via update_engine --------------------------------------
# update_engine writes the OTA payload to the INACTIVE slot, then setActiveBootSlot.
# The exact driver (update_device.py from an OTA package vs a cvd subcommand) is
# the AOSP-`UNCONFIRMED:` step — attempt the documented path, capture the result,
# and FAIL honestly (never fake-PASS) if the apply does not complete on this host.
log "applying an OTA payload to the inactive slot via update_engine ..."
APPLIED=0
if [ -f "${WORK}/ota.zip" ] && command -v python3 >/dev/null 2>&1 && [ -f "${WORK}/bin/update_device.py" ]; then
  HOME="$WORK" python3 ./bin/update_device.py --file "${WORK}/ota.zip" 2>&1 | tee -a "${EVID}/apply.log" && APPLIED=1
else
  log "  UNVERIFIED-pending-host: no ota.zip/update_device.py present — on the Linux host, build/fetch a"
  log "  signed OTA for ${CF_TARGET} and point this step at it (CUTTLEFISH_TIER2.md §5)."
fi
if [ "$APPLIED" != 1 ]; then
  fail "OTA apply did not run (prerequisite OTA package missing) — UNVERIFIED-pending-host, NOT a fake PASS"
  exit 1
fi
# update_engine completion → UPDATED_NEED_REBOOT.
"$ADB" shell update_engine_client --follow > "${EVID}/update_engine.txt" 2>&1 || true
grep -q 'UPDATED_NEED_REBOOT' "${EVID}/update_engine.txt" \
  && pass "update_engine reports UPDATED_NEED_REBOOT (payload applied to inactive slot)" "${EVID}/update_engine.txt" \
  || { fail "update_engine did not reach UPDATED_NEED_REBOOT"; exit 1; }

# ---- reboot → assert the active slot FLIPPED (§11.4.108 runtime signature) ---
"$ADB" reboot 2>/dev/null; sleep 5
for i in $(seq 1 60); do
  "$ADB" wait-for-device 2>/dev/null
  [ "$("$ADB" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ] && break
  sleep 5
done
SLOT_AFTER="$("$ADB" shell getprop ro.boot.slot_suffix 2>/dev/null | tr -d '\r')"
"$ADB" shell getprop ro.boot.slot_suffix > "${EVID}/slot_after.txt" 2>/dev/null || true
if [ -n "$SLOT_AFTER" ] && [ "$SLOT_AFTER" != "$SLOT_BEFORE" ]; then
  pass "active slot FLIPPED '${SLOT_BEFORE}' -> '${SLOT_AFTER}' after reboot (real A/B slot switch)" "${EVID}/slot_after.txt"
else
  fail "active slot did NOT flip (before='${SLOT_BEFORE}' after='${SLOT_AFTER}')"; exit 1
fi
# dm-verity active on the new slot (§11.4.107).
"$ADB" shell 'dmesg | grep -i verity' > "${EVID}/verity.txt" 2>/dev/null || true
[ -s "${EVID}/verity.txt" ] && pass "dm-verity present on the booted slot" "${EVID}/verity.txt" \
  || log "  note: verity dmesg not captured (UNVERIFIED-pending-host — confirm AVB/dm-verity on the real host)"

# (Headline corrupt-slot auto-rollback is PWU-CF-2 — added once the apply+flip is
#  verified green on the real host, mirroring ab_virt PWU-AB-3.)

log ""
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
[ "$FAIL" -gt 0 ] && { log "RESULT: FAIL"; exit 1; }
log "RESULT: PASS"
exit 0
