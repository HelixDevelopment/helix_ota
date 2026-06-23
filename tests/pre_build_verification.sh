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

# §11.4.153–§11.4.166 propagation gates (feature video recording + Status doc
# mandates + universal media-validation / verification / propagation mandates).
# Each verifies the literal `11.4.1XX` anchor is present in the project context carriers.
run_gate "CM-COVENANT-114-153-PROPAGATION" grep -qF '11.4.153' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-154-PROPAGATION" grep -qF '11.4.154' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-155-PROPAGATION" grep -qF '11.4.155' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-156-PROPAGATION" grep -qF '11.4.156' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-157-PROPAGATION" grep -qF '11.4.157' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-158-PROPAGATION" grep -qF '11.4.158' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-159-PROPAGATION" grep -qF '11.4.159' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-160-PROPAGATION" grep -qF '11.4.160' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-161-PROPAGATION" grep -qF '11.4.161' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-162-PROPAGATION" grep -qF '11.4.162' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-163-PROPAGATION" grep -qF '11.4.163' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-164-PROPAGATION" grep -qF '11.4.164' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-165-PROPAGATION" grep -qF '11.4.165' "${SCRIPT_DIR}/../CLAUDE.md"
run_gate "CM-COVENANT-114-166-PROPAGATION" grep -qF '11.4.166' "${SCRIPT_DIR}/../CLAUDE.md"

# ---- gate: CM-COVERAGE-MINIMUM ----
# Enforce minimum test coverage for server/internal packages.
echo ">>> gate: CM-COVERAGE-MINIMUM"
COVER_MIN=60
COVER_OUT=$(mktemp)
if (cd "${SCRIPT_DIR}/../server" && go test -coverprofile="$COVER_OUT" -coverpkg=./internal/... ./internal/... 2>&1); then
    COVER_PCT=$(cd "${SCRIPT_DIR}/../server" && go tool cover -func="$COVER_OUT" | grep '^total:' | awk '{print $3}' | sed 's/%//')
    COVER_INT=$(printf "%.0f" "$COVER_PCT" 2>/dev/null || echo 0)
    if [ "$COVER_INT" -lt "$COVER_MIN" ]; then
        echo "  COVERAGE ${COVER_PCT}% < ${COVER_MIN}% minimum"
        echo "<<< gate: CM-COVERAGE-MINIMUM FAIL"
        rc=1
    else
        echo "  Coverage ${COVER_PCT}% >= ${COVER_MIN}% minimum"
        echo "<<< gate: CM-COVERAGE-MINIMUM OK"
    fi
else
    echo "  go test failed (exit code $?)"
    echo "<<< gate: CM-COVERAGE-MINIMUM FAIL"
    rc=1
fi
rm -f "$COVER_OUT"

# ---- gate: CM-SEMGREP-WIRED (§11.4.166) ----
# Asserts (1) the commit_all.sh semgrep wiring exists and is NOT fail-open
# (an absent semgrep must NOT silently return clean), and (2) semgrep is on
# PATH. If semgrep is genuinely not installed, the gate reports OPERATOR-BLOCKED
# honestly (§11.4.6) rather than silently passing.
echo ">>> gate: CM-SEMGREP-WIRED"
COMMIT_ALL="${SCRIPT_DIR}/../scripts/commit_all.sh"
semgrep_wired_ok=1
if [[ ! -f "$COMMIT_ALL" ]]; then
    echo "  scripts/commit_all.sh missing — §11.4.166 wiring absent"
    semgrep_wired_ok=0
else
    # Wiring present: the semgrep scan function + the blocking invocation.
    if ! grep -q '_semgrep_scan_check' "$COMMIT_ALL"; then
        echo "  _semgrep_scan_check wiring removed from commit_all.sh (§11.4.166 hole)"
        semgrep_wired_ok=0
    fi
    if ! grep -q 'semgrep scan --config auto --error' "$COMMIT_ALL"; then
        echo "  blocking 'semgrep scan --config auto --error' invocation removed (§11.4.166 hole)"
        semgrep_wired_ok=0
    fi
    # Not fail-open: the not-installed branch must NOT silently 'return 0' clean.
    # The honest fix emits an error + (in block mode) returns 1.
    if ! grep -q 'semgrep NOT installed' "$COMMIT_ALL"; then
        echo "  semgrep not-installed path is fail-open (no honest blocker message) — §11.4.166 silent SKIP hole"
        semgrep_wired_ok=0
    fi
fi
if [[ "$semgrep_wired_ok" -eq 1 ]]; then
    if command -v semgrep >/dev/null 2>&1; then
        echo "  semgrep on PATH ($(command -v semgrep)); wiring present + not fail-open"
        echo "<<< gate: CM-SEMGREP-WIRED OK"
    else
        # Honest OPERATOR-BLOCKED, not a silent pass (§11.4.6 / §11.4.166).
        echo "  OPERATOR-BLOCKED: semgrep wiring present but semgrep NOT on PATH — install via constitution/scripts/semgrep/semgrep_setup.sh"
        echo "<<< gate: CM-SEMGREP-WIRED FAIL (OPERATOR-BLOCKED: semgrep install needed)"
        rc=1
    fi
else
    echo "<<< gate: CM-SEMGREP-WIRED FAIL (wiring broken / fail-open)"
    rc=1
fi
echo

# ---- gate: META-TESTS (§1.1 paired-mutation gate bluff-proofing) ----
# Runs every tests/meta/meta_test_*.sh — each PROVES a gate catches its own
# negation (mutate→FAIL→restore→PASS). A gate without a green meta-test here is
# not yet proven bluff-proof.
run_gate "meta-tests-bluff-proof" bash "${SCRIPT_DIR}/meta/run_all.sh"

if [[ "${rc}" -ne 0 ]]; then
    echo "PRE-BUILD VERIFICATION: FAIL"
    exit 1
fi
echo "PRE-BUILD VERIFICATION: PASS"
exit 0
