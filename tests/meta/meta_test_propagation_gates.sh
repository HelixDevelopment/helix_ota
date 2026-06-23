#!/usr/bin/env bash
# =============================================================================
# meta_test_propagation_gates.sh — §1.1 meta-tests for the
#   CM-COVENANT-114-N-PROPAGATION anchor-presence gates (§11.4.157 lockstep).
# -----------------------------------------------------------------------------
# Each propagation gate in tests/pre_build_verification.sh is the literal
# invocation:
#       grep -qF '11.4.N' "${SCRIPT_DIR}/../CLAUDE.md"
# i.e. it PASSes iff the anchor literal `11.4.N` is present in the project
# context carrier CLAUDE.md, and FAILs iff it is absent. The pre-build audit
# (F-METAGATES) found these gates REAL-but-UNPAIRED: they pass on a clean tree
# but nothing PROVES they catch their own negation (a stripped anchor). A
# grep-presence gate with no paired mutation could be silently weakened (e.g.
# the grep target file edited away) and never noticed.
#
# This meta-test closes that hole for a representative SAMPLE across the gate
# range (early 11.4.153, middle 11.4.159, and the newest 11.4.166), proving for
# each that the EXACT gate command:
#   (a) PASSes on the clean carrier,
#   (b) FAILs when the carrier is mutated to strip the `11.4.N` anchor literal,
#   (c) PASSes again after byte-identical restore (§11.4.84, sha256-verified by
#       mt_restore_all).
#
# Subject-under-test is the REAL gate command run against the REAL CLAUDE.md —
# no inline replica of grep semantics, the literal gate is invoked. Mutation
# strips ALL occurrences of the anchor (sed g-flag) so a multi-occurrence
# carrier (CLAUDE.md repeats each anchor) genuinely loses the literal.
#
# Honest boundary (§11.4.6): the pre-build propagation gates only read CLAUDE.md
# (lines 37-50 of pre_build_verification.sh), so this meta-test mutates CLAUDE.md
# — the exact file the gate reads. AGENTS.md / GEMINI.md lockstep (§11.4.157) is
# a SEPARATE concern not currently wired as a pre-build gate; this meta-test
# scopes itself to what the pre-build gate actually enforces (no overclaim).
#
# FAST: pure grep + sed, no go test, no build.
#
# Usage: bash tests/meta/meta_test_propagation_gates.sh
# Exit 0 = every sampled CM-COVENANT-114-N-PROPAGATION gate proven bluff-proof.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=tests/meta/lib_metatest.sh
. "${SCRIPT_DIR}/lib_metatest.sh"

mt_init "meta_test_propagation_gates"

CLAUDE_MD="${ROOT}/CLAUDE.md"
[[ -f "$CLAUDE_MD" ]] || mt_fail "CLAUDE.md missing — propagation gates have no carrier to read."

# propagation_gate <anchor> — the EXACT command the pre-build gate runs.
# Returns 0 (gate PASS) iff the literal anchor is present in CLAUDE.md.
propagation_gate() {
    grep -qF "$1" "$CLAUDE_MD"
}

# Representative sample across the gate range. Each must be a literal anchor the
# pre-build wires a CM-COVENANT-114-N-PROPAGATION gate for (lines 37-50).
SAMPLE_ANCHORS="11.4.153 11.4.159 11.4.166"

for anchor in $SAMPLE_ANCHORS; do
    echo "  --- gate: CM-COVENANT-114-${anchor#11.4.}-PROPAGATION (literal ${anchor}) ---"

    # Sanity: the anchor must actually be present (else the gate is already
    # broken on the clean tree — a §11.4.157 propagation drift, not our concern
    # to silently paper over).
    mt_assert_gate_passes "anchor ${anchor} present on clean carrier" propagation_gate "$anchor"

    # Mutation: strip EVERY occurrence of the anchor literal from CLAUDE.md.
    # Escape the dots so sed treats them literally (defensive; '.' would match
    # any char but the replacement-with-nothing of the broader pattern still
    # removes the literal — escaping keeps the strip precise).
    esc="${anchor//./\\.}"
    mt_mutate_file "$CLAUDE_MD" "s/${esc}/STRIPPED_FOR_META/g"
    mt_assert_gate_fails  "gate FAILs when ${anchor} stripped" propagation_gate "$anchor"
    mt_restore_all
    mt_assert_gate_passes "gate PASSes after byte-identical restore (${anchor})" propagation_gate "$anchor"
done

echo "META-GREEN: sampled CM-COVENANT-114-{153,159,166}-PROPAGATION gates each catch a stripped anchor (bluff-proof)."
exit 0
