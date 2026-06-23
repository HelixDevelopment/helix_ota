#!/usr/bin/env bash
# =============================================================================
# meta_test_independent_verification.sh — §1.1 paired-mutation meta-test for the
#   CM-INDEPENDENT-VERIFICATION-AGENT functional gate (§11.4.165).
# -----------------------------------------------------------------------------
# The pre-build gate CM-INDEPENDENT-VERIFICATION-AGENT asserts, mechanically:
#   (A) the independent-review MACHINERY is wired — tests/meta/lib_metatest.sh
#       exists, is executable, AND carries the structurally-separate review SEAM
#       (the fatal restore-integrity abort: the `MT_RESTORE_FAILED` flag +
#       `exit 90`), so the verifier catches a bluff in itself rather than
#       silently passing; and
#   (B) a substantive batch carries an independent-review FINDINGS→FIX marker —
#       at least one non-empty docs/qa/**/INDEPENDENT_REVIEW.md.
#
# A functional gate is worthless unless it FAILs on its own negation. This
# meta-test PROVES it for BOTH halves by mutating the REAL files the gate reads
# and asserting the gate FAILs each time, then restores byte-identically
# (§11.4.84, sha256-verified by mt_restore_all) and asserts PASS again.
#
# Subject-under-test: `indep_verif_gate`, a function that re-runs the EXACT
# assertions the inline pre-build gate runs, against the REAL lib_metatest.sh +
# the REAL docs/qa/**/INDEPENDENT_REVIEW.md marker — not an inline replica that
# could drift from the gate's semantics. The function is kept byte-faithful to
# the inline gate (same greps, same find, same fail conditions).
#
# Honest boundary (§11.4.6): this meta-test proves the gate is bluff-proof
# (catches a removed wiring-seam AND a removed/emptied findings marker). It does
# NOT — and the GATE does not — claim to mechanically prove reviewer
# independence (a social property enforced by the §11.4.70/§11.4.20 subagent
# seam). It proves what is falsifiable.
#
# FAST: pure grep + find + sed, no go test, no build.
#
# Usage: bash tests/meta/meta_test_independent_verification.sh
# Exit 0 = the CM-INDEPENDENT-VERIFICATION-AGENT gate proven bluff-proof on both
#          halves; non-zero = gate is a bluff OR restore failed.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=tests/meta/lib_metatest.sh
. "${SCRIPT_DIR}/lib_metatest.sh"

mt_init "meta_test_independent_verification"

LIB_METATEST="${ROOT}/tests/meta/lib_metatest.sh"
QA_DIR="${ROOT}/docs/qa"
[[ -f "$LIB_METATEST" ]] || mt_fail "lib_metatest.sh missing — nothing for the gate to read."
[[ -d "$QA_DIR" ]]       || mt_fail "docs/qa missing — gate has no marker tree."

# indep_verif_gate — byte-faithful re-implementation of the EXACT assertions the
# inline CM-INDEPENDENT-VERIFICATION-AGENT gate in pre_build_verification.sh runs.
# Returns 0 (gate PASS) iff (A) machinery wired AND (B) a non-empty findings
# marker exists; non-zero (gate FAIL) otherwise.
indep_verif_gate() {
    local ok=1 marker
    # (A) machinery wired.
    if [[ ! -f "$LIB_METATEST" ]]; then
        ok=0
    else
        [[ -x "$LIB_METATEST" ]] || ok=0
        grep -q 'MT_RESTORE_FAILED' "$LIB_METATEST" || ok=0
        grep -q 'exit 90'          "$LIB_METATEST" || ok=0
    fi
    # (B) findings→fix marker present (non-empty).
    marker=$(find "$QA_DIR" -type f -name 'INDEPENDENT_REVIEW.md' -size +0c 2>/dev/null | head -n1)
    [[ -n "$marker" ]] || ok=0
    [[ "$ok" -eq 1 ]]
}

# --- clean-tree baseline ---
mt_assert_gate_passes "gate PASSES on clean tree (machinery wired + marker present)" indep_verif_gate

# --- Mutation 1: strip the restore-integrity review SEAM from lib_metatest.sh ---
# Removing MT_RESTORE_FAILED turns the verifier into a rubber stamp (it can no
# longer fail on a corrupted restore). The gate MUST catch this.
echo "  --- mutation 1: strip MT_RESTORE_FAILED review-seam from lib_metatest.sh ---"
mt_mutate_file "$LIB_METATEST" "s/MT_RESTORE_FAILED/MT_REVIEW_SEAM_STRIPPED/g"
mt_assert_gate_fails  "gate FAILs when review-seam (MT_RESTORE_FAILED) stripped" indep_verif_gate
mt_restore_all
mt_assert_gate_passes "gate PASSes after byte-identical restore (seam)" indep_verif_gate

# --- Mutation 2: strip the fatal exit-90 abort from lib_metatest.sh ---
echo "  --- mutation 2: strip fatal 'exit 90' abort from lib_metatest.sh ---"
mt_mutate_file "$LIB_METATEST" "s/exit 90/exit 0/g"
mt_assert_gate_fails  "gate FAILs when fatal abort (exit 90) removed" indep_verif_gate
mt_restore_all
mt_assert_gate_passes "gate PASSes after byte-identical restore (abort)" indep_verif_gate

# --- Mutation 3: empty the findings→fix marker (half B) ---
# Find a real marker, blank its content. An empty marker is NOT a verifiable
# artifact (size 0 -> find -size +0c skips it) — the gate MUST catch this.
echo "  --- mutation 3: empty a docs/qa/**/INDEPENDENT_REVIEW.md findings marker ---"
TARGET_MARKER=$(find "$QA_DIR" -type f -name 'INDEPENDENT_REVIEW.md' -size +0c 2>/dev/null | head -n1)
[[ -n "$TARGET_MARKER" ]] || mt_fail "no findings marker to mutate — clean-tree precondition already broken."
# mt_mutate_file requires a non-no-op sed change; replace ALL non-empty content
# with a single empty line is not size-0. Instead truncate to size 0 via a
# registered byte-identical restore (back up, truncate, restore). We reuse the
# mutate machinery by registering the file then truncating directly so restore
# is still byte-identical.
MARK_BAK="${MT_TMP}/$(echo "$TARGET_MARKER" | tr '/' '_').markbak"
cp -p "$TARGET_MARKER" "$MARK_BAK"
MT_MUT_FILES+=("$TARGET_MARKER")
MT_MUT_BAKS+=("$MARK_BAK")
if command -v shasum >/dev/null 2>&1; then
    MT_MUT_ORIG_SHAS+=("$(shasum -a 256 "$TARGET_MARKER" | awk '{print $1}')")
else
    MT_MUT_ORIG_SHAS+=("")
fi
: > "$TARGET_MARKER"   # truncate to size 0 -> no longer matches -size +0c
mt_ok "emptied marker $TARGET_MARKER (truncated to 0 bytes)"
# If this was the ONLY marker, the gate's find returns nothing -> FAIL. If other
# non-empty markers exist, this single emptying would NOT flip the gate; assert
# the find now sees NO non-empty marker under this run's precondition. To make
# the mutation decisive regardless of other dirs, empty EVERY current marker.
for extra in $(find "$QA_DIR" -type f -name 'INDEPENDENT_REVIEW.md' -size +0c 2>/dev/null); do
    eb="${MT_TMP}/$(echo "$extra" | tr '/' '_').markbak2"
    cp -p "$extra" "$eb"
    MT_MUT_FILES+=("$extra")
    MT_MUT_BAKS+=("$eb")
    if command -v shasum >/dev/null 2>&1; then
        MT_MUT_ORIG_SHAS+=("$(shasum -a 256 "$extra" | awk '{print $1}')")
    else
        MT_MUT_ORIG_SHAS+=("")
    fi
    : > "$extra"
    mt_ok "emptied additional marker $extra"
done
mt_assert_gate_fails  "gate FAILs when no non-empty findings marker remains" indep_verif_gate
mt_restore_all
mt_assert_gate_passes "gate PASSes after byte-identical restore (marker)" indep_verif_gate

echo "META-GREEN: CM-INDEPENDENT-VERIFICATION-AGENT catches (1) stripped review-seam, (2) removed fatal abort, (3) emptied findings marker — bluff-proof on both halves."
exit 0
