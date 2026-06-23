#!/usr/bin/env bash
# =============================================================================
# guard_antibluff_evidence_lib.sh — §11.4.135 regression guard
# -----------------------------------------------------------------------------
# Bug guarded: helix_ota had NO shell helper implementing the §11.4.69 canonical
#   PASS/SKIP evidence-gating contract (ab_pass_with_evidence /
#   ab_skip_with_reason) — every test could emit a PASS without citing real
#   captured evidence, the exact §11.4 PASS-bluff vector §11.4.69 forbids.
#   FIX: tests/lib/anti_bluff.sh implements the contract; this guard makes the
#   library's bluff-proof self-test a STANDING regression guard so the
#   no-evidence-no-PASS guarantee can never silently regress.
#
# Strategy: run tests/lib/anti_bluff_selftest.sh, which (1) proves the five
#   PASS/SKIP behaviours and (2) runs the §1.1 always-pass mutation proof
#   (RED mutant must be caught). A green self-test is the GREEN guard.
#
# Usage: bash tests/regression/guard_antibluff_evidence_lib.sh
# Exit 0 = guard GREEN.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
LIB="${ROOT}/tests/lib/anti_bluff.sh"
SELFTEST="${ROOT}/tests/lib/anti_bluff_selftest.sh"

fail() { echo "  GUARD-FAIL: $*" >&2; exit 1; }
pass() { echo "  ok: $*"; }

echo "[guard_antibluff_evidence_lib]"
[[ -f "$LIB" ]]      || fail "tests/lib/anti_bluff.sh missing — §11.4.69 evidence helper gone."
[[ -f "$SELFTEST" ]] || fail "tests/lib/anti_bluff_selftest.sh missing — bluff-proof gone."

# Source-layer assertion (§11.4.108 layer 1): the no-evidence-no-PASS literal
# must be present so the guarantee is mechanically auditable in the artifact.
grep -q 'no-evidence-no-PASS' "$LIB" \
    || fail "anti_bluff.sh lost the §11.4.69 no-evidence-no-PASS rejection — bluff vector reopened."
pass "anti_bluff.sh retains the §11.4.69 no-evidence-no-PASS rejection."

# Behavioural GREEN: the self-test (five behaviours + §1.1 mutation proof).
if sh "$SELFTEST"; then
    pass "anti_bluff_selftest.sh GREEN (five behaviours + always-pass mutation caught)."
else
    fail "anti_bluff_selftest.sh FAILED — §11.4.69 evidence-gating regressed."
fi

echo "GUARD-GREEN: §11.4.69 anti-bluff evidence helper"
exit 0
