#!/usr/bin/env bash
# inheritance_gate.sh — Helix OTA constitution-inheritance gate.
#
# Verifies that this project genuinely inherits from the Helix
# Constitution submodule mounted at `constitution/`. Wired into the
# pre-build / pre-merge pipeline.
#
# Invariants (Step 7 of the Constitution-Submodule-Setup runbook):
#   1. constitution/ exists with Constitution.md
#   2. Constitution.md contains the EXACT §11.4 forensic-anchor heading
#      line — the same sentinel the §1.1 paired mutation strips. We match
#      the full `### …` heading (NOT the bare substring) so that the
#      ToC copy of the same phrase cannot keep this gate green after the
#      mutation removes the heading. A bare-substring check would be a
#      bluff gate.
#   3. constitution/CLAUDE.md exists and carries the anti-bluff covenant.
#   4. constitution/AGENTS.md exists and carries the anti-bluff covenant.
#   5. Parent CLAUDE.md, AGENTS.md, and the project constitution all
#      reference the constitution submodule.
#
# Paired mutation: constitution/meta_test_inheritance.sh (Constitution §1.1).
# Exit 0 = all invariants hold; non-zero = at least one FAILed.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
CONST="${ROOT}/constitution"

# The §11.4 anchor heading — byte-identical to the SENTINEL_LINE in
# constitution/meta_test_inheritance.sh so the paired mutation provably
# flips this gate.
SENTINEL='### §11.4 End-user quality guarantee — forensic anchor (User mandate, 2026-04-28)'
CLAUDE_ANCHOR='MANDATORY ANTI-BLUFF COVENANT'
AGENTS_ANCHOR='Anti-bluff covenant'

fail=0
pass() { printf '  ✓ %s\n' "$1"; }
bad()  { printf '  ✗ %s\n' "$1"; fail=1; }

echo "Helix OTA — constitution inheritance gate"

# --- Invariant 1 ---
if [[ -f "${CONST}/Constitution.md" ]]; then
    pass "Inv1: constitution/Constitution.md exists"
else
    bad  "Inv1: constitution/Constitution.md MISSING"
fi

# --- Invariant 2 (anti-bluff sentinel) ---
if [[ -f "${CONST}/Constitution.md" ]] && grep -qF -- "${SENTINEL}" "${CONST}/Constitution.md"; then
    pass "Inv2: §11.4 forensic-anchor heading present"
else
    bad  "Inv2: §11.4 forensic-anchor heading MISSING"
fi

# --- Invariant 3 ---
if [[ -f "${CONST}/CLAUDE.md" ]] && grep -qF -- "${CLAUDE_ANCHOR}" "${CONST}/CLAUDE.md"; then
    pass "Inv3: constitution/CLAUDE.md anti-bluff covenant present"
else
    bad  "Inv3: constitution/CLAUDE.md anchor MISSING"
fi

# --- Invariant 4 ---
if [[ -f "${CONST}/AGENTS.md" ]] && grep -qF -- "${AGENTS_ANCHOR}" "${CONST}/AGENTS.md"; then
    pass "Inv4: constitution/AGENTS.md anti-bluff covenant present"
else
    bad  "Inv4: constitution/AGENTS.md anchor MISSING"
fi

# --- Invariant 5 (parent references the submodule) ---
check_ref() {
    local file="$1"
    if [[ -f "${file}" ]] && grep -qF -- "constitution/" "${file}"; then
        pass "Inv5: $(basename "${file}") references the constitution submodule"
    else
        bad  "Inv5: $(basename "${file}") does NOT reference the constitution submodule"
    fi
}
check_ref "${ROOT}/CLAUDE.md"
check_ref "${ROOT}/AGENTS.md"
check_ref "${ROOT}/docs/guides/HELIX_OTA_CONSTITUTION.md"

if [[ "${fail}" -ne 0 ]]; then
    echo "INHERITANCE GATE: FAIL"
    exit 1
fi
echo "INHERITANCE GATE: PASS"
exit 0
