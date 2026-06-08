#!/usr/bin/env bash
# test_constitution_inheritance.sh — host-side comprehensive proof that
# Helix OTA's constitution inheritance is real AND that its gate is not a
# bluff gate.
#
# Two assertions:
#   A. The inheritance gate PASSes on the clean working tree (Step 7/9).
#   B. The §1.1 paired mutation makes the gate FAIL: we hand our gate to
#      the constitution-side constitution/meta_test_inheritance.sh, which
#      strips the §11.4 anchor, runs our gate, asserts it FAILs, then
#      restores (Step 8 — anti-bluff, Constitution §1.1).
#
# Exit 0 only when BOTH hold.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
GATE="${ROOT}/tests/inheritance_gate.sh"
META="${ROOT}/constitution/meta_test_inheritance.sh"

rc=0

echo "=================================================================="
echo "A. Inheritance gate on clean tree (MUST PASS)"
echo "=================================================================="
if bash "${GATE}"; then
    echo "→ A PASS"
else
    echo "→ A FAIL: gate does not pass on a correctly-wired tree"
    rc=1
fi

echo
echo "=================================================================="
echo "B. §1.1 paired mutation (gate MUST FAIL under mutated Constitution)"
echo "=================================================================="
if [[ ! -f "${META}" ]]; then
    echo "→ B FAIL: ${META} missing (constitution submodule not initialised?)"
    rc=1
elif bash "${META}" "bash ${GATE}"; then
    echo "→ B PASS: gate correctly FAILed under mutation"
else
    echo "→ B FAIL: gate is a BLUFF GATE — it stayed green under mutation"
    rc=1
fi

echo
echo "=================================================================="
echo "C. Recursive owned-submodule inheritance pointers (runbook Step 9)"
echo "=================================================================="
OWNED_SUBMODULES=(
    submodules/ota-protocol
    submodules/ota-telemetry-schema
    submodules/ota-artifact-validator
    submodules/ota-rollout-engine
    submodules/ota-update-engine-bridge
    submodules/ota-android-agent
)
for sm in "${OWNED_SUBMODULES[@]}"; do
    d="${ROOT}/${sm}"
    if [[ ! -d "${d}" ]]; then
        echo "  ⊘ ${sm}: not checked out (skipped)"
        continue
    fi
    ok=1
    for f in CLAUDE.md AGENTS.md; do
        if [[ -f "${d}/${f}" ]] && grep -qiF "Helix Constitution" "${d}/${f}"; then :; else ok=0; fi
    done
    if [[ "${ok}" -eq 1 ]]; then
        echo "  ✓ ${sm}: CLAUDE.md + AGENTS.md reference the Helix Constitution"
    else
        echo "  ✗ ${sm}: missing/incomplete inheritance pointer"
        rc=1
    fi
done

echo
if [[ "${rc}" -eq 0 ]]; then
    echo "CONSTITUTION INHERITANCE: PASS (gate real + mutation-proven + submodules wired)"
else
    echo "CONSTITUTION INHERITANCE: FAIL"
fi
exit "${rc}"
