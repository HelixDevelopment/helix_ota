#!/usr/bin/env bash
# =============================================================================
# meta_test_regression_guards.sh — §1.1 meta-test proving regression guards real
# -----------------------------------------------------------------------------
# Proves that the §11.4.135 standing regression guards genuinely catch their own
# negation — that they are not always-green bluffs. For each subject guard:
#   1. assert the guard is GREEN on the clean source tree;
#   2. MUTATE the real source file the guard checks (byte-safe backup) so the
#      guarded invariant is broken;
#   3. assert the guard now goes RED (non-zero) — proving it is a real test;
#   4. restore byte-identically (§11.4.84 no residue, sha256-verified by the
#      lib_metatest restore) and assert the guard is GREEN again.
#
# Subject guards (2, per the batch scope):
#   - guard_push_all_honest_exit.sh — asserts scripts/push_all.sh retains the
#     NOT_CONFIRMED honest-exit accounting. Mutation renames NOT_CONFIRMED so the
#     guard's grep fails → RED.
#   - guard_healthcheck_binary_present.sh — asserts the helixtrack-core
#     healthcheck probe binary is 'curl' (in the known-installed set). Mutation
#     rewrites the probe to a binary NOT in the image → RED.
#
# Usage: bash tests/meta/meta_test_regression_guards.sh
# Exit 0 = both guards proven RED-on-broken / GREEN-on-fixed (bluff-proof).
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=tests/meta/lib_metatest.sh
. "${SCRIPT_DIR}/lib_metatest.sh"

mt_init "meta_test_regression_guards"

GUARD_PUSH="${ROOT}/tests/regression/guard_push_all_honest_exit.sh"
PUSH_ALL="${ROOT}/scripts/push_all.sh"
GUARD_HC="${ROOT}/tests/regression/guard_healthcheck_binary_present.sh"
COMPOSE="${ROOT}/deploy/helixtrack/compose.helixtrack.yml"

[[ -f "$GUARD_PUSH" && -f "$PUSH_ALL" ]] || mt_fail "push-all guard or source missing."
[[ -f "$GUARD_HC"   && -f "$COMPOSE"  ]] || mt_fail "healthcheck guard or compose missing."

# --- Subject 1: guard_push_all_honest_exit (NOT_CONFIRMED accounting) --------
echo "  --- subject: guard_push_all_honest_exit.sh ---"
mt_assert_gate_passes "push-guard clean" bash "$GUARD_PUSH"

# Break the invariant: rename the NOT_CONFIRMED token the guard greps for.
mt_mutate_file "$PUSH_ALL" 's/NOT_CONFIRMED/NOT_CONFIRMED_RENAMED_FOR_META/g'
mt_assert_gate_fails  "push-guard on mutation" bash "$GUARD_PUSH"

mt_restore_all
mt_assert_gate_passes "push-guard post-restore" bash "$GUARD_PUSH"

# --- Subject 2: guard_healthcheck_binary_present (curl probe) ----------------
echo "  --- subject: guard_healthcheck_binary_present.sh ---"
mt_assert_gate_passes "hc-guard clean" bash "$GUARD_HC"

# Break the invariant: rewrite the healthcheck probe binary curl -> a binary
# that is NOT in the guard's KNOWN_IMAGE_BINARIES set (unhealthy-despite-200).
mt_mutate_file "$COMPOSE" 's#"curl"#"definitely-not-in-image"#g'
mt_assert_gate_fails  "hc-guard on mutation" bash "$GUARD_HC"

mt_restore_all
mt_assert_gate_passes "hc-guard post-restore" bash "$GUARD_HC"

echo "META-GREEN: both regression guards proven RED-on-broken / GREEN-on-fixed (bluff-proof)."
exit 0
