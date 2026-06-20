#!/usr/bin/env bash
# =============================================================================
# HelixTrack Sync — Pull (HelixTrack API → workable_items.db)
# -----------------------------------------------------------------------------
# Purpose: Fetch ticket updates from HelixTrack API and write sync state
#          markdown for docs_chain to ingest back into workable_items.db.
# Context: Invoked as transform_b_to_a in .docs_chain/contexts/helixtrack.yaml
# Usage:   scripts/sync_helixtrack_pull.sh
# Dependencies: curl, jq
# =============================================================================
set -euo pipefail

HELIXTRACK_API="${HELIXTRACK_API:-http://localhost:8080/do}"
HELIXTRACK_JWT="${HELIXTRACK_JWT:-}"
SYNC_STATE_MD="docs/helixtrack_sync_state.md"
LOG_PREFIX="[helixtrack-pull]"
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; NC='\033[0m'
log_info() { echo -e "${CYAN}${LOG_PREFIX}${NC} $*"; }
log_ok()   { echo -e "${GREEN}${LOG_PREFIX}${NC} $*"; }
log_error(){ echo -e "${RED}${LOG_PREFIX}${NC} $*"; }

for cmd in curl jq; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: Missing dependency: $cmd" >&2
        exit 1
    fi
done

# Check API reachability
if ! curl -sf "$HELIXTRACK_API" -o /dev/null 2>/dev/null; then
    log_info "HelixTrack API not reachable — nothing to pull"
    exit 2
fi

log_info "Pulling updates from HelixTrack..."

# Fetch tickets from HelixTrack
LIST_PAYLOAD=$(jq -n \
    --arg action "list" \
    --arg object "ticket" \
    --arg jwt "$HELIXTRACK_JWT" \
    '{
        action: $action,
        jwt: $jwt,
        object: $object,
        data: { external_system: "helix_ota", limit: 100 }
    }'
)

RESPONSE=$(curl -sf -X POST "$HELIXTRACK_API" \
    -H "Content-Type: application/json" \
    -d "$LIST_PAYLOAD" 2>/dev/null) || {
    log_error "Failed to fetch tickets from HelixTrack API"
    exit 1
}

# Parse response
TICKET_COUNT=$(echo "$RESPONSE" | jq -r '.data.tickets | length // .data | length // 0' 2>/dev/null || echo "unknown")

# Write sync state
cat > "$SYNC_STATE_MD" <<STATEMD
# HelixTrack Sync State

**Last synced:** ${TIMESTAMP}
**Direction:** pull (HelixTrack API → workable_items.db)
**Tickets fetched:** ${TICKET_COUNT}

| Metric | Value |
|--------|-------|
| Timestamp | ${TIMESTAMP} |
| API Endpoint | ${HELIXTRACK_API} |
| Tickets | ${TICKET_COUNT} |

\`\`\`json
$(echo "$RESPONSE" | jq '.' 2>/dev/null || echo "{}")
\`\`\`
STATEMD

log_ok "Pull state written to ${SYNC_STATE_MD}"
exit 0
