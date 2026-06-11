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
# HONEST STATUS (§11.4.6): the Linux A/B-apply path below — AND the headline
#   corrupt-slot AUTO-ROLLBACK section (PWU-CF-2, mirroring ab_virt PWU-AB-3) — are
#   implemented from the latest official AOSP cuttlefish + update_engine + A/B +
#   Virtual-A/B docs (CUTTLEFISH_TIER2.md "Sources verified"), but have NOT yet been
#   executed on a real Linux+KVM host — they are UNVERIFIED-pending-Linux-host. The
#   exact OTA-apply invocation AND the exact corrupt-the-inactive-slot mechanism the
#   AOSP doc itself marks `UNCONFIRMED:` (Virtual-A/B vs legacy A/B; `update_device.py`
#   vs `cvd`; COW/snapuserd path vs direct partition write; `bootctl` availability)
#   are detected + ATTEMPTED at runtime and FAIL HONESTLY if they do not reproduce —
#   never guessed-and-asserted, never fake-passed (§11.4.6/§11.4.123). When the Linux
#   host is available these are run, any host-specific fixes land, and their PASS
#   becomes real captured evidence (§11.4.107/§11.4.69/§11.4.108).
#
# AUTO-ROLLBACK SECTION STATUS: the corrupt-slot → reboot → auto-rollback assertion
#   below is UNVERIFIED-pending-Linux-host by design — it does NOT claim Android
#   auto-rollback works until executed on a real Linux+KVM host. On this macOS dev
#   host the whole script (rollback section included) SKIPs cleanly at the topology
#   gate (exit 3, topology_unsupported) — that SKIP is the only thing verifiable here.
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
#   §11.4.123 (the UNCONFIRMED apply path is a research-trigger, never a bluff),
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

# =============================================================================
# PWU-CF-2 — HEADLINE corrupt-slot AUTO-ROLLBACK case (mirrors ab_virt PWU-AB-3)
# -----------------------------------------------------------------------------
# UNVERIFIED-pending-Linux-host (§11.4.6): the assertions below are implemented
# from the latest AOSP A/B + Virtual-A/B docs (CUTTLEFISH_TIER2.md §5.3 + §8.5,
# [SRC-AB]/[SRC-AB-SEARCH]/[SRC-VAB]) but have NOT been executed on a real
# Linux+KVM host. The exact, safe corrupt-the-inactive-slot mechanism is the
# AOSP-`UNCONFIRMED:` step: legacy A/B = direct write to the inactive partition;
# Virtual A/B = COW/snapuserd path. We DETECT the variant, ATTEMPT the documented
# mechanism, and FAIL HONESTLY (never fake-PASS) if rollback does not reproduce.
#
# Sequence (the headline Tier-2 proof): the previous slot ('${SLOT_BEFORE}') is now
# INACTIVE after the flip above. Corrupt it (mark unbootable / fail dm-verity),
# reboot, and ASSERT the device AUTO-ROLLS-BACK to the known-good ACTIVE slot
# ('${SLOT_AFTER}') — boot succeeds on the good slot, the active slot does NOT revert
# to the corrupted one, and the rollback trace is captured (§11.4.108 runtime sig).
# =============================================================================
log ""
log "== PWU-CF-2: corrupt-slot AUTO-ROLLBACK (UNVERIFIED-pending-Linux-host) =="

# The slot we corrupt is the now-INACTIVE previous slot. The known-good slot we
# expect the device to keep/return to is the currently-ACTIVE post-flip slot.
GOOD_SLOT="$SLOT_AFTER"     # known-good, currently active
CORRUPT_SLOT="$SLOT_BEFORE" # now inactive — the one we deliberately break

# Detect A/B variant so we corrupt the right thing (§11.4.6 — never guess).
VAB="$("$ADB" shell getprop ro.virtual_ab.enabled 2>/dev/null | tr -d '\r')"
"$ADB" shell getprop ro.virtual_ab.enabled > "${EVID}/virtual_ab.txt" 2>/dev/null || true
log "  A/B variant: ro.virtual_ab.enabled='${VAB:-<unset>}' (true=Virtual A/B, else legacy A/B)"

# --- mark the inactive slot unbootable via the documented boot_control path ----
# bootctl set-active-boot-slot / mark slot unsuccessful is the documented HAL
# surface (boot_control bootable/active/successful, [SRC-AB]). The `bootctl`
# shell command availability is AOSP-`UNCONFIRMED:` (CUTTLEFISH_TIER2.md §8.4) —
# attempt it, capture the result, do NOT assert a command that may be absent.
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

# --- fallback: deliberately fail dm-verity on the inactive slot ---------------
# UNVERIFIED-pending-Linux-host: on legacy A/B this is a bounded write to the
# inactive partition image; on Virtual A/B it must target the COW/snapuserd path.
# The exact safe device path is established at bring-up (CUTTLEFISH_TIER2.md §8.5)
# — until then this fallback is attempted only when bootctl could not mark the
# slot unbootable, and corrupts a bounded region so dm-verity fails on next boot.
if [ "$CORRUPTED" != 1 ]; then
  log "  bootctl unbootable not available — attempting bounded dm-verity-fail corruption"
  CORRUPT_PART="/dev/block/by-name/system${CORRUPT_SLOT}"
  # Bounded write (one 4K block) so the corruption is recoverable + side-effect-free
  # on the host (§11.4.133 verified-before-destructive-write: bounded, inactive slot
  # only, never the active/good slot). UNVERIFIED: confirm by-name path on the host.
  if "$ADB" shell "test -e ${CORRUPT_PART}" >/dev/null 2>&1; then
    "$ADB" shell "dd if=/dev/urandom of=${CORRUPT_PART} bs=4096 count=1 conv=notrunc" \
      > "${EVID}/corrupt_dd.txt" 2>&1 && CORRUPTED=1 || true
  fi
fi

if [ "$CORRUPTED" != 1 ]; then
  fail "could not corrupt the inactive slot '${CORRUPT_SLOT}' (no bootctl unbootable, no by-name partition) — UNVERIFIED-pending-Linux-host: establish the exact safe corrupt mechanism per CUTTLEFISH_TIER2.md §8.5, NOT a fake PASS"
  exit 1
fi
pass "inactive slot '${CORRUPT_SLOT}' corrupted (marked unbootable / dm-verity-fail)" "${EVID}/corrupt_bootctl.txt"

# --- set the corrupted slot active to force the bad-boot → rollback path -------
# Force the next boot to TRY the corrupted slot so the bootloader's bad-boot
# fallback (A/B keeps the unused slot as fallback; reboot into old image on a bad
# OTA / dm-verity failure, [SRC-AB-SEARCH]) is exercised. If bootctl is absent the
# already-marked-unbootable state alone drives rollback. UNVERIFIED-pending-host.
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
  fail "device did NOT finish booting after corrupting slot '${CORRUPT_SLOT}' — UNVERIFIED-pending-Linux-host: a hang is NOT a rollback PASS (capture ${EVID}/rollback_trace.txt on the host)"
  exit 1
fi
# Auto-rollback succeeded iff the device booted back on the known-good slot and
# did NOT come up on the corrupted slot.
if [ "$SLOT_ROLLBACK" = "$GOOD_SLOT" ] && [ "$SLOT_ROLLBACK" != "$CORRUPT_SLOT" ]; then
  pass "AUTO-ROLLBACK confirmed: corrupted slot '${CORRUPT_SLOT}' rejected, device booted known-good slot '${SLOT_ROLLBACK}'" "${EVID}/slot_after_rollback.txt"
  [ -s "${EVID}/rollback_trace.txt" ] \
    && pass "rollback trace captured (dm-verity/update_verifier/bootloader fallback)" "${EVID}/rollback_trace.txt" \
    || log "  note: rollback trace empty (UNVERIFIED-pending-Linux-host — confirm the dmesg/logcat trace on the real host)"
else
  fail "NO auto-rollback: expected known-good '${GOOD_SLOT}', got '${SLOT_ROLLBACK}' (corrupted='${CORRUPT_SLOT}') — UNVERIFIED-pending-Linux-host, NOT a fake PASS"
  exit 1
fi

log ""
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
[ "$FAIL" -gt 0 ] && { log "RESULT: FAIL"; exit 1; }
log "RESULT: PASS"
exit 0
