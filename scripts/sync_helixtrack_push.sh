#!/usr/bin/env bash
# =============================================================================
# HelixTrack Sync — Push (workable_items.db → HelixTrack API)
# -----------------------------------------------------------------------------
# Purpose: Read modified workable items from the local SQLite SSoT and create
#          or update corresponding tickets in the HelixTrack API.
# Context: Invoked as transform_a_to_b in .docs_chain/contexts/helixtrack.yaml
# Usage:   scripts/sync_helixtrack_push.sh
# Dependencies: sqlite3, curl, jq
# =============================================================================
set -euo pipefail

# --- Configuration ---
HELIXTRACK_API="${HELIXTRACK_API:-http://localhost:8080/do}"
HELIXTRACK_JWT="${HELIXTRACK_JWT:-}"
# Per helix-deps.yaml (§11.4.31), the helixtrack submodule has layout: flat,
# so it lives at <project_root>/helixtrack/.  Fall back to empty when the
# submodule is not checked out (API-only create will be used instead).
HELIXTRACK_DB="${HELIXTRACK_DB:-helixtrack/core/Application/Database/Definition.sqlite}"
WORKABLE_ITEMS_DB="docs/workable_items.db"
SYNC_STATE_MD="docs/helixtrack_sync_state.md"
LOG_PREFIX="[helixtrack-push]"

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()  { echo -e "${CYAN}${LOG_PREFIX}${NC} $*"; }
log_ok()    { echo -e "${GREEN}${LOG_PREFIX}${NC} $*"; }
log_warn()  { echo -e "${YELLOW}${LOG_PREFIX}${NC} $*"; }
log_error() { echo -e "${RED}${LOG_PREFIX}${NC} $*"; }

# --- Check dependencies ---
for cmd in sqlite3 curl jq; do
    if ! command -v "$cmd" &>/dev/null; then
        log_error "Missing dependency: $cmd"
        exit 1
    fi
done

if [ ! -f "$WORKABLE_ITEMS_DB" ]; then
    log_warn "No workable items DB at ${WORKABLE_ITEMS_DB} — nothing to push"
    exit 0
fi

# --- Check HelixTrack API reachability ---
if ! curl -sf -X POST "$HELIXTRACK_API" -H "Content-Type: application/json" -d '{"action":"version"}' -o /dev/null 2>/dev/null; then
    log_warn "HelixTrack API not reachable at ${HELIXTRACK_API} — SKIP"
    exit 2  # OPERATOR-BLOCKED signal
fi

log_info "Pushing workable items to HelixTrack..."

# --- Push each item ---
PUSHED=0
FAILED=0
TIMESTAMP=$(date -u +%Y-%m-%dT%H:%M:%SZ)

process_item() {
    local ota_id="$1"
    local item_type="$2"
    local status="$3"
    local title="$4"
    local description="$5"

    # Map OTA item type to HelixTrack ticket type
    # HelixTrack uses title-based lookup: values must match DB ticket_type.title (lowercase)
    case "$item_type" in
        Bug)     HT_TYPE="bug" ;;
        Feature) HT_TYPE="feature" ;;
        Task)    HT_TYPE="task" ;;
        *)       HT_TYPE="task" ;;
    esac

    # Map OTA status to HelixTrack ticket status
    # Statuses must match DB ticket_status.title (lowercase)
    case "$status" in
        "In progress")                 HT_STATUS="in_progress" ;;
        "Ready for testing")           HT_STATUS="in_progress" ;;
        "In testing")                  HT_STATUS="testing" ;;
        "Fixed (→ Fixed.md)")          HT_STATUS="done" ;;
        "Implemented (→ Fixed.md)")    HT_STATUS="done" ;;
        "Completed (→ Fixed.md)")      HT_STATUS="done" ;;
        "Obsolete (→ Fixed.md)")       HT_STATUS="closed" ;;
        "Operator-blocked")            HT_STATUS="blocked" ;;
        Queued)                        HT_STATUS="open" ;;
        Reopened)                      HT_STATUS="in_progress" ;;
        *)                             HT_STATUS="open" ;;
    esac

    local existing_id=""

    # Use OTA-prefixed title for cross-reference
    local prefixed_title="[${ota_id}] ${title}"

    # Query HelixTrack DB to check if ticket with this OTA-ID exists
    existing_id=$(sqlite3 "$HELIXTRACK_DB" "SELECT id FROM ticket WHERE INSTR(title, '${ota_id}') > 0 LIMIT 1;" 2>/dev/null | tr -d ' ' || echo "")

    if [ -n "$existing_id" ]; then
        # Ticket exists — modify it
        log_info "${ota_id} found — modifying..."
        PAYLOAD=$(jq -n \
            --arg action "modify" \
            --arg object "ticket" \
            --arg jwt "$HELIXTRACK_JWT" \
            --arg id "$existing_id" \
            --arg type "$HT_TYPE" \
            --arg status "$HT_STATUS" \
            --arg title "$prefixed_title" \
            --arg description "$description" \
            --arg ota_id "$ota_id" \
            '{
                action: $action, jwt: $jwt, object: $object,
                data: {
                    id: $id, title: $title, description: $description,
                    type: $type, status: $status,
                    project_id: "ota-helix-001",
                    external_id: $ota_id, external_system: "helix_ota"
                }
            }'
        )
    else
        # Ticket doesn't exist — create
        PAYLOAD=$(jq -n \
            --arg action "create" \
            --arg object "ticket" \
            --arg jwt "$HELIXTRACK_JWT" \
            --arg ota_id "$ota_id" \
            --arg type "$HT_TYPE" \
            --arg status "$HT_STATUS" \
            --arg title "$prefixed_title" \
            --arg description "$description" \
            '{
                action: $action, jwt: $jwt, object: $object,
                data: {
                    title: $title, description: $description,
                    type: $type, status: $status,
                    project_id: "ota-helix-001",
                    external_id: $ota_id, external_system: "helix_ota",
                    assignee: "admin"
                }
            }'
        )
    fi

    RESPONSE=$(curl -s -X POST "$HELIXTRACK_API" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD" 2>/dev/null) || true

    ERR=$(echo "$RESPONSE" | jq -r '.errorCode // ""' 2>/dev/null || echo "")
    if [ "$ERR" = "-1" ]; then
        # Success — now sync status into HelixTrack DB (the create handler hardcodes "open")
        # NOTE: SQLite LIKE interprets [ as a character class — use INSTR instead
        if [ -f "$HELIXTRACK_DB" ]; then
            sqlite3 "$HELIXTRACK_DB" "
                UPDATE ticket SET ticket_status_id = (SELECT id FROM ticket_status WHERE title = '${HT_STATUS}' LIMIT 1)
                WHERE INSTR(title, '${ota_id}') > 0 AND deleted = 0;
                UPDATE ticket SET ticket_type_id = (SELECT id FROM ticket_type WHERE title = '${HT_TYPE}' LIMIT 1)
                WHERE INSTR(title, '${ota_id}') > 0 AND deleted = 0;
            " 2>/dev/null || true
        fi
        log_ok "Pushed [${ota_id}] ${title}"
        PUSHED=$((PUSHED + 1))
        return 0
    else
        log_error "Failed to push [${ota_id}] ${title} (errorCode=$ERR)"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# Read items from SQLite and process each (process substitution avoids subshell)
while IFS='|' read -r id type status title desc; do
    process_item "$id" "$type" "$status" "$title" "$desc" || true
done < <(sqlite3 "$WORKABLE_ITEMS_DB" "SELECT ota_id, type, status, title, COALESCE(description, '') FROM items;")

# --- Write sync state markdown ---
cat > "$SYNC_STATE_MD" <<STATEMD
# HelixTrack Sync State

**Last synced:** ${TIMESTAMP}
**Direction:** push (workable_items.db → HelixTrack API)
**Pushed:** ${PUSHED}
**Failed:** ${FAILED}

| Status | Count |
|--------|-------|
| Pushed | ${PUSHED} |
| Failed | ${FAILED} |
| Total  | $((PUSHED + FAILED)) |

STATEMD

log_info "Sync state written to ${SYNC_STATE_MD}"

if [ "$FAILED" -gt 0 ]; then
    log_warn "${FAILED} items failed — check HelixTrack API"
    exit 1
fi

log_ok "Push complete — ${PUSHED} items synced"
exit 0
