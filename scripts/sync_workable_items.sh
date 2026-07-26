#!/usr/bin/env bash
# =============================================================================
# sync_workable_items.sh — Automated DB Sync Pipeline
# -----------------------------------------------------------------------------
# Cross-references git log commit messages against the workable-items SQLite DB.
# Auto-updates statuses for items referenced in commits by parsing OTA-IDs from
# commit messages and inferring the transition from the commit's keyword prefix.
#
# Transition inference:
#   fix(…) / fix: / "fix "         → Bug → "Fixed (→ Fixed.md)"
#   feat(…) / feat:                → Feature/Task → "Implemented (→ Fixed.md)"
#   chore: close / "close "        → Task → "Completed (→ Fixed.md)"
#   Reverts / "revert"             → Reopened
#
# A sync-marker tag (db-sync-marker) is placed after a successful run so the
# next invocation only scans new commits.  When --auto is used only the named
# OTA-IDs are processed regardless of git-log scan.
#
# Usage:
#   bash scripts/sync_workable_items.sh              # scan since last marker
#   bash scripts/sync_workable_items.sh --dry-run    # preview, no writes
#   bash scripts/sync_workable_items.sh --auto OTA-001 OTA-002   # named IDs
#   bash scripts/sync_workable_items.sh --since HEAD~10          # custom range
#   bash scripts/sync_workable_items.sh --full                  # full history
#
# Inputs:
#   - docs/workable_items.db (SQLite, schema: items + item_history)
#   - git log between last db-sync-marker tag and HEAD
#
# Outputs:
#   - DB UPDATEs on items table (status + modified_at)
#   - INSERTs into item_history table
#   - Sync log written to docs/research/production_planning_20260726/sync_logs/
#
# Dependencies: git, sqlite3
#
# Cross-references:
#   - §11.4.148 — HelixTrack auto-sync mandate (DB counterpart)
#   - §11.4.93  — Workable-items status tracking
#   - scripts/git_hooks/post-commit — calls this with --auto
#
# Last verified: 2026-07-26
# =============================================================================
set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DB="$PROJECT_ROOT/docs/workable_items.db"
SYNC_LOG_DIR="$PROJECT_ROOT/docs/research/production_planning_20260726/sync_logs"
RUN_ID="$(date +%Y%m%d-%H%M%S)"
SYNC_LOG="$SYNC_LOG_DIR/sync_${RUN_ID}.log"

DRY_RUN=false
AUTO_MODE=false
FULL_SCAN=false
SINCE_RANGE=""
AUTO_IDS=()

# --- parse args ---------------------------------------------------------------
while [[ $# -gt 0 ]]; do
    case "$1" in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --auto)
            AUTO_MODE=true
            shift
            while [[ $# -gt 0 && "$1" != --* ]]; do
                AUTO_IDS+=("$1")
                shift
            done
            ;;
        --full)
            FULL_SCAN=true
            shift
            ;;
        --since)
            SINCE_RANGE="$2"
            shift 2
            ;;
        *)
            echo "Unknown flag: $1" >&2
            exit 1
            ;;
    esac
done

# --- helpers ------------------------------------------------------------------
log_msg() {
    local ts
    ts="$(date -Iseconds)"
    echo "[${ts}] $*" >&2
    echo "[${ts}] $*" >> "$SYNC_LOG"
}

db_exists() {
    [ -f "$DB" ] || { log_msg "ERROR: DB not found at $DB"; exit 1; }
}

# --- resolve commit range -----------------------------------------------------
resolve_commits() {
    if $FULL_SCAN; then
        git -C "$PROJECT_ROOT" log --format="%H %s" --reverse
        return
    fi
    if [ -n "$SINCE_RANGE" ]; then
        git -C "$PROJECT_ROOT" log --format="%H %s" "$SINCE_RANGE"
        return
    fi
    local marker
    marker=$(git -C "$PROJECT_ROOT" tag -l 'db-sync-marker' 2>/dev/null || true)
    if [ -n "$marker" ]; then
        local range="${marker}..HEAD"
        local count
        count=$(git -C "$PROJECT_ROOT" rev-list --count "$range" 2>/dev/null || echo 0)
        if [ "$count" -eq 0 ]; then
            log_msg "No new commits since marker $marker — nothing to sync"
            exit 0
        fi
        log_msg "Scanning $count commits since marker $marker"
        git -C "$PROJECT_ROOT" log --format="%H %s" "$range"
    else
        log_msg "No sync marker found — scanning last 50 commits"
        git -C "$PROJECT_ROOT" log --format="%H %s" -n 50
    fi
}

# --- infer status transition from commit message ------------------------------
infer_transition() {
    local msg="$1"
    local item_type="$2"

    # Detect fix
    if echo "$msg" | grep -qiE '(^fix|^bug\s*fix|^hotfix|fix\s*:|bug\s*fix:)'; then
        case "$item_type" in
            Bug)     echo "Fixed (→ Fixed.md)" ;;
            Feature) echo "Implemented (→ Fixed.md)" ;;
            Task)    echo "Completed (→ Fixed.md)" ;;
            *)       echo "Completed (→ Fixed.md)" ;;
        esac
        return
    fi

    # Detect feat/feature
    if echo "$msg" | grep -qiE '(^feat|^feature)'; then
        case "$item_type" in
            Feature) echo "Implemented (→ Fixed.md)" ;;
            Bug)     echo "Fixed (→ Fixed.md)" ;;
            Task)    echo "Completed (→ Fixed.md)" ;;
            *)       echo "Completed (→ Fixed.md)" ;;
        esac
        return
    fi

    # Detect explicit close/chore-done
    if echo "$msg" | grep -qiE '(close|complete|done|resolve|finish)'; then
        case "$item_type" in
            Bug)     echo "Fixed (→ Fixed.md)" ;;
            Feature) echo "Implemented (→ Fixed.md)" ;;
            Task)    echo "Completed (→ Fixed.md)" ;;
            *)       echo "Completed (→ Fixed.md)" ;;
        esac
        return
    fi

    # Detect revert
    if echo "$msg" | grep -qiE '(revert|rollback)'; then
        echo "Reopened"
        return
    fi

    # Default for anything mentioning an OTA-ID — mark as Completed
    echo "Completed (→ Fixed.md)"
}

# --- process a single OTA-ID --------------------------------------------------
process_ota_id() {
    local ota_id="$1"
    local source_commit="$2"
    local commit_msg="$3"

    # Look up the item
    local item_type current_status
    item_type=$(sqlite3 "$DB" "SELECT type FROM items WHERE ota_id='$ota_id';" 2>/dev/null || true)
    current_status=$(sqlite3 "$DB" "SELECT status FROM items WHERE ota_id='$ota_id';" 2>/dev/null || true)

    if [ -z "$item_type" ]; then
        log_msg "SKIP   $ota_id — not found in DB"
        return 0
    fi

    # Skip already-closed items
    case "$current_status" in
        "Fixed (→ Fixed.md)"|"Implemented (→ Fixed.md)"|"Completed (→ Fixed.md)"|"Obsolete (→ Fixed.md)")
            log_msg "SKIP   $ota_id — already closed ($current_status)"
            return 0
            ;;
    esac

    local new_status
    new_status=$(infer_transition "$commit_msg" "$item_type")

    if [ "$new_status" == "$current_status" ]; then
        log_msg "SKIP   $ota_id — status unchanged ($current_status)"
        return 0
    fi

    if $DRY_RUN; then
        log_msg "DRYRUN $ota_id — $item_type: $current_status → $new_status (commit $source_commit)"
        return 0
    fi

    sqlite3 "$DB" <<SQL
BEGIN TRANSACTION;
UPDATE items SET status = '$new_status', modified_at = datetime('now') WHERE ota_id = '$ota_id';
INSERT INTO item_history (ota_id, by, event, on_date, reason, evidence_path, outcome)
VALUES ('$ota_id', 'AI', '$new_status', datetime('now'),
        'Auto-synced from commit $source_commit: ${commit_msg:0:120}',
        '',
        '$new_status');
COMMIT;
SQL

    if [ $? -eq 0 ]; then
        log_msg "SYNCED $ota_id — $item_type: $current_status → $new_status ($source_commit)"
        echo "$ota_id"
    else
        log_msg "FAIL   $ota_id — DB update failed"
    fi
}

# --- main ---------------------------------------------------------------------
mkdir -p "$SYNC_LOG_DIR"
: > "$SYNC_LOG"

log_msg "=== Sync run $RUN_ID ==="

if $AUTO_MODE; then
    log_msg "Auto mode — processing ${#AUTO_IDS[@]} named OTA-IDs"
    for ota_id in "${AUTO_IDS[@]}"; do
        process_ota_id "$ota_id" "auto" "auto-triggered sync"
    done
    log_msg "=== Auto sync complete ==="
    exit 0
fi

db_exists

SYNCED_COUNT=0
while IFS=' ' read -r commit_hash commit_msg_rest; do
    [ -z "$commit_hash" ] && continue
    commit_msg="$commit_msg_rest"

    # Extract OTA-IDs (OTA-NNN with 3 digits)
    ota_ids=$(echo "$commit_msg" | grep -oP 'OTA-\d{3}' 2>/dev/null || true)
    [ -z "$ota_ids" ] && continue

    while IFS= read -r ota_id; do
        [ -z "$ota_id" ] && continue
        result=$(process_ota_id "$ota_id" "${commit_hash:0:9}" "$commit_msg")
        if [ -n "$result" ]; then
            SYNCED_COUNT=$((SYNCED_COUNT + 1))
        fi
    done <<< "$ota_ids"
done < <(resolve_commits)

# --- place sync marker --------------------------------------------------------
if [ "$SYNCED_COUNT" -gt 0 ] && ! $DRY_RUN; then
    # Move/place marker tag
    git -C "$PROJECT_ROOT" tag -f db-sync-marker HEAD 2>/dev/null || true
    log_msg "Placed db-sync-marker tag at HEAD"
fi

log_msg "=== Sync complete — $SYNCED_COUNT item(s) updated ==="
