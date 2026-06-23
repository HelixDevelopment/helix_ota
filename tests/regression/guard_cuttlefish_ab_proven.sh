#!/usr/bin/env bash
# =============================================================================
# guard_cuttlefish_ab_proven.sh — §11.4.135 standing regression guard
# -----------------------------------------------------------------------------
# Locks in the 2026-06-23 Tier-2 Cuttlefish REAL Android A/B PASS (run-id
# docs/qa/20260623-cuttlefish-tier2-ab/). A real `update_engine` A/B OTA was
# applied to a live cvd (build 15660610) on nezha: payload applied (kSuccess) ->
# slot flip _a->_b -> auto-rollback of a forced-bad slot. This guard asserts the
# captured evidence anchors AND the validator's VERIFIED status header still
# exist, so a future edit that strips the proof or downgrades the claim FAILs the
# §11.4.40 release sweep.
#
# Asserted GREEN anchors (the proof must stay present):
#   E1  slot_flip.log  contains "SLOT FLIPPED _a -> _b"   (real A/B slot switch)
#   E2  rollback.log   contains "AUTO-ROLLBACK CONFIRMED" (forced-bad slot rejected)
#   E3  apply_full.log contains "kSuccess"                (real update_engine apply)
#   E4  tier2_cuttlefish_ab.sh HONEST STATUS header says VERIFIED on nezha 2026-06-23
#
# §11.4.115 polarity (RED-on-stripped): each anchor is also checked against a
#   synthetic "evidence stripped" string, which MUST FAIL the same predicate —
#   proving the guard genuinely catches the negation (stripped proof / reverted
#   VERIFIED claim), not a tautology.
#
# Honest boundary (§11.4.6): this is an EVIDENCE-PRESENCE + CLAIM-CONSISTENCY
#   guard over committed artefacts. It does NOT re-run the cvd (that needs the
#   nezha Linux+KVM host). It guarantees the captured PASS evidence and the
#   VERIFIED claim cannot be silently removed or downgraded without a guard FAIL.
#
# Usage: bash tests/regression/guard_cuttlefish_ab_proven.sh
# Exit 0 = guard GREEN.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
EVID="${ROOT}/docs/qa/20260623-cuttlefish-tier2-ab"
VALIDATOR="${ROOT}/tests/emulator/tier2_cuttlefish_ab.sh"

fail() { echo "  GUARD-FAIL: $*" >&2; exit 1; }
pass() { echo "  ok: $*"; }

echo "[guard_cuttlefish_ab_proven]"

# contains FILE NEEDLE -> 0 if NEEDLE present in FILE, else 1.
contains() {
    local f="$1" needle="$2"
    [ -f "$f" ] || return 2
    grep -qF -- "$needle" "$f"
}

# --- §11.4.115 RED polarity: the predicate MUST reject a stripped/empty proof ---
STRIPPED="$(mktemp)"; printf 'evidence removed\nnothing here\n' > "$STRIPPED"
trap 'rm -f "$STRIPPED"' EXIT
if contains "$STRIPPED" "SLOT FLIPPED _a -> _b"; then
    fail "RED broken: 'SLOT FLIPPED _a -> _b' matched a stripped-evidence file — guard is a tautology."
fi
if contains "$STRIPPED" "AUTO-ROLLBACK CONFIRMED"; then
    fail "RED broken: 'AUTO-ROLLBACK CONFIRMED' matched a stripped-evidence file — guard is a tautology."
fi
if contains "$STRIPPED" "kSuccess"; then
    fail "RED broken: 'kSuccess' matched a stripped-evidence file — guard is a tautology."
fi
echo "  RED-CONFIRMED: every proof predicate FAILS on stripped/empty evidence (catches the negation)."

# --- E1: real A/B slot flip evidence -----------------------------------------
contains "${EVID}/slot_flip.log" "SLOT FLIPPED _a -> _b" \
    || fail "E1: slot_flip.log missing 'SLOT FLIPPED _a -> _b' — real A/B slot-switch proof stripped."
pass "E1: slot_flip.log proves real A/B slot flip _a -> _b."

# --- E2: auto-rollback evidence ----------------------------------------------
contains "${EVID}/rollback.log" "AUTO-ROLLBACK CONFIRMED" \
    || fail "E2: rollback.log missing 'AUTO-ROLLBACK CONFIRMED' — auto-rollback proof stripped."
pass "E2: rollback.log proves forced-bad-slot auto-rollback to known-good slot."

# --- E3: real update_engine apply evidence -----------------------------------
contains "${EVID}/apply_full.log" "kSuccess" \
    || fail "E3: apply_full.log missing 'kSuccess' — real update_engine OTA apply proof stripped."
pass "E3: apply_full.log proves update_engine OTA apply completed kSuccess."

# --- E4: validator VERIFIED status header ------------------------------------
[ -f "$VALIDATOR" ] || fail "E4: tier2_cuttlefish_ab.sh not found."
contains "$VALIDATOR" "VERIFIED on nezha 2026-06-23" \
    || fail "E4: validator HONEST STATUS no longer claims 'VERIFIED on nezha 2026-06-23' — VERIFIED claim downgraded/reverted."
# negation proof for E4 too: a synthetic old header MUST FAIL the predicate
OLD_HDR="$(mktemp)"; printf 'HONEST STATUS: UNVERIFIED-pending-Linux-host\n' > "$OLD_HDR"
if contains "$OLD_HDR" "VERIFIED on nezha 2026-06-23"; then
    rm -f "$OLD_HDR"; fail "RED broken (E4): old UNVERIFIED header matched the VERIFIED predicate."
fi
rm -f "$OLD_HDR"
pass "E4: validator HONEST STATUS header reflects VERIFIED on nezha 2026-06-23."

echo "GUARD-GREEN: Cuttlefish Tier-2 REAL Android A/B PASS evidence + VERIFIED claim intact."
exit 0
