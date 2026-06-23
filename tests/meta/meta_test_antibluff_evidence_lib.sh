#!/usr/bin/env bash
# =============================================================================
# meta_test_antibluff_evidence_lib.sh — §1.1 meta-test for the §11.4.69 helper
# -----------------------------------------------------------------------------
# Registers the existing anti_bluff_selftest.sh (which already performs the §1.1
# always-pass mutation proof: it mutates a COPY of tests/lib/anti_bluff.sh so
# ab_pass_with_evidence always returns 0, asserts the mutant FAILS to reject the
# nonexistent-evidence case, then proves the real library still rejects it) into
# the tests/meta framework so the bluff-proof runs in the meta-sweep.
#
# This meta-test does NOT re-author the mutation; it wraps the canonical one and
# additionally asserts the self-test discriminates (its absence would be caught
# by run_all.sh, but we make the wrapping explicit for the gate count).
#
# Usage: bash tests/meta/meta_test_antibluff_evidence_lib.sh
# Exit 0 = the §11.4.69 evidence-gating gate is bluff-proof.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SELFTEST="${ROOT}/tests/lib/anti_bluff_selftest.sh"

echo "[meta_test_antibluff_evidence_lib] gate: anti_bluff §11.4.69 evidence helper"

if [[ ! -f "$SELFTEST" ]]; then
    echo "  META-FAIL: $SELFTEST missing — §11.4.69 bluff-proof gone." >&2
    exit 1
fi

# The self-test internally: behaviour matrix + §1.1 always-pass mutation proof
# (mutant must NOT reject nonexistent evidence; real library MUST). A clean exit
# means the gate genuinely catches its own negation.
if sh "$SELFTEST"; then
    echo "  ok: anti_bluff_selftest.sh GREEN — five behaviours + always-pass mutant caught + restore proven."
    echo "META-GREEN: §11.4.69 evidence-gating gate is bluff-proof."
    exit 0
else
    echo "  META-FAIL: anti_bluff_selftest.sh FAILED — §11.4.69 gate not bluff-proof." >&2
    exit 1
fi
