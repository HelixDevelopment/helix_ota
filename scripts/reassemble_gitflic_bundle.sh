#!/usr/bin/env bash
# =============================================================================
# reassemble_gitflic_bundle.sh — Reassemble GitFlic bundle chunks + push guide
# -----------------------------------------------------------------------------
# Purpose: Given a directory containing chunk_* files produced by
#          commit_all.sh's GitFlic packfile-limit fallback, cat the chunks
#          back into a single .bundle file and print step-by-step operator
#          instructions for cloning and pushing the repo to GitFlic.
#
# Usage:
#   bash scripts/reassemble_gitflic_bundle.sh <bundle-dir>
#
# Example:
#   bash scripts/reassemble_gitflic_bundle.sh /path/to/.git/gitflic_bundle_20260621T124331
#
# Inputs:
#   $1 — Path to directory containing chunk_* files
#
# Outputs:
#   - <bundle-dir>/helix_ota.bundle (reassembled)
#   - Printed clone + push instructions to stdout
#
# Exit codes:
#   0 — bundle reassembled successfully
#   1 — argument missing / directory not found
#   2 — no chunk_* files found in directory
#   3 — bundle creation failed (cat or git clone dry-run)
#
# Dependencies:
#   - cat, split (for chunking; only cat needed for reassembly)
#   - git (for clone validation)
#
# Cross-references:
#   - scripts/commit_all.sh — produces the chunk_* files via split(1)
#   - docs/scripts/reassemble_gitflic_bundle.md — user guide (§11.4.18)
# =============================================================================
set -euo pipefail

# --- Colors ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'

info()  { echo -e "${CYAN}${NC} $*"; }
ok()    { echo -e "${GREEN}${NC} $*"; }
warn()  { echo -e "${YELLOW}${NC} $*"; }
error() { echo -e "${RED}${NC} $*" >&2; }
bold()  { echo -e "${BOLD}${NC} $*"; }

# --- Validate arguments ---
if [ $# -lt 1 ]; then
    error "Usage: bash scripts/reassemble_gitflic_bundle.sh <bundle-dir>"
    exit 1
fi

BUNDLE_DIR="$1"

if [ ! -d "$BUNDLE_DIR" ]; then
    error "Bundle directory not found: ${BUNDLE_DIR}"
    exit 1
fi

# --- Find chunks ---
CHUNK_PATTERN="${BUNDLE_DIR}/chunk_"
# shellcheck disable=SC2086
CHUNK_FILES=$(ls -1 ${CHUNK_PATTERN}* 2>/dev/null || true)

if [ -z "$CHUNK_FILES" ]; then
    error "No chunk_* files found in ${BUNDLE_DIR}"
    exit 2
fi

CHUNK_COUNT=$(echo "$CHUNK_FILES" | wc -l | tr -d ' ')
BUNDLE_FILE="${BUNDLE_DIR}/helix_ota.bundle"

# --- Reassemble ---
info "Reassembling ${CHUNK_COUNT} chunks into ${BUNDLE_FILE}..."
cat "${BUNDLE_DIR}"/chunk_* > "$BUNDLE_FILE"

BUNDLE_SIZE=$(du -h "$BUNDLE_FILE" | awk '{print $1}')
ok "Bundle reassembled: ${BUNDLE_FILE} (${BUNDLE_SIZE})"

# --- Quick integrity check ---
info "Validating bundle integrity..."
if ! git bundle verify "$BUNDLE_FILE" >/dev/null 2>&1; then
    warn "git bundle verify returned errors — bundle may be incomplete or corrupted"
    warn "  Expected chunks: ${CHUNK_COUNT}"
    warn "  Run: ls -la ${BUNDLE_DIR}/chunk_*"
    exit 3
fi
ok "Bundle integrity verified (git bundle verify PASS)"

# --- Count refs ---
REF_COUNT=$(git bundle list-heads "$BUNDLE_FILE" 2>/dev/null | wc -l | tr -d ' ')
ok "Bundle contains ${REF_COUNT} ref(s)"

# --- Print operator instructions ---
bold ""
bold "══════════════════════════════════════════════════════════════════"
bold "  GitFlic Sync Instructions"
bold "══════════════════════════════════════════════════════════════════"
bold ""
bold "  Bundle is ready at:"
bold "    ${BUNDLE_FILE}"
bold "  (${BUNDLE_SIZE}, ${CHUNK_COUNT} original chunks)"
bold ""
bold "  OPTION A — Clone and push from a host WITH SSH access to GitFlic"
bold "  ─────────────────────────────────────────────────────────────────"
echo "    # Copy the bundle to the target host (e.g. nezha):"
echo "    rsync -av ${BUNDLE_DIR}/ user@host:/tmp/gitflic_sync/"
echo ""
echo "    # On the target host:"
echo "    cd /tmp"
echo "    cat chunk_* > helix_ota.bundle"
echo "    git clone helix_ota.bundle helix_ota_gitflic"
echo "    cd helix_ota_gitflic"
echo "    git remote add gitflic git@gitflic.ru:helixdevelopment/helix_ota.git"
echo "    git push gitflic --all"
bold ""
bold "  OPTION B — Manual download + GitFlic web UI import"
bold "  ────────────────────────────────────────────────────"
echo "    1. On your dev machine:"
echo "       cat ${BUNDLE_DIR}/chunk_* > /tmp/helix_ota.bundle"
echo "       git clone /tmp/helix_ota.bundle /tmp/helix_ota_repo"
echo "       cd /tmp/helix_ota_repo"
echo "    2. Create a new empty repository on gitflic.ru"
echo "    3. Set the remote URL:"
echo "       git remote add origin git@gitflic.ru:helixdevelopment/helix_ota.git"
echo "    4. Push:"
echo "       git push origin main"
bold ""
bold "  NOTE: This is a one-time procedure. Subsequent pushes will be"
bold "  small deltas well under the 100 MB packfile limit."
bold "══════════════════════════════════════════════════════════════════"
bold ""

exit 0
