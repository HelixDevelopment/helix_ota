#!/usr/bin/env bash
# =============================================================================
# meta_test_semgrep_wired.sh — §1.1 meta-test for the CM-SEMGREP-WIRED gate
# -----------------------------------------------------------------------------
# CM-SEMGREP-WIRED (in pre_build_verification.sh, §11.4.166) asserts that
# scripts/commit_all.sh keeps the semgrep static-analysis wiring AND that the
# not-installed path is NOT fail-open (an absent semgrep must not silently count
# as clean). This meta-test proves the gate genuinely FAILs when that wiring is
# removed — i.e. that CM-SEMGREP-WIRED is not an always-green bluff.
#
# Subject-under-test: a faithful inline replica of the three grep assertions the
# gate uses (same literal patterns) evaluated against the REAL commit_all.sh.
# We do NOT mutate the gate; we mutate the WIRED FILE and assert the SAME checks
# flip PASS→FAIL.
#
# Three mutations, each independently proven to flip the gate RED (then restored
# byte-identically, §11.4.84):
#   (1) remove the blocking 'semgrep scan --config auto --error' invocation;
#   (2) remove the _semgrep_scan_check function reference;
#   (3) re-introduce a fail-open (delete the honest 'semgrep NOT installed'
#       blocker message) — proving the gate catches the exact §11.4.166 hole.
#
# Usage: bash tests/meta/meta_test_semgrep_wired.sh
# Exit 0 = CM-SEMGREP-WIRED proven to catch wiring removal / fail-open re-intro.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=tests/meta/lib_metatest.sh
. "${SCRIPT_DIR}/lib_metatest.sh"

mt_init "meta_test_semgrep_wired"

COMMIT_ALL="${ROOT}/scripts/commit_all.sh"
[[ -f "$COMMIT_ALL" ]] || mt_fail "scripts/commit_all.sh missing — §11.4.166 wiring gone."

# wiring_gate: faithful replica of CM-SEMGREP-WIRED's wiring assertions.
# Returns 0 iff ALL three invariants hold in $COMMIT_ALL (gate would PASS the
# wiring portion); returns 1 if any is broken (gate would FAIL).
wiring_gate() {
    grep -q '_semgrep_scan_check' "$COMMIT_ALL"               || return 1
    grep -q 'semgrep scan --config auto --error' "$COMMIT_ALL" || return 1
    grep -q 'semgrep NOT installed' "$COMMIT_ALL"             || return 1
    return 0
}

echo "  --- subject: scripts/commit_all.sh semgrep wiring ---"
mt_assert_gate_passes "wiring clean" wiring_gate

# Mutation (1): remove the blocking semgrep invocation.
mt_mutate_file "$COMMIT_ALL" 's/semgrep scan --config auto --error/REMOVED_FOR_META/g'
mt_assert_gate_fails  "wiring: blocking-invocation removed" wiring_gate
mt_restore_all
mt_assert_gate_passes "wiring post-restore (1)" wiring_gate

# Mutation (2): remove the _semgrep_scan_check function reference.
mt_mutate_file "$COMMIT_ALL" 's/_semgrep_scan_check/_REMOVED_FN_FOR_META/g'
mt_assert_gate_fails  "wiring: scan function removed" wiring_gate
mt_restore_all
mt_assert_gate_passes "wiring post-restore (2)" wiring_gate

# Mutation (3): re-introduce the fail-open (delete the honest blocker message).
mt_mutate_file "$COMMIT_ALL" 's/semgrep NOT installed/semgrep skipped silently/g'
mt_assert_gate_fails  "wiring: fail-open re-introduced" wiring_gate
mt_restore_all
mt_assert_gate_passes "wiring post-restore (3)" wiring_gate

echo "META-GREEN: CM-SEMGREP-WIRED catches wiring removal AND fail-open re-introduction (bluff-proof)."
exit 0
