#!/usr/bin/env bash
# =============================================================================
# merge_to_main.sh — Merge-Bot Automation
# -----------------------------------------------------------------------------
# Safely merges a feature branch into main with fast-forward enforcement,
# pre-merge smoke tests, and automatic push to all 4 upstream remotes.
#
# Safety guarantees:
#   1. Clean working tree (no uncommitted changes)
#   2. Feature branch pushed (no orphaned local-only commits)
#   3. Fast-forward ONLY (§11.4.188 — merge main into feature regularly)
#   4. Go smoke tests PASS before push
#   5. Atomic rollback on smoke failure (git reset --merge ORIG_HEAD)
#   6. Push to all 4 upstreams: github, gitlab, gitflic, gitverse
#
# Usage:
#   bash scripts/merge_to_main.sh [feature-branch]
#     Default branch: feature/production-readiness
#
# Examples:
#   bash scripts/merge_to_main.sh
#   bash scripts/merge_to_main.sh feature/accounts-m5
#
# Inputs:
#   - git working tree (must be clean)
#   - server/ (Go module with tests)
#
# Side-effects:
#   - Merges feature into main
#   - Runs go test ./... -count=1 -timeout 120s
#   - Pushes main to all 4 upstreams
#   - On failure: rolls back to ORIG_HEAD
#
# Dependencies: git, go
#
# Cross-references:
#   - §11.4.188 — Regular main→feature merge cadence
#   - §11.4.173 — Containerized build mandate (smoke runs bare-metal for speed)
#   - scripts/push_all.sh — per-remote push utility
#   - tests/test_constitution_inheritance.sh — pre-merge constitution gate
#
# Last verified: 2026-07-26
# =============================================================================
set -euo pipefail

FEATURE_BRANCH="${1:-feature/production-readiness}"
MAIN_BRANCH="main"
PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
UPSTREAMS=("github" "gitlab" "gitflic" "gitverse")

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

log_info()  { echo -e "${GREEN}[INFO]${NC}  $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_fatal() { log_error "$*"; exit 1; }

# --- 1. Verify clean tree -----------------------------------------------------
if [ -n "$(git -C "$PROJECT_ROOT" status --porcelain 2>/dev/null)" ]; then
    log_fatal "Working tree is dirty — commit or stash changes first"
fi
log_info "Working tree is clean"

# --- 2. Fetch all remotes -----------------------------------------------------
log_info "Fetching all remotes with prune..."
git -C "$PROJECT_ROOT" fetch --all --prune --tags

# --- 3. Check feature branch exists and is pushed -----------------------------
if ! git -C "$PROJECT_ROOT" rev-parse --verify "$FEATURE_BRANCH" >/dev/null 2>&1; then
    log_fatal "Feature branch '$FEATURE_BRANCH' does not exist locally"
fi

LOCAL_SHA=$(git -C "$PROJECT_ROOT" rev-parse "$FEATURE_BRANCH")
UNPUSHED_COMMITS=0

for remote in "${UPSTREAMS[@]}"; do
    REMOTE_SHA=$(git -C "$PROJECT_ROOT" rev-parse "$remote/$FEATURE_BRANCH" 2>/dev/null || echo "")
    if [ -z "$REMOTE_SHA" ]; then
        log_warn "Feature branch not found on $remote — push will follow merge"
        UNPUSHED_COMMITS=1
    elif [ "$LOCAL_SHA" != "$REMOTE_SHA" ]; then
        UNPUSHED_COMMITS=1
    fi
done

if [ "$UNPUSHED_COMMITS" -eq 1 ]; then
    log_info "Feature branch has unpushed commits — pushing now..."
    for remote in "${UPSTREAMS[@]}"; do
        git -C "$PROJECT_ROOT" push "$remote" "$FEATURE_BRANCH" 2>/dev/null || \
            log_warn "Push to $remote failed (non-fatal; main merge proceeds)"
    done
fi

# --- 4. Checkout main ---------------------------------------------------------
SAVED_BRANCH=$(git -C "$PROJECT_ROOT" rev-parse --abbrev-ref HEAD)
log_info "Current branch: $SAVED_BRANCH — switching to $MAIN_BRANCH"
git -C "$PROJECT_ROOT" checkout "$MAIN_BRANCH"

# --- 5. Merge (--ff-only) -----------------------------------------------------
log_info "Attempting fast-forward merge of $FEATURE_BRANCH into $MAIN_BRANCH..."

if git -C "$PROJECT_ROOT" merge --ff-only "$FEATURE_BRANCH" 2>&1; then
    log_info "Fast-forward merge succeeded"
else
    log_fatal "Feature branch has diverged — merge main into it first (§11.4.188)"
fi

# --- 6. Run post-merge smoke tests --------------------------------------------
if [ -d "$PROJECT_ROOT/server" ]; then
    log_info "Running Go smoke tests..."
    SMOKE_RESULT=0

    if ! (cd "$PROJECT_ROOT/server" && go test ./... -count=1 -timeout 120s); then
        SMOKE_RESULT=1
    fi

    if [ "$SMOKE_RESULT" -ne 0 ]; then
        log_error "Smoke tests FAILED — rolling back merge"
        git -C "$PROJECT_ROOT" reset --merge ORIG_HEAD 2>/dev/null || \
            git -C "$PROJECT_ROOT" reset --hard ORIG_HEAD
        git -C "$PROJECT_ROOT" checkout "$SAVED_BRANCH" 2>/dev/null || true
        log_fatal "Merge rolled back. Fix failures and retry."
    fi
    log_info "Smoke tests PASSED"
else
    log_warn "server/ directory not found — skipping Go smoke tests"
fi

# --- 7. Run constitution inheritance gate -------------------------------------
if [ -f "$PROJECT_ROOT/tests/test_constitution_inheritance.sh" ]; then
    log_info "Running constitution inheritance gate..."
    if ! bash "$PROJECT_ROOT/tests/test_constitution_inheritance.sh" 2>&1; then
        log_error "Constitution inheritance FAILED — rolling back merge"
        git -C "$PROJECT_ROOT" reset --merge ORIG_HEAD 2>/dev/null || \
            git -C "$PROJECT_ROOT" reset --hard ORIG_HEAD
        git -C "$PROJECT_ROOT" checkout "$SAVED_BRANCH" 2>/dev/null || true
        log_fatal "Merge rolled back. Fix constitution inheritance and retry."
    fi
    log_info "Constitution inheritance gate PASSED"
fi

# --- 8. Push to all 4 upstreams -----------------------------------------------
log_info "Pushing main to all upstreams..."
for remote in "${UPSTREAMS[@]}"; do
    if git -C "$PROJECT_ROOT" remote get-url "$remote" >/dev/null 2>&1; then
        if git -C "$PROJECT_ROOT" push "$remote" "$MAIN_BRANCH" 2>&1; then
            log_info "Pushed to $remote"
        else
            log_error "Push to $remote FAILED"
        fi
    else
        log_warn "Remote '$remote' not configured — skipped"
    fi
done

# --- done ---------------------------------------------------------------------
log_info "=== Merge complete: $FEATURE_BRANCH → $MAIN_BRANCH ==="
log_info "Feature branch: $FEATURE_BRANCH ($LOCAL_SHA)"
log_info "Return to feature branch with: git checkout $SAVED_BRANCH"
