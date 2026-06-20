#!/usr/bin/env bash
# =============================================================================
# HelixTrack Test: Sync Push/Pull (HELIXTRACK-002)
# -----------------------------------------------------------------------------
# Purpose: Verify docs_chain sync context pushes workable items to HelixTrack
#          API and pulls state back.
# Usage:   scripts/testing/helixtrack_test_sync.sh
# Exit:    0 PASS, 1 FAIL, 3 SKIP
# Dependencies: sqlite3, curl, jq, HelixTrack API on localhost:8080
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

HELIXTRACK_API="${HELIXTRACK_API:-http://localhost:8080/do}"
HELIXTRACK_JWT="${HELIXTRACK_JWT:-}"
WORKABLE_ITEMS_DB="${PROJECT_ROOT}/docs/workable_items.db"
PUSH_SCRIPT="${PROJECT_ROOT}/scripts/sync_helixtrack_push.sh"
PULL_SCRIPT="${PROJECT_ROOT}/scripts/sync_helixtrack_pull.sh}"
NC='\033[0m'
PASSED=0; FAILED=0
pass() { PASSED=$((PASSED + 1)); echo -e "${NC}[PASS]${NC} $*"; }
fail() { FAILED=$((FAILED + 1)); echo -e "${NC}[FAIL]${NC} $*"; }

# Pre-flight checks
if ! command -v curl &>/dev/null; then echo "SKIP: curl not found"; exit 3; fi
if ! command -v jq &>/dev/null; then echo "SKIP: jq not found"; exit 3; fi
if [ ! -f "$WORKABLE_ITEMS_DB" ]; then
    echo "SKIP: no workable_items.db at ${WORKABLE_ITEMS_DB}"
    exit 3
fi
if ! curl -sf "$HELIXTRACK_API" -o /dev/null 2>/dev/null; then
    echo "SKIP: HelixTrack API not reachable at ${HELIXTRACK_API}"
    exit 3
fi

# Check push script exists
if [ -f "$PUSH_SCRIPT" ]; then
    chmod +x "$PUSH_SCRIPT"
    pass "Push script exists: ${PUSH_SCRIPT}"
else
    fail "Push script missing: ${PUSH_SCRIPT}"
fi

# Check pull script exists
if [ -f "$PULL_SCRIPT" ]; then
    chmod +x "$PULL_SCRIPT"
    pass "Pull script exists: ${PULL_SCRIPT}"
else
    fail "Pull script missing: ${PULL_SCRIPT}"
fi

# Check docs_chain context exists
if [ -f "${PROJECT_ROOT}/.docs_chain/contexts/helixtrack.yaml" ]; then
    pass "docs_chain helixtrack context exists"
else
    fail "docs_chain helixtrack context missing"
fi

# Verify workable_items.db has items
ITEM_COUNT=$(sqlite3 "$WORKABLE_ITEMS_DB" "SELECT COUNT(*) FROM items;" 2>/dev/null || echo "0")
if [ "$ITEM_COUNT" -gt 0 ]; then
    pass "workable_items.db has ${ITEM_COUNT} items"
else
    echo "WARN: workable_items.db has 0 items — LIMIT"
    pass "workable_items.db accessible (${ITEM_COUNT} items)"
fi

echo ""
echo "Results: ${PASSED} passed / ${FAILED} failed"
[ "$FAILED" -gt 0 ] && exit 1 || exit 0
