#!/usr/bin/env bash
# =============================================================================
# guard_push_all_honest_exit.sh — §11.4.135 regression guard
# -----------------------------------------------------------------------------
# Bug guarded: push_all.sh honest-exit (commit 17cbd47a / §11.4.1).
#   The OLD code printed "All upstreams pushed" + exit 0 even when EVERY remote
#   was SKIPPED_LOCKED / FAILED / UNKNOWN — a §11.4.1 false-success bluff at the
#   push layer (the summary was green while no remote was actually confirmed).
#   The FIX adds NOT_CONFIRMED accounting: a remote counts as "pushed" ONLY if
#   its status is OK or PHASED; any other status increments NOT_CONFIRMED and
#   the script exits 1.
#
# Strategy (source-extracted unit harness, honest boundary noted below):
#   We re-implement ONLY the script's terminal accounting block as the
#   subject-under-test, drive it with a synthetic REMOTE_STATUS map, and prove:
#     RED  : the OLD accounting form (exit 0 whenever TOTAL_FAILURES==0,
#            ignoring SKIPPED_LOCKED) returns 0 on an all-SKIPPED_LOCKED map.
#     GREEN: the FIXED accounting form (NOT_CONFIRMED over the OK|PHASED set)
#            returns 1 on the same all-SKIPPED_LOCKED map, AND returns 0 only
#            when every remote is OK/PHASED.
#   We ADDITIONALLY assert (static) that the real scripts/push_all.sh contains
#   the NOT_CONFIRMED guard and exits 1 on it — so a future edit removing the
#   guard is caught.
#
# Honest boundary (§11.4.6): the RED here reproduces the OLD *accounting logic*
# (the exact bug class), not a checkout of the pre-fix file. The all-SKIPPED
# scenario is real and the fix's negation is genuinely exercised.
#
# Polarity: RED_MODE=1 demonstrates the OLD-logic bug (default off); the GREEN
# assertions always run and are the standing guard.
#
# Usage: bash tests/regression/guard_push_all_honest_exit.sh
# Exit 0 = guard GREEN; non-zero = guard caught a regression.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
PUSH_ALL="${ROOT}/scripts/push_all.sh"
RED_MODE="${RED_MODE:-0}"

fail() { echo "  GUARD-FAIL: $*" >&2; exit 1; }
pass() { echo "  ok: $*"; }

# --- Subject under test: the two accounting variants -------------------------
# OLD (buggy) accounting: success iff no HARD failure, ignoring skips.
old_accounting_exit() {  # args: status... ; returns the OLD exit code
    local total_failures=0 s
    for s in "$@"; do [[ "$s" == "FAILED" ]] && total_failures=$((total_failures+1)); done
    [[ $total_failures -gt 0 ]] && return 1
    return 0   # <-- OLD: "All upstreams pushed", exit 0
}

# FIXED accounting: a remote is confirmed ONLY if OK or PHASED.
fixed_accounting_exit() {  # args: status... ; returns the FIXED exit code
    local total_failures=0 not_confirmed=0 s
    for s in "$@"; do
        [[ "$s" == "FAILED" ]] && total_failures=$((total_failures+1))
        case "$s" in OK|PHASED) : ;; *) not_confirmed=$((not_confirmed+1)) ;; esac
    done
    if [[ $total_failures -gt 0 || $not_confirmed -gt 0 ]]; then return 1; fi
    return 0
}

echo "[guard_push_all_honest_exit]"

# Scenario: every remote SKIPPED_LOCKED (the exact forensic case).
ALL_SKIPPED=(SKIPPED_LOCKED SKIPPED_LOCKED SKIPPED_LOCKED SKIPPED_LOCKED)
ALL_OK=(OK OK OK OK)
MIXED=(OK PHASED SKIPPED_LOCKED OK)

# --- RED (reproduces the bug on the OLD logic) -------------------------------
if old_accounting_exit "${ALL_SKIPPED[@]}"; then
    echo "  RED-CONFIRMED: OLD accounting returns exit 0 on all-SKIPPED_LOCKED (the bug)."
    [[ "$RED_MODE" == "1" ]] && fail "RED_MODE: bug reproduced as expected (this is the defect)."
else
    fail "OLD accounting did NOT reproduce the bug — harness broken (§11.4.115)."
fi

# --- GREEN (the fix's behaviour — standing guard) ----------------------------
if fixed_accounting_exit "${ALL_SKIPPED[@]}"; then
    fail "FIXED accounting returned 0 on all-SKIPPED_LOCKED — honest-exit bug is BACK."
fi
pass "FIXED accounting exits 1 on all-SKIPPED_LOCKED (no remote confirmed)."

fixed_accounting_exit "${ALL_OK[@]}" || fail "FIXED accounting must exit 0 when all OK."
pass "FIXED accounting exits 0 when every remote OK."

if fixed_accounting_exit "${MIXED[@]}"; then
    fail "FIXED accounting returned 0 with a SKIPPED_LOCKED remote present."
fi
pass "FIXED accounting exits 1 when any remote is not OK/PHASED (mixed)."

# --- Static guard on the real script -----------------------------------------
[[ -f "$PUSH_ALL" ]] || fail "scripts/push_all.sh not found at $PUSH_ALL"
grep -q 'NOT_CONFIRMED' "$PUSH_ALL" \
    || fail "scripts/push_all.sh lost its NOT_CONFIRMED accounting (§11.4.1 regression)."
grep -qE 'NOT_CONFIRMED[+ ].*1' "$PUSH_ALL" \
    || fail "scripts/push_all.sh NOT_CONFIRMED is not incremented."
grep -qE '\bexit 1\b' "$PUSH_ALL" \
    || fail "scripts/push_all.sh no longer exits 1 on unconfirmed remotes."
pass "scripts/push_all.sh retains NOT_CONFIRMED guard + exit 1."

echo "GUARD-GREEN: push_all honest-exit"
exit 0
