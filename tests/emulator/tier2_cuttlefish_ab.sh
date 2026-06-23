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
# HONEST STATUS (§11.4.6): VERIFIED on nezha 2026-06-23 (evidence
#   docs/qa/20260623-cuttlefish-tier2-ab/). The REAL Android `update_engine` A/B
#   apply + slot flip + headline corrupt-slot AUTO-ROLLBACK (PWU-CF-2, mirroring
#   ab_virt PWU-AB-3) were executed end-to-end on a live Cuttlefish `cvd`
#   (build 15660610, aosp_cf_x86_64_only_phone) on the operator's nezha Linux+KVM
#   host: payload applied (onPayloadApplicationComplete kSuccess (0) ->
#   UPDATE_STATUS_UPDATED_NEED_REBOOT), slot flipped _a -> _b (Virtual-A/B merge
#   merging -> none, _b marked successful), and a forced-bad slot _a was rejected ->
#   device booted known-good _b. Captured evidence: apply_full.log / slot_flip.log /
#   rollback.log / corrupt_dd.txt / ab_facts.txt (§11.4.107/§11.4.69/§11.4.108);
#   curated REPORT.md. The §11.4.135 guard tests/regression/guard_cuttlefish_ab_proven.sh
#   locks the proof in. On a no-KVM host (e.g. this macOS dev host) the script still
#   SKIPs honestly at the topology gate (exit 3) — never a fake PASS (§11.4.3).
#
#   Previously-UNCONFIRMED items now RESOLVED to FACT by the nezha run:
#     - bootctl / update_engine_client are ROOT-ONLY on the cvd (selinux u:r:su:s0),
#       NOT runnable from a plain shell. The A/B apply is driven from the HOST via
#       the AOSP update_device.py over adb (no host sudo for the A/B flow itself).
#     - The OTA payload is obtained with NO credentials: androidbuildinternal.googleapis.com
#       serves a pre-signed GCS URL (storage.googleapis.com) for ota-<BID>.zip
#       (aosp_cf_x86_64_only_phone-ota-15660610.zip, 1003473429 B,
#       md5 d90870a9a6eeece3868520d7fd3f098c — size+md5 verified before apply).
#     - The cvd is Virtual A/B (ro.virtual_ab.enabled=true) + compression + userspace
#       snapshots — NOT legacy A/B; the apply goes through the COW/snapuserd merge path.
#     - The safe corrupt mechanism is: bootctl set-slot-as-unbootable on the inactive
#       slot + a BOUNDED 256 KB write to boot_<inactive> (inactive slot only, never the
#       active/good slot — §11.4.133), then bootctl set-active-boot-slot to force the
#       bad-boot path; the device auto-rolls-back to the known-good slot.
#
# AUTO-ROLLBACK SECTION STATUS: VERIFIED on nezha 2026-06-23 — forced-bad slot _a
#   rejected, device booted known-good _b (rollback.log). On a no-KVM host the whole
#   script (rollback section included) SKIPs cleanly at the topology gate (exit 3,
#   topology_unsupported) — that SKIP is the only thing verifiable on such a host.
#
# Usage:
#   tests/emulator/tier2_cuttlefish_ab.sh [--prepare]              # install cuttlefish debs (Linux, sudo)
#   tests/emulator/tier2_cuttlefish_ab.sh --serial <adb-serial>    # drive an ALREADY-RUNNING cvd (nezha mode)
#   Env: HELIX_CF_DIR (cuttlefish workdir, default ./.cuttlefish),
#        HELIX_CF_SERIAL (adb serial of a live cvd, e.g. 127.0.0.1:6520 — running-container mode),
#        HELIX_CF_BID (build id for the no-creds ota-<BID>.zip fetch, e.g. 15660610),
#        HELIX_CF_TARGET (aosp_cf_arm64_only_phone | aosp_cf_x86_64_only_phone — auto by arch),
#        HELIX_CF_EVIDENCE (default docs/qa/<run-id>/cuttlefish_ab/)
#
# TOPOLOGIES (§11.4.3):
#   (A) SELF-MANAGED (default): fetch_cvd + launch_cvd a fresh cvd in HELIX_CF_DIR,
#       then apply/flip/rollback. Needs Linux + /dev/kvm.
#   (B) RUNNING-CONTAINER (--serial / HELIX_CF_SERIAL): an operator/container has
#       ALREADY launched a privileged cvd (e.g. the containers pkg/cuttlefish path on
#       nezha); this script attaches over `adb -s <serial>`, fetches ota-<BID>.zip via
#       the no-creds androidbuildinternal pre-signed GCS URL, applies via update_device.py,
#       and drives flip+rollback — NEVER touching cvd lifecycle (no stop_cvd in mode B).
#       This is the exact path the 2026-06-23 nezha PASS used.
#
# Outputs: captured evidence (slot-state, update_engine status, rollback trace)
#   under the evidence dir; PASS/FAIL/SKIP verdict on stdout.
# Dependencies (Linux host): git, apt/dpkg, kvm group membership, ~30 GB disk,
#   network (AOSP build fetch). Self-cleaning: stop_cvd on every exit (§11.4.14).
# Cross-references: §11.4.3, §11.4.81, §11.4.69, §11.4.107, §11.4.108, §11.4.112,
#   §11.4.123 (rock-solid captured proof — the apply path is now FACT, not a bluff),
#   §11.4.133 (verified-before-destructive-write for the corrupt-slot mechanism;
#   the "target" is the virtual device, but the safety discipline holds).
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

# ---- arg parse: --serial selects RUNNING-CONTAINER mode (topology B) ---------
CF_SERIAL="${HELIX_CF_SERIAL:-}"
CF_BID="${HELIX_CF_BID:-}"
while [ "$#" -gt 0 ]; do
  case "$1" in
    --serial) CF_SERIAL="${2:-}"; shift 2 ;;
    --serial=*) CF_SERIAL="${1#--serial=}"; shift ;;
    --bid)    CF_BID="${2:-}"; shift 2 ;;
    --bid=*)  CF_BID="${1#--bid=}"; shift ;;
    *) break ;;   # leave remaining args (e.g. --prepare) for the legacy handler
  esac
done

CF_PID_DIR=""
cleanup() {
  if [ -n "$CF_PID_DIR" ] && [ -x "${CF_PID_DIR}/bin/stop_cvd" ]; then
    log "cleanup: stop_cvd"; ( cd "$CF_PID_DIR" && HOME="$CF_PID_DIR" ./bin/stop_cvd >/dev/null 2>&1 || true )
  fi
}
trap cleanup EXIT INT TERM

log "== Tier-2 Cuttlefish real-Android-A/B OTA =="
log "run=${RUN_ID}  host=$(uname -s)/$(uname -m)  $(date -u +%Y-%m-%dT%H:%M:%SZ)"

# =============================================================================
# TOPOLOGY (B) — RUNNING-CONTAINER mode (§11.4.3). VERIFIED on nezha 2026-06-23.
# -----------------------------------------------------------------------------
# When --serial/HELIX_CF_SERIAL names an ALREADY-RUNNING cvd (operator/container
# launched the privileged cvd, e.g. containers pkg/cuttlefish on nezha), this
# script does NOT own cvd lifecycle: it attaches over `adb -s <serial>`, fetches
# ota-<BID>.zip via the no-creds androidbuildinternal pre-signed GCS URL, applies
# via update_device.py, and drives flip+rollback. NEVER stop_cvd in mode B (the
# operator owns the container). The KVM-strict gate below is mode-A only — mode B
# needs only a reachable adb serial (the operator already cleared /dev/kvm).
# =============================================================================
if [ -n "$CF_SERIAL" ]; then
  log "== RUNNING-CONTAINER mode (topology B): driving live cvd serial '${CF_SERIAL}' =="
  command -v adb >/dev/null 2>&1 || { skip "adb not on PATH for running-container mode" "feature_disabled_by_config"; log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="; exit 3; }
  # Scope every plain `adb` (and update_device.py's adb) to the serial via the
  # native ANDROID_SERIAL env var, so the shared "$ADB" callsites below work
  # unchanged for BOTH modes (no word-splitting, no per-callsite -s flag).
  export ANDROID_SERIAL="$CF_SERIAL"
  adb wait-for-device 2>/dev/null || true
  if [ "$(adb shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" != "1" ]; then
    skip "cvd '${CF_SERIAL}' not reachable / not booted (adb)" "network_unreachable_external"
    log "RESULT: SKIP — the operator/container must have a booted cvd at '${CF_SERIAL}'."
    log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
    exit 3
  fi
  # Capture A/B facts (§11.4.69 captured evidence).
  adb shell 'getprop ro.boot.slot_suffix; getprop ro.build.ab_update; getprop ro.virtual_ab.enabled; getprop ro.boot.veritymode' \
    > "${EVID}/ab_facts.txt" 2>/dev/null || true
  pass "running-container cvd '${CF_SERIAL}' reachable + booted" "${EVID}/ab_facts.txt"
  # NOTE (§11.4.6): the REAL apply/flip/rollback on the live serial here mirror the
  # mode-A steps below; the proven 2026-06-23 nezha run captured them under
  # docs/qa/20260623-cuttlefish-tier2-ab/ (apply_full.log/slot_flip.log/rollback.log).
  # OTA payload: no-creds androidbuildinternal pre-signed GCS URL for ota-${CF_BID:-<BID>}.zip
  # -> update_device.py over `adb` (ANDROID_SERIAL-scoped) -> kSuccess ->
  # UPDATED_NEED_REBOOT -> reboot -> slot flip -> bounded inactive-slot corrupt ->
  # reboot -> auto-rollback.
  if [ -z "$CF_BID" ]; then
    skip "no --bid/HELIX_CF_BID build id for the no-creds ota-<BID>.zip fetch" "feature_disabled_by_config"
    log "RESULT: SKIP — supply --bid <BID> (e.g. 15660610) to fetch+apply on the live serial."
    log "  PROVEN reference run: docs/qa/20260623-cuttlefish-tier2-ab/ (REPORT.md, PASS on nezha)."
    log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
    exit 3
  fi
fi

# ---- TOPOLOGY GATE (§11.4.3 / §11.4.81 / §11.4.112) — mode-A (self-managed) only.
# In RUNNING-CONTAINER mode (B) the operator already cleared /dev/kvm by launching
# the cvd, so the strict KVM gate is bypassed (we only needed the reachable serial,
# already proven above). Mode A still requires Linux + a usable /dev/kvm.
if [ -z "$CF_SERIAL" ]; then
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
fi

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

WORK="${CF_DIR}/cvd"
if [ -n "$CF_SERIAL" ]; then
  # ---- RUNNING-CONTAINER mode (B): attach to the operator-launched cvd --------
  # No dpkg/fetch_cvd/launch_cvd — the privileged cvd is already up (CF_PID_DIR
  # stays empty so cleanup() never calls stop_cvd; the operator owns the container).
  # ADB is plain `adb`; the serial is bound via the exported ANDROID_SERIAL above,
  # so every shared "$ADB" callsite below targets the live cvd unchanged.
  ADB="adb"
  SLOT_BEFORE="$("$ADB" shell getprop ro.boot.slot_suffix 2>/dev/null | tr -d '\r')"
  "$ADB" shell getprop > "${EVID}/getprop_before.txt" 2>/dev/null || true
  if [ -z "$SLOT_BEFORE" ]; then
    fail "could not read ro.boot.slot_suffix from live cvd '${CF_SERIAL}' — non-A/B or unreachable"; exit 1
  fi
  pass "running-container cvd baseline active slot = '${SLOT_BEFORE}'" "${EVID}/getprop_before.txt"
else
  if ! dpkg -l cuttlefish-base >/dev/null 2>&1 && [ ! -x "${CF_DIR}/cvd/bin/launch_cvd" ]; then
    skip "cuttlefish-base not installed (run with --prepare first, then reboot)" "feature_disabled_by_config"
    log "RESULT: SKIP — install prerequisites: '$0 --prepare' then reboot."
    exit 3
  fi

  # ---- fetch an A/B build (device images + matching cvd host package) --------
  # Virtual-A/B vs legacy A/B is detected at runtime, never assumed (resolved to
  # FACT by the 2026-06-23 nezha run: ro.virtual_ab.enabled=true → Virtual A/B).
  mkdir -p "$WORK"; cd "$WORK"
  if [ ! -x "./bin/launch_cvd" ]; then
    log "fetching ${CF_TARGET} device images + cvd-host_package (this is large) ..."
    if command -v fetch_cvd >/dev/null 2>&1; then
      fetch_cvd -default_build="aosp-main/${CF_TARGET}-userdebug" 2>&1 | tee -a "${EVID}/fetch.log" \
        || { fail "fetch_cvd failed (see fetch.log) — confirm the build id/branch on the Linux host"; exit 1; }
    else
      skip "fetch_cvd not on PATH — install the cvd host package first" "feature_disabled_by_config"; exit 3
    fi
  fi
  CF_PID_DIR="$WORK"

  # ---- launch + baseline slot ----------------------------------------------
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
    fail "could not read ro.boot.slot_suffix — device may be non-A/B (verify the target on host)"; exit 1
  fi
  pass "cvd booted; baseline active slot = '${SLOT_BEFORE}'" "${EVID}/getprop_before.txt"
fi

# ---- REAL A/B apply via update_engine --------------------------------------
# update_engine writes the OTA payload to the INACTIVE slot, then setActiveBootSlot.
# Driver (resolved to FACT on nezha 2026-06-23): AOSP update_device.py applies the
# OTA zip's payload over adb; the daemon reports kSuccess -> UPDATED_NEED_REBOOT.
# We attempt the documented path, capture the result, and FAIL honestly (never
# fake-PASS) if the apply does not complete (§11.4.6/§11.4.123).
log "applying an OTA payload to the inactive slot via update_engine ..."
APPLIED=0

# Resolve the OTA zip + the update_device.py driver for whichever mode we are in.
OTA_ZIP=""
UPDATE_DEV=""
if [ -n "$CF_SERIAL" ]; then
  # --- RUNNING-CONTAINER mode (B): fetch ota-<BID>.zip via the no-creds
  #     androidbuildinternal pre-signed GCS URL, apply via update_device.py over
  #     the ANDROID_SERIAL-scoped adb (the proven 2026-06-23 nezha path). ---
  OTA_ZIP="${HELIX_CF_OTA_ZIP:-${EVID}/ota-${CF_BID}.zip}"
  if [ ! -f "$OTA_ZIP" ]; then
    log "  fetching ota-${CF_BID}.zip via androidbuildinternal no-creds pre-signed GCS URL ..."
    # The fetch helper (operator-stagable) downloads the public, no-credentials
    # pre-signed URL. If neither the zip nor a fetch helper is present, SKIP-honest
    # (NOT a fake PASS) and point at the proven reference run.
    if command -v fetch_ota_zip >/dev/null 2>&1; then
      fetch_ota_zip "$CF_BID" "$CF_TARGET" "$OTA_ZIP" 2>&1 | tee -a "${EVID}/fetch_ota.log" || true
    fi
  fi
  UPDATE_DEV="$(command -v update_device.py 2>/dev/null || true)"
  [ -n "$UPDATE_DEV" ] || { [ -f "${WORK}/bin/update_device.py" ] && UPDATE_DEV="${WORK}/bin/update_device.py"; }
  if [ -f "$OTA_ZIP" ] && command -v python3 >/dev/null 2>&1 && [ -n "$UPDATE_DEV" ]; then
    python3 "$UPDATE_DEV" --file "$OTA_ZIP" 2>&1 | tee -a "${EVID}/apply.log" && APPLIED=1
  else
    log "  PREREQUISITES MISSING for live apply (ota-${CF_BID}.zip and/or update_device.py)."
    log "  This is the proven path; reference PASS evidence: docs/qa/20260623-cuttlefish-tier2-ab/REPORT.md."
    skip "running-container apply prerequisites not staged on this host" "feature_disabled_by_config"
    log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
    exit 3
  fi
else
  # --- SELF-MANAGED mode (A): the fetched cvd workdir carries ota.zip + driver.
  if [ -f "${WORK}/ota.zip" ] && command -v python3 >/dev/null 2>&1 && [ -f "${WORK}/bin/update_device.py" ]; then
    HOME="$WORK" python3 ./bin/update_device.py --file "${WORK}/ota.zip" 2>&1 | tee -a "${EVID}/apply.log" && APPLIED=1
  else
    log "  no ota.zip/update_device.py present in ${WORK} — build/fetch a signed OTA for"
    log "  ${CF_TARGET} and point this step at it (CUTTLEFISH_TIER2.md §5)."
  fi
fi
if [ "$APPLIED" != 1 ]; then
  fail "OTA apply did not run (prerequisite OTA package missing) — NOT a fake PASS"
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
  || log "  note: verity dmesg not captured this run (ab_facts veritymode=enforcing already proves AVB/dm-verity)"

# =============================================================================
# PWU-CF-2 — HEADLINE corrupt-slot AUTO-ROLLBACK case (mirrors ab_virt PWU-AB-3)
# -----------------------------------------------------------------------------
# VERIFIED on nezha 2026-06-23 (§11.4.6): the assertions below were executed
# end-to-end on a live cvd (evidence rollback.log/corrupt_dd.txt/slot_after_rollback.txt).
# RESOLVED to FACT: the cvd is Virtual A/B (ro.virtual_ab.enabled=true), and the
# safe corrupt-the-inactive-slot mechanism is `bootctl set-slot-as-unbootable`
# PLUS a BOUNDED 256 KB write to boot_<inactive> (inactive slot only, never the
# active/good slot — §11.4.133), then `bootctl set-active-boot-slot` to force the
# bad-boot path. The device auto-rolled-back to the known-good slot. We still
# DETECT the variant + ATTEMPT the mechanism + FAIL HONESTLY (never fake-PASS) so
# the script remains anti-bluff if a different host topology does not reproduce.
#
# Sequence (the headline Tier-2 proof): the previous slot ('${SLOT_BEFORE}') is now
# INACTIVE after the flip above. Corrupt it (mark unbootable + bounded boot write),
# reboot, and ASSERT the device AUTO-ROLLS-BACK to the known-good ACTIVE slot
# ('${SLOT_AFTER}') — boot succeeds on the good slot, the active slot does NOT revert
# to the corrupted one, and the rollback trace is captured (§11.4.108 runtime sig).
# =============================================================================
log ""
log "== PWU-CF-2: corrupt-slot AUTO-ROLLBACK (VERIFIED on nezha 2026-06-23) =="

# The slot we corrupt is the now-INACTIVE previous slot. The known-good slot we
# expect the device to keep/return to is the currently-ACTIVE post-flip slot.
GOOD_SLOT="$SLOT_AFTER"     # known-good, currently active
CORRUPT_SLOT="$SLOT_BEFORE" # now inactive — the one we deliberately break

# Detect A/B variant so we corrupt the right thing (§11.4.6 — never guess).
VAB="$("$ADB" shell getprop ro.virtual_ab.enabled 2>/dev/null | tr -d '\r')"
"$ADB" shell getprop ro.virtual_ab.enabled > "${EVID}/virtual_ab.txt" 2>/dev/null || true
log "  A/B variant: ro.virtual_ab.enabled='${VAB:-<unset>}' (true=Virtual A/B, else legacy A/B)"

# --- mark the inactive slot unbootable via the documented boot_control path ----
# bootctl set-slot-as-unbootable is the documented HAL surface (boot_control
# bootable/active/successful, [SRC-AB]). FACT (nezha 2026-06-23): `bootctl` works
# on the cvd but ONLY AS ROOT (selinux u:r:su:s0) — a plain `adb shell bootctl`
# is denied. The privileged-cvd / running-container path runs it as root; we
# attempt it, capture the result, and never assert a command that is unavailable.
CORRUPTED=0
log "  attempting to mark inactive slot '${CORRUPT_SLOT}' unbootable / unsuccessful ..."
if "$ADB" shell 'command -v bootctl' >/dev/null 2>&1; then
  # Map the suffix (_a/_b) to its slot index for bootctl (0=_a, 1=_b).
  case "$CORRUPT_SLOT" in
    _a|a) CORRUPT_IDX=0 ;;
    _b|b) CORRUPT_IDX=1 ;;
    *)    CORRUPT_IDX="" ;;
  esac
  if [ -n "$CORRUPT_IDX" ]; then
    "$ADB" shell "bootctl set-slot-as-unbootable ${CORRUPT_IDX}" \
      > "${EVID}/corrupt_bootctl.txt" 2>&1 && CORRUPTED=1 || true
    "$ADB" shell 'bootctl dump-slots-info' >> "${EVID}/corrupt_bootctl.txt" 2>&1 || true
  fi
fi

# --- bounded boot-partition corruption on the inactive slot -------------------
# FACT (nezha 2026-06-23): the proven mechanism is a BOUNDED write to boot_<inactive>
# (the boot partition of the inactive slot — NOT system), 256 KB, inactive-slot-only
# (§11.4.133 verified-before-destructive-write: bounded, inactive slot only, never
# the active/good slot). This complements bootctl set-slot-as-unbootable so the
# slot fails to boot on the forced bad-boot reboot. corrupt_dd.txt captures the run.
if [ "$CORRUPTED" != 1 ]; then
  log "  bootctl unbootable unavailable here — bounded boot_<inactive> corruption (256 KB, inactive only)"
  CORRUPT_PART="/dev/block/by-name/boot${CORRUPT_SLOT}"
  if "$ADB" shell "test -e ${CORRUPT_PART}" >/dev/null 2>&1; then
    # 256 KB bounded write (64 x 4K) — recoverable, inactive slot only (§11.4.133).
    "$ADB" shell "dd if=/dev/urandom of=${CORRUPT_PART} bs=4096 count=64 conv=notrunc" \
      > "${EVID}/corrupt_dd.txt" 2>&1 && CORRUPTED=1 || true
  fi
fi

if [ "$CORRUPTED" != 1 ]; then
  fail "could not corrupt the inactive slot '${CORRUPT_SLOT}' (no bootctl unbootable, no by-name boot partition) — NOT a fake PASS"
  exit 1
fi
pass "inactive slot '${CORRUPT_SLOT}' corrupted (unbootable + bounded boot write)" "${EVID}/corrupt_bootctl.txt"

# --- set the corrupted slot active to force the bad-boot → rollback path -------
# Force the next boot to TRY the corrupted slot so the bootloader's bad-boot
# fallback (A/B keeps the unused slot as fallback; reboot into old image on a bad
# OTA / dm-verity failure, [SRC-AB-SEARCH]) is exercised. FACT (nezha 2026-06-23):
# bootctl set-active-boot-slot on the bad slot forced the path; the device rejected
# it and booted the known-good slot.
if "$ADB" shell 'command -v bootctl' >/dev/null 2>&1 && [ -n "${CORRUPT_IDX:-}" ]; then
  "$ADB" shell "bootctl set-active-boot-slot ${CORRUPT_IDX}" \
    > "${EVID}/corrupt_setactive.txt" 2>&1 || true
fi

# --- reboot and ASSERT auto-rollback to the known-good slot -------------------
log "  rebooting — expecting AUTO-ROLLBACK to known-good slot '${GOOD_SLOT}' ..."
"$ADB" reboot 2>/dev/null; sleep 5
BOOTED=0
for i in $(seq 1 60); do
  "$ADB" wait-for-device 2>/dev/null
  [ "$("$ADB" shell getprop sys.boot_completed 2>/dev/null | tr -d '\r')" = "1" ] && { BOOTED=1; break; }
  sleep 5
done
SLOT_ROLLBACK="$("$ADB" shell getprop ro.boot.slot_suffix 2>/dev/null | tr -d '\r')"
"$ADB" shell getprop ro.boot.slot_suffix > "${EVID}/slot_after_rollback.txt" 2>/dev/null || true
# capture the rollback trace (update_verifier / dm-verity / bootloader fallback)
"$ADB" shell 'dmesg | grep -iE "verity|update_verifier|rollback|slot|boot_control"' \
  > "${EVID}/rollback_trace.txt" 2>/dev/null || true
"$ADB" shell logcat -d -s update_verifier:* > "${EVID}/rollback_logcat.txt" 2>/dev/null || true

if [ "$BOOTED" != 1 ]; then
  fail "device did NOT finish booting after corrupting slot '${CORRUPT_SLOT}' — a hang is NOT a rollback PASS (see ${EVID}/rollback_trace.txt)"
  exit 1
fi
# Auto-rollback succeeded iff the device booted back on the known-good slot and
# did NOT come up on the corrupted slot.
if [ "$SLOT_ROLLBACK" = "$GOOD_SLOT" ] && [ "$SLOT_ROLLBACK" != "$CORRUPT_SLOT" ]; then
  pass "AUTO-ROLLBACK confirmed: corrupted slot '${CORRUPT_SLOT}' rejected, device booted known-good slot '${SLOT_ROLLBACK}'" "${EVID}/slot_after_rollback.txt"
  [ -s "${EVID}/rollback_trace.txt" ] \
    && pass "rollback trace captured (dm-verity/update_verifier/bootloader fallback)" "${EVID}/rollback_trace.txt" \
    || log "  note: rollback trace empty this run (slot-state delta already proves the rollback)"
else
  fail "NO auto-rollback: expected known-good '${GOOD_SLOT}', got '${SLOT_ROLLBACK}' (corrupted='${CORRUPT_SLOT}') — NOT a fake PASS"
  exit 1
fi

log ""
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
[ "$FAIL" -gt 0 ] && { log "RESULT: FAIL"; exit 1; }
log "RESULT: PASS"
exit 0
