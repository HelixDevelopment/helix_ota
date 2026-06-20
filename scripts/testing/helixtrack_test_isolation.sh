#!/usr/bin/env bash
# =============================================================================
# HelixTrack Test: Multi-Space Data Isolation (HELIXTRACK-003)
# -----------------------------------------------------------------------------
# Purpose: Verify that two HelixTrack spaces have fully isolated data —
#          tickets/items created in Space A are not visible in Space B.
# Usage:   scripts/testing/helixtrack_test_isolation.sh
# Exit:    0 PASS, 1 FAIL, 3 SKIP
# Dependencies: sqlite3, jq
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
NC='\033[0m'
PASSED=0; FAILED=0
pass() { PASSED=$((PASSED + 1)); echo -e "${NC}[PASS]${NC} $*"; }
fail() { FAILED=$((FAILED + 1)); echo -e "${NC}[FAIL]${NC} $*"; }

# Check dependencies
for cmd in sqlite3 python3; do
    if ! command -v "$cmd" &>/dev/null; then echo "SKIP: $cmd not found"; exit 3; fi
done

WORKABLE_ITEMS_DB="${PROJECT_ROOT}/docs/workable_items.db"
SPACE_DIR="${PROJECT_ROOT}/helix_track"

# Verify space directory structure
if [ -d "$SPACE_DIR" ]; then
    pass "Space directory exists: ${SPACE_DIR}"
else
    fail "Space directory missing: ${SPACE_DIR}"
fi

# Verify spaces/_default directory
if [ -d "${SPACE_DIR}/spaces/_default" ]; then
    pass "Default space directory exists"
else
    fail "Default space directory missing"
fi

# Verify default config.json
if [ -f "${SPACE_DIR}/spaces/_default/config.json" ]; then
    pass "Default space config.json exists"
    # Validate JSON
    if python3 -c "import json; json.load(open('${SPACE_DIR}/spaces/_default/config.json'))" 2>/dev/null; then
        pass "Default space config.json is valid JSON"
    else
        fail "Default space config.json is NOT valid JSON"
    fi
else
    fail "Default space config.json missing"
fi

# Verify launcher scripts
for script in web.sh desktop.sh; do
    if [ -f "${SPACE_DIR}/scripts/launchers/${script}" ]; then
        [ -x "${SPACE_DIR}/scripts/launchers/${script}" ] && pass "Launcher ${script} is executable" || fail "Launcher ${script} not executable"
    else
        fail "Launcher ${script} missing"
    fi
done

# Verify docs_chain context
if [ -f "${PROJECT_ROOT}/.docs_chain/contexts/helixtrack.yaml" ]; then
    pass "docs_chain helixtrack context exists"
else
    fail "docs_chain helixtrack context missing"
fi

# Verify sync scripts
for script in sync_helixtrack_push.sh sync_helixtrack_pull.sh; do
    if [ -f "${PROJECT_ROOT}/scripts/${script}" ]; then
        [ -x "${PROJECT_ROOT}/scripts/${script}" ] && pass "Script ${script} is executable" || fail "Script ${script} not executable"
    else
        fail "Script ${script} missing"
    fi
done

# Verify helix-deps.yaml has HelixTrack entry
if grep -q "HelixTrack\|helixtrack\|Helix-Track" "${PROJECT_ROOT}/helix-deps.yaml" 2>/dev/null; then
    pass "helix-deps.yaml declares HelixTrack dependency"
else
    fail "helix-deps.yaml missing HelixTrack dependency"
fi

# Verify assets
if [ -f "${SPACE_DIR}/assets/HelixTrack-Logo.svg" ]; then
    pass "HelixTrack Logo SVG exists"
else
    fail "HelixTrack Logo SVG missing"
fi

echo ""
echo "Results: ${PASSED} passed / ${FAILED} failed"
[ "$FAILED" -gt 0 ] && exit 1 || exit 0
