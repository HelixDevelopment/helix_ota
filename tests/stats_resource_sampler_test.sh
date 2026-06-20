#!/usr/bin/env bash
# =============================================================================
# Test: Resource Sampler Integration Test (§11.4.24)
# -----------------------------------------------------------------------------
# Purpose: Verify that the resource sampler starts, collects samples, stops,
#          appends to docs/Stats.tsv, and regenerates docs/Stats.md.
#
# Usage:
#   bash tests/stats_resource_sampler_test.sh
#
# Exit:
#   0  ALL PASS
#   1  Any FAIL
#   3  SKIP (precondition not met)
#
# Dependencies:
#   scripts/resource_sampler.sh
#   scripts/generate_stats_report.sh
#   ps(1), uptime(1), iostat(1)
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
SAMPLER="${PROJECT_ROOT}/scripts/resource_sampler.sh"
GENERATOR="${PROJECT_ROOT}/scripts/generate_stats_report.sh"
REGISTRY="${PROJECT_ROOT}/docs/Stats.tsv"
REPORT="${PROJECT_ROOT}/docs/Stats.md"
RAW_DIR="${PROJECT_ROOT}/qa-results/stats"
PID_FILE="${PROJECT_ROOT}/.resource_sampler.pid"

PASSED=0
FAILED=0

# ─── helper ──────────────────────────────────────────────────────────────────

pass() { PASSED=$((PASSED + 1)); echo "  [PASS] $*"; }
fail() { FAILED=$((FAILED + 1)); echo "  [FAIL] $*"; }

cleanup() {
    # Stop any running sampler
    if [ -f "${PID_FILE}" ]; then
        kill "$(cat "${PID_FILE}")" 2>/dev/null || true
        rm -f "${PID_FILE}"
    fi
    # Remove any raw files created by this test run
    rm -f "${RAW_DIR}/test_sampler_"*.tsv
}
trap cleanup EXIT

# ─── pre-flight ──────────────────────────────────────────────────────────────

if [ ! -x "${SAMPLER}" ]; then
    echo "SKIP: ${SAMPLER} not found or not executable"
    exit 3
fi

if [ ! -x "${GENERATOR}" ]; then
    echo "SKIP: ${GENERATOR} not found or not executable"
    exit 3
fi

echo ""
echo "=== Resource Sampler Integration Test ==="
echo ""

# ─── 1. Backup existing registry ──────────────────────────────────────────────

REGISTRY_BACKUP=""
if [ -f "${REGISTRY}" ]; then
    REGISTRY_BACKUP="$(mktemp)"
    cp "${REGISTRY}" "${REGISTRY_BACKUP}"
    echo "  Backed up existing ${REGISTRY}"
fi

REPORT_BACKUP=""
if [ -f "${REPORT}" ]; then
    REPORT_BACKUP="$(mktemp)"
    cp "${REPORT}" "${REPORT_BACKUP}"
    echo "  Backed up existing ${REPORT}"
fi

# ─── 2. Start the sampler ─────────────────────────────────────────────────────

echo ""
echo "--- Test 1: Start sampler ---"
if bash "${SAMPLER}" start "test_sampler" 2>&1; then
    pass "Sampler started successfully"
else
    fail "Sampler failed to start"
    # Restore backups and exit
    [ -n "${REGISTRY_BACKUP}" ] && cp "${REGISTRY_BACKUP}" "${REGISTRY}"
    [ -n "${REPORT_BACKUP}" ] && cp "${REPORT_BACKUP}" "${REPORT}"
    exit 1
fi

# Verify PID file
sleep 1
if [ -f "${PID_FILE}" ]; then
    PID="$(cat "${PID_FILE}")"
    if kill -0 "${PID}" 2>/dev/null; then
        pass "Sampler PID ${PID} is running"
    else
        fail "Sampler PID ${PID} not running"
    fi
else
    fail "PID file ${PID_FILE} not created"
fi

# ─── 3. Sample for 10 seconds ────────────────────────────────────────────────

echo ""
echo "--- Test 2: Collect samples for 10 seconds ---"
sleep 10

# Count raw TSV files
RAW_COUNT="$(ls "${RAW_DIR}"/test_sampler_*.tsv 2>/dev/null | wc -l | tr -d ' ')"
if [ "${RAW_COUNT}" -ge 1 ]; then
    pass "Raw TSV file(s) created: ${RAW_COUNT}"
else
    fail "No raw TSV files created in ${RAW_DIR}"
fi

# Latest raw file
RAW_FILE="$(ls -t "${RAW_DIR}"/test_sampler_*.tsv 2>/dev/null | head -1)"
if [ -n "${RAW_FILE}" ] && [ -f "${RAW_FILE}" ]; then
    SAMPLE_COUNT="$(awk 'NR>1' "${RAW_FILE}" | wc -l | tr -d ' ')"
    if [ "${SAMPLE_COUNT}" -ge 1 ]; then
        pass "Raw file has ${SAMPLE_COUNT} sample(s)"
    else
        fail "Raw file is empty (no data rows)"
    fi
else
    fail "Could not find raw TSV file"
fi

# ─── 4. Stop the sampler ─────────────────────────────────────────────────────

echo ""
echo "--- Test 3: Stop sampler ---"
if bash "${SAMPLER}" stop "SUCCESS" 2>&1; then
    pass "Sampler stopped successfully"
else
    fail "Sampler stop failed"
fi

# Verify PID file removed
if [ ! -f "${PID_FILE}" ]; then
    pass "PID file cleaned up"
else
    fail "PID file still exists"
fi

# ─── 5. Verify registry (docs/Stats.tsv) ─────────────────────────────────────

echo ""
echo "--- Test 4: Verify docs/Stats.tsv ---"
if [ -f "${REGISTRY}" ]; then
    pass "Stats.tsv exists"

    # Check header
    HEADER="$(head -1 "${REGISTRY}")"
    if echo "${HEADER}" | grep -q "label"; then
        pass "Stats.tsv has header row"
    else
        fail "Stats.tsv missing header row"
    fi

    # Check for test_sampler row
    if grep -q "test_sampler" "${REGISTRY}"; then
        pass "Stats.tsv contains test_sampler row"
    else
        fail "Stats.tsv missing test_sampler row (got: '$(tail -1 "${REGISTRY}")')"
    fi

    # Check columns: should have 11 tab-separated fields
    COL_COUNT="$(tail -1 "${REGISTRY}" | awk -F'\t' '{print NF}')"
    if [ "${COL_COUNT}" -eq 11 ]; then
        pass "Stats.tsv row has 11 columns"
    else
        fail "Stats.tsv row has ${COL_COUNT} columns, expected 11"
    fi
else
    fail "Stats.tsv not created"
fi

# ─── 6. Verify report (docs/Stats.md) ────────────────────────────────────────

echo ""
echo "--- Test 5: Verify docs/Stats.md ---"
if [ -f "${REPORT}" ]; then
    pass "Stats.md exists"

    if grep -q "Ever-values" "${REPORT}"; then
        pass "Stats.md has Ever-values section"
    else
        fail "Stats.md missing Ever-values section"
    fi

    if grep -q "Per-build" "${REPORT}"; then
        pass "Stats.md has Per-build section"
    else
        fail "Stats.md missing Per-build section"
    fi

    if grep -q "test_sampler" "${REPORT}"; then
        pass "Stats.md contains test_sampler entry"
    else
        fail "Stats.md missing test_sampler entry"
    fi
else
    fail "Stats.md not created"
fi

# ─── 7. Verify the report has proper revision header ─────────────────────────

echo ""
echo "--- Test 6: Verify §11.4.44 revision header ---"
if head -5 "${REPORT}" 2>/dev/null | grep -q "Revision:"; then
    pass "Stats.md has Revision header"
else
    fail "Stats.md missing Revision header"
fi

# ─── 8. Verify raw file format ───────────────────────────────────────────────

echo ""
echo "--- Test 7: Verify raw TSV format ---"
if [ -n "${RAW_FILE:-}" ] && [ -f "${RAW_FILE}" ]; then
    RAW_HEADER="$(head -1 "${RAW_FILE}")"
    if echo "${RAW_HEADER}" | grep -q "timestamp"; then
        pass "Raw TSV has correct header"
    else
        fail "Raw TSV header wrong: ${RAW_HEADER}"
    fi
fi

# ─── restore backups ─────────────────────────────────────────────────────────

echo ""
echo "=== Summary ==="
echo "  PASSED: ${PASSED}"
echo "  FAILED: ${FAILED}"

if [ "${FAILED}" -eq 0 ]; then
    echo "  RESULT: ALL PASS"
else
    echo "  RESULT: SOME FAILED"
fi

[ -n "${REGISTRY_BACKUP}" ] && cp "${REGISTRY_BACKUP}" "${REGISTRY}" && rm -f "${REGISTRY_BACKUP}"
[ -n "${REPORT_BACKUP}" ] && cp "${REPORT_BACKUP}" "${REPORT}" && rm -f "${REPORT_BACKUP}"
# Remove test raw files
rm -f "${RAW_DIR}/test_sampler_"*.tsv

[ "${FAILED}" -eq 0 ] && exit 0 || exit 1
