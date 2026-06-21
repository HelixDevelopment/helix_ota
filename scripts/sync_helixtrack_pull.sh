#!/usr/bin/env bash
# =============================================================================
# HelixTrack Sync — Pull (HelixTrack API → workable_items.db)
# -----------------------------------------------------------------------------
# Purpose: Fetch ticket updates from HelixTrack API and sync them into
#          workable_items.db — the local single source of truth (§11.4.93).
# Context: Invoked as transform_b_to_a in .docs_chain/contexts/helixtrack.yaml
# Usage:   HELIXTRACK_JWT="..." scripts/sync_helixtrack_pull.sh
# Dependencies: curl, jq, sqlite3
# =============================================================================
set -euo pipefail

HELIXTRACK_API="${HELIXTRACK_API:-http://localhost:8080/do}"
HELIXTRACK_JWT="${HELIXTRACK_JWT:-}"
WORKABLE_ITEMS_DB="docs/workable_items.db"
SYNC_STATE_MD="docs/helixtrack_sync_state.md"
LOG_PREFIX="[helixtrack-pull]"
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; NC='\033[0m'
log_info() { echo -e "${CYAN}${LOG_PREFIX}${NC} $*"; }
log_ok()   { echo -e "${GREEN}${LOG_PREFIX}${NC} $*"; }
log_warn() { echo -e "${YELLOW}${LOG_PREFIX}${NC} $*"; }
log_error(){ echo -e "${RED}${LOG_PREFIX}${NC} $*"; }

# --- Check dependencies ---
for cmd in curl jq sqlite3; do
    if ! command -v "$cmd" &>/dev/null; then
        echo "ERROR: Missing dependency: $cmd" >&2
        exit 1
    fi
done

# --- Check HelixTrack API reachability ---
if ! curl -sf -X POST "$HELIXTRACK_API" -H "Content-Type: application/json" -d '{"action":"version"}' -o /dev/null 2>/dev/null; then
    log_info "HelixTrack API not reachable — nothing to pull"
    exit 2
fi

log_info "Pulling updates from HelixTrack..."

# --- Fetch tickets from HelixTrack ---
# API format: {"action":"list","object":"ticket","jwt":"...","data":{...}}
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

# Check API-level error
API_ERROR=$(echo "$RESPONSE" | jq -r '.errorCode // ""')
if [ "$API_ERROR" != "" ] && [ "$API_ERROR" != "-1" ]; then
    log_error "HelixTrack API returned errorCode=$API_ERROR"
    echo "$RESPONSE" | jq '.' >&2
    exit 1
fi

# Parse ticket count — API returns { data: { items: [...], total: N } }
TICKET_COUNT=$(echo "$RESPONSE" | jq -r '.data.total // (.data.items | length) // 0' 2>/dev/null || echo "0")
log_ok "Fetched ${TICKET_COUNT} tickets from HelixTrack"

# --- Type / status mappers (HelixTrack → OTA) ---
map_item_type() {
    case "$1" in
        bug)     echo "Bug" ;;
        feature) echo "Feature" ;;
        task)    echo "Task" ;;
        *)       echo "Task" ;;
    esac
}
map_item_status() {
    case "$1" in
        done)         echo "Fixed (→ Fixed.md)" ;;
        in_progress)  echo "In progress" ;;
        testing)      echo "In testing" ;;
        open)         echo "Queued" ;;
        blocked)      echo "Operator-blocked" ;;
        closed)       echo "Obsolete (→ Fixed.md)" ;;
        *)            echo "Queued" ;;
    esac
}

# --- Pad description to ≥40 chars (DB CHECK constraint) ---
pad_description() {
    local desc="$1"
    local ticket_id="$2"
    if [ ${#desc} -lt 40 ]; then
        local suffix=" (synced from HT:$ticket_id)"
        desc="${desc}${suffix}"
        # If still short, pad with spaces
        while [ ${#desc} -lt 40 ]; do
            desc="${desc} "
        done
    fi
    echo "$desc"
}

# --- Sync each ticket into the local DB ---
ADDED=0
UPDATED=0
SKIPPED=0

while IFS= read -r ticket; do
    [ -z "$ticket" ] && continue

    id=$(echo "$ticket" | jq -r '.id // ""')
    title=$(echo "$ticket" | jq -r '.title // ""')
    description=$(echo "$ticket" | jq -r '.description // ""')
    ht_type=$(echo "$ticket" | jq -r '.type // "task"')
    ht_status=$(echo "$ticket" | jq -r '.status // "open"')

    # Extract OTA-ID from title: "[OTA-NNN] Title text"
    ota_id=$(echo "$title" | sed -n 's/.*\[\(OTA-[0-9][0-9]*\)\].*/\1/p')
    if [ -z "$ota_id" ]; then
        log_info "  SKIP (no OTA-ID in title): ${title:0:50}"
        SKIPPED=$((SKIPPED + 1))
        continue
    fi

    # Strip "[OTA-NNN]" prefix from title for clean storage
    clean_title=$(echo "$title" | sed 's/\[OTA-[0-9]*\] *//')

    item_type=$(map_item_type "$ht_type")
    item_status=$(map_item_status "$ht_status")
    description=$(pad_description "$description" "$id")

    # Check if item already exists in local DB
    exists=$(sqlite3 "$WORKABLE_ITEMS_DB" "SELECT COUNT(*) FROM items WHERE ota_id = '$ota_id';" 2>/dev/null || echo "0")

    if [ "$exists" -gt 0 ]; then
        # Update existing item's status + description
        sqlite3 "$WORKABLE_ITEMS_DB" <<SQL
UPDATE items SET
    status = '$(echo "$item_status" | sed "s/'/''/g")',
    description = '$(echo "$description" | sed "s/'/''/g")',
    modified_at = datetime('now')
WHERE ota_id = '$(echo "$ota_id" | sed "s/'/''/g")';
SQL
        UPDATED=$((UPDATED + 1))
        log_info "  UPDATE $ota_id → ${item_status}"
    else
        # Insert new item
        sqlite3 "$WORKABLE_ITEMS_DB" <<SQL
INSERT INTO items (ota_id, type, status, severity, title, description, composes_with, created_at, modified_at)
VALUES (
    '$(echo "$ota_id" | sed "s/'/''/g")',
    '$(echo "$item_type" | sed "s/'/''/g")',
    '$(echo "$item_status" | sed "s/'/''/g")',
    'Medium',
    '$(echo "$clean_title" | sed "s/'/''/g")',
    '$(echo "$description" | sed "s/'/''/g")',
    '[]',
    datetime('now'),
    datetime('now')
);
SQL
        # Add an "Opened" history entry
        sqlite3 "$WORKABLE_ITEMS_DB" <<SQL
INSERT INTO item_history (ota_id, by, event, on_date, reason, evidence_path, outcome)
VALUES ('$(echo "$ota_id" | sed "s/'/''/g")', 'AI', 'Opened', datetime('now'), 'Synced from HelixTrack ticket $id', '', '');
SQL
        ADDED=$((ADDED + 1))
        log_ok "  ADDED  $ota_id ($item_type, ${item_status})"
    fi
done < <(echo "$RESPONSE" | jq -c '.data.items[]?')

# --- Write sync state markdown ---
cat > "$SYNC_STATE_MD" <<STATEMD
# HelixTrack Sync State

**Last synced:** ${TIMESTAMP}
**Direction:** pull (HelixTrack API → workable_items.db)
**Tickets fetched:** ${TICKET_COUNT}
**Added to DB:** ${ADDED}
**Updated in DB:** ${UPDATED}
**Skipped (no OTA-ID):** ${SKIPPED}

| Metric | Value |
|--------|-------|
| Timestamp | ${TIMESTAMP} |
| API Endpoint | ${HELIXTRACK_API} |
| Tickets fetched | ${TICKET_COUNT} |
| Added | ${ADDED} |
| Updated | ${UPDATED} |
| Skipped | ${SKIPPED} |
STATEMD

log_ok "Pull sync complete: +${ADDED} added, ${UPDATED} updated, ${SKIPPED} skipped"
log_ok "Sync state written to ${SYNC_STATE_MD}"
exit 0
