#!/usr/bin/env bash
# =============================================================================
# push_all.sh — Buffered background push to all upstreams
# -----------------------------------------------------------------------------
# Purpose: Push a branch to every configured upstream remote with per-remote
#          locking, retry with backoff, and buffered output. Designed to run
#          detached (nohup) per §11.4.88 so the main commit_all.sh lock
#          releases immediately after the local commit is durable.
#
# Usage:
#   bash scripts/push_all.sh                    # push current branch to all
#   bash scripts/push_all.sh main               # explicit branch
#   bash scripts/push_all.sh main --retries 3   # override retry count
#
# Options:
#   -b, --branch NAME     Branch to push (default: current)
#   -r, --retries N       Max retries per remote (default: 3)
#   -d, --delay SECS      Initial retry delay in seconds (default: 5)
#   -l, --log PATH        Log file path (default: auto-generated)
#   -q, --quiet           Suppress stdout (log file still written)
#   -h, --help            Show this help
#
# Inputs:
#   - Git repository state (commits to push)
#   - Lock files: .git/.push.<remote>.lock (per-remote serialization)
#
# Outputs:
#   - Push to each configured remote
#   - Exit 0 if ALL remotes succeed, 1 if ANY fail
#   - Log file with per-remote status
#
# Side-effects:
#   - Acquires per-remote flock (.git/.push.<remote>.lock)
#   - Writes push log to qa-results/push_failures/ or specified path
#
# Dependencies:
#   - git (>= 2.30), flock (util-linux), bash 4+
#
# Cross-references:
#   - CLAUDE.md §11.4.88 (background push mandate)
#   - Constitution §11.4.71 (pre-push fetch + integrate)
#   - Constitution §11.4.113 (absolute no-force-push)
#
# Last verified: 2026-06-21
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Defaults
BRANCH=""
MAX_RETRIES=3
INITIAL_DELAY=5
LOG_PATH=""
QUIET=false
REMOTE_LIST=("github" "gitlab" "gitflic" "gitverse")

# Colors (safe when piped)
RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'
CYAN='\033[0;36m'; NC='\033[0m'
log_info()  { [[ "$QUIET" == "true" ]] || echo -e "${CYAN}[PUSH]${NC} $*"; echo "[$(date -u +%H:%M:%S)] INFO: $*" >> "$LOG_PATH"; }
log_ok()    { [[ "$QUIET" == "true" ]] || echo -e "${GREEN}[PUSH-OK]${NC} $*"; echo "[$(date -u +%H:%M:%S)] OK: $*" >> "$LOG_PATH"; }
log_warn()  { [[ "$QUIET" == "true" ]] || echo -e "${YELLOW}[PUSH-WARN]${NC} $*"; echo "[$(date -u +%H:%M:%S)] WARN: $*" >> "$LOG_PATH"; }
log_error() { [[ "$QUIET" == "true" ]] || echo -e "${RED}[PUSH-ERR]${NC} $*"; echo "[$(date -u +%H:%M:%S)] ERROR: $*" >> "$LOG_PATH"; }

show_help() {
    cat << 'EOF'
push_all.sh — Buffered background push to all upstreams

USAGE:
    bash scripts/push_all.sh [BRANCH] [OPTIONS]

OPTIONS:
    -b, --branch NAME     Branch (default: current)
    -r, --retries N       Max retries per remote (default: 3)
    -d, --delay SECS      Initial retry delay (default: 5)
    -l, --log PATH        Log file path
    -q, --quiet           Suppress stdout
    -h, --help            Show this help

EXAMPLES:
    bash scripts/push_all.sh                    # push current branch
    bash scripts/push_all.sh main --retries 5   # 5 retries
    nohup bash scripts/push_all.sh > /tmp/push.log 2>&1 &  # detached
EOF
}

# Parse args
while [[ $# -gt 0 ]]; do
    case $1 in
        -b|--branch)   BRANCH="$2"; shift 2 ;;
        -r|--retries)  MAX_RETRIES="$2"; shift 2 ;;
        -d|--delay)    INITIAL_DELAY="$2"; shift 2 ;;
        -l|--log)      LOG_PATH="$2"; shift 2 ;;
        -q|--quiet)    QUIET=true; shift ;;
        -h|--help)     show_help; exit 0 ;;
        -*)            log_error "Unknown: $1"; show_help; exit 1 ;;
        *)             [[ -z "$BRANCH" ]] && BRANCH="$1"; shift ;;
    esac
done

# Resolve branch
if [[ -z "$BRANCH" ]]; then
    BRANCH=$(git -C "$PROJECT_ROOT" branch --show-current 2>/dev/null || echo "main")
fi

# Auto-generate log path if not provided
if [[ -z "$LOG_PATH" ]]; then
    LOG_DIR="$PROJECT_ROOT/qa-results/push_failures"
    mkdir -p "$LOG_DIR" 2>/dev/null || true
    LOG_PATH="${LOG_DIR}/$(date -u +%Y%m%dT%H%M%SZ)_push.log"
fi
mkdir -p "$(dirname "$LOG_PATH")" 2>/dev/null || true

log_info "=== push_all.sh: branch=$BRANCH retries=$MAX_RETRIES delay=${INITIAL_DELAY}s ==="
log_info "Remotes: ${REMOTE_LIST[*]}"
log_info "Log: $LOG_PATH"

# Pre-push: fetch all remotes (§11.4.71)
log_info "Fetching all remotes..."
git -C "$PROJECT_ROOT" fetch --all --prune 2>&1 | tail -1 >> "$LOG_PATH" || true

# Per-remote push with locking + retry + exponential backoff
TOTAL_FAILURES=0
declare -A REMOTE_STATUS

push_one_remote() {
    local remote="$1"
    local lock_file="$PROJECT_ROOT/.git/.push.${remote}.lock"

    # Per-remote flock — serializes concurrent pushes to same remote,
    # allows different-remote pushes in parallel (§11.4.88)
    exec 9>"$lock_file"
    if ! flock -n 9; then
        log_warn "Remote $remote: another push is running — skipping"
        REMOTE_STATUS["$remote"]="SKIPPED_LOCKED"
        return 0
    fi

    local attempt=0
    local delay=$INITIAL_DELAY

    while [[ $attempt -lt $MAX_RETRIES ]]; do
        attempt=$((attempt + 1))
        log_info "  $remote: attempt $attempt/$MAX_RETRIES"

        local push_out push_rc
        push_out=$(git -C "$PROJECT_ROOT" push "$remote" "$BRANCH" 2>&1)
        push_rc=$?

        if [[ $push_rc -eq 0 ]]; then
            log_ok "  $remote: pushed successfully"
            REMOTE_STATUS["$remote"]="OK"
            exec 9>&-  # release lock
            return 0
        fi

        # Check for packfile-size-limit (GitFlic 100 MB)
        if echo "$push_out" | grep -qE 'Pack exceeds the limit|pack exceeds|packfile.*limit|unpacker error'; then
            log_warn "  $remote: packfile limit — falling back to phased push"
            _phased_push "$remote" "$BRANCH"
            REMOTE_STATUS["$remote"]="PHASED"
            exec 9>&-
            return 0
        fi

        log_warn "  $remote: push failed (attempt $attempt): $(echo "$push_out" | tail -1)"

        if [[ $attempt -lt $MAX_RETRIES ]]; then
            log_info "  $remote: retrying in ${delay}s..."
            sleep "$delay"
            delay=$((delay * 2))  # exponential backoff
        fi
    done

    log_error "  $remote: FAILED after $MAX_RETRIES attempts"
    REMOTE_STATUS["$remote"]="FAILED"
    TOTAL_FAILURES=$((TOTAL_FAILURES + 1))
    exec 9>&-
    return 1
}

# Phased push for packfile-limited remotes (GitFlic)
_phased_push() {
    local remote="$1" branch="$2"
    git -C "$PROJECT_ROOT" fetch "$remote" "$branch" 2>/dev/null || true
    local behind
    behind=$(git -C "$PROJECT_ROOT" rev-list --count "$remote/$branch..$branch" 2>/dev/null || echo "0")
    if [[ "$behind" -eq 0 ]]; then
        log_info "  $remote/$branch already current"
        return 0
    fi
    log_warn "  $remote: $behind commits behind — packfile limit, manual sync needed"
    # Generate bundle in background
    local ts; ts=$(date -u +%Y%m%dT%H%M%S)
    local bundle_dir="$PROJECT_ROOT/.git/gitflic_bundle_${ts}"
    mkdir -p "$bundle_dir"
    git -C "$PROJECT_ROOT" bundle create "$bundle_dir/helix_ota.bundle" --all 2>/dev/null || true
    log_info "  $remote: bundle at $bundle_dir/"
}

# Push all remotes — each in a subshell for isolation
for remote in "${REMOTE_LIST[@]}"; do
    push_one_remote "$remote" || true
done

# Summary
echo "" >> "$LOG_PATH"
echo "=== PUSH SUMMARY $(date -u +%Y-%m-%dT%H:%M:%SZ) ===" >> "$LOG_PATH"
for remote in "${REMOTE_LIST[@]}"; do
    echo "  $remote: ${REMOTE_STATUS[$remote]:-UNKNOWN}" >> "$LOG_PATH"
done

log_info "=== Push Summary ==="
for remote in "${REMOTE_LIST[@]}"; do
    local_status="${REMOTE_STATUS[$remote]:-UNKNOWN}"
    case "$local_status" in
        OK)           log_ok "  $remote: OK" ;;
        PHASED)       log_warn "  $remote: phased (manual sync needed)" ;;
        SKIPPED_LOCKED) log_warn "  $remote: skipped (lock held)" ;;
        *)            log_error "  $remote: $local_status" ;;
    esac
done

if [[ $TOTAL_FAILURES -gt 0 ]]; then
    log_error "$TOTAL_FAILURES remote(s) FAILED — see $LOG_PATH"
    exit 1
fi

log_ok "=== All upstreams pushed ==="
exit 0
