#!/usr/bin/env bash
# pre_build_verification.sh — Helix OTA pre-build / pre-merge gate
# aggregator. Runs the project's blocking invariants before a build,
# merge, or commit is allowed to proceed.
#
# Currently wired gates:
#   - Constitution inheritance (real + §1.1 mutation-proven).
#   - HelixQA bank-runner self-test (§1.1: the evidence ledger catches its
#     own negation — a missing-evidence challenge FAILs, a real one PASSes).
#
# Extend by appending more gate invocations below; keep every gate
# paired with a mutation per Constitution §1.1.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
rc=0

run_gate() {
    local name="$1"; shift
    echo ">>> gate: ${name}"
    if "$@"; then
        echo "<<< gate: ${name} OK"
    else
        echo "<<< gate: ${name} FAILED"
        rc=1
    fi
    echo
}

run_gate "constitution-inheritance" bash "${SCRIPT_DIR}/test_constitution_inheritance.sh"
run_gate "helixqa-bank-runner-self-test" bash "${SCRIPT_DIR}/../tools/helixqa/run_bank.sh" --self-test

# §11.4.153–§11.4.158 propagation gates (feature video recording + Status doc mandates).
# Each verifies the literal `11.4.15X` anchor is present in the project context carriers.
run_gate "CM-COVENANT-114-153-PROPAGATION" grep -qF '11.4.153' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-154-PROPAGATION" grep -qF '11.4.154' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-155-PROPAGATION" grep -qF '11.4.155' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-156-PROPAGATION" grep -qF '11.4.156' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-157-PROPAGATION" grep -qF '11.4.157' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-158-PROPAGATION" grep -qF '11.4.158' "${SCRIPT_DIR}/../CLAUDE.md"

if [[ "${rc}" -ne 0 ]]; then
    echo "PRE-BUILD VERIFICATION: FAIL"
    exit 1
fi
echo "PRE-BUILD VERIFICATION: PASS"
exit 0
