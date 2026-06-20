#!/usr/bin/env bash
# =============================================================================
# HelixTrack Launcher — Common Functions
# -----------------------------------------------------------------------------
# Purpose: Shared boot/infra logic for HelixTrack launcher scripts (web/desktop).
# Usage: source "$(dirname "$0")/common.sh"
# =============================================================================
set -euo pipefail

HELIX_OTA_ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
HELIX_TRACK_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
HELIX_TRACK_CODE="${HELIX_TRACK_ROOT}/helix_track_code"

# Resolve the HelixTrack codebase — sibling Projects/helix_track/
if [ -d "${HELIX_OTA_ROOT}/../helix_track/.git" ]; then
    HELIX_TRACK_CODE="$(cd "${HELIX_OTA_ROOT}/../helix_track" && pwd)"
fi

SPACE_ROOT="${HELIX_TRACK_ROOT}/spaces/_default"
SPACE_CONFIG="${SPACE_ROOT}/config.json"
SPACE_DB="${SPACE_ROOT}/data/helixtrack.db"

# Colors for progress output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

log_info()  { echo -e "${BLUE}[INFO]${NC} $*"; }
log_ok()    { echo -e "${GREEN}[OK]${NC} $*"; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC} $*"; }
log_error() { echo -e "${RED}[ERROR]${NC} $*"; }
log_step()  { echo -e "${CYAN}[STEP]${NC} $*"; }

# ---------------------------------------------------------------------------
# check_dependencies — verify all required tooling is available
# ---------------------------------------------------------------------------
check_dependencies() {
    local missing=0
    for cmd in curl go docker; do
        if ! command -v "$cmd" &>/dev/null; then
            log_warn "Missing dependency: $cmd"
            missing=$((missing + 1))
        fi
    done
    if [ $missing -gt 0 ]; then
        log_error "$missing required dependencies missing. Install them first."
        return 1
    fi
    log_ok "All core dependencies available"
}

# ---------------------------------------------------------------------------
# ensure_space — create or verify the space directory structure
# ---------------------------------------------------------------------------
ensure_space() {
    local space_dir="${1:-$SPACE_ROOT}"
    log_step "Ensuring space directory: ${space_dir}"

    mkdir -p "${space_dir}/data/assets"
    mkdir -p "${space_dir}/data"

    # Create default config if absent
    if [ ! -f "${space_dir}/config.json" ]; then
        log_info "Creating default space config..."
        cat > "${space_dir}/config.json" <<CONFIGEOF
{
  "schema_version": 1,
  "space_id": "$(basename "${space_dir}")",
  "title": "HelixTrack Space — $(basename "${space_dir}")",
  "description": "Auto-created space for HelixTrack",
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "core_endpoint": "http://localhost:8080",
  "web_client_url": "http://localhost:4200",
  "database": {
    "path": "data/helixtrack.db",
    "type": "sqlite"
  },
  "assets_path": "data/assets",
  "onboarding_complete": false
}
CONFIGEOF
        log_ok "Default config created"
    fi

    log_ok "Space directory ready: ${space_dir}"
}

# ---------------------------------------------------------------------------
# boot_core — start HelixTrack Core if not running
# ---------------------------------------------------------------------------
boot_core() {
    local space_dir="${1:-$SPACE_ROOT}"
    local core_binary="${HELIX_TRACK_CODE}/core/Application/htCore"

    log_step "HelixTrack Core boot"
    log_info "Space: ${space_dir}"

    # Check if Core is already running
    if curl -s "http://localhost:8080" &>/dev/null; then
        log_ok "HelixTrack Core already running on :8080"
        return 0
    fi

    # Build Core if binary missing
    if [ ! -f "$core_binary" ]; then
        log_info "Building HelixTrack Core..."
        (cd "${HELIX_TRACK_CODE}/core/Application" && go build -o htCore main.go)
        log_ok "Core built"
    fi

    # Start Core
    log_info "Starting HelixTrack Core..."
    nohup "$core_binary" --space-root="${space_dir}" \
        > "${space_dir}/data/core.log" 2>&1 &
    local pid=$!
    log_info "Core PID: ${pid}"

    # Wait for readiness
    for i in $(seq 1 30); do
        if curl -s "http://localhost:8080" &>/dev/null; then
            log_ok "HelixTrack Core is ready on :8080 (${i}s)"
            return 0
        fi
        sleep 1
    done

    log_error "HelixTrack Core did not start within 30s — check ${space_dir}/data/core.log"
    return 1
}

# ---------------------------------------------------------------------------
# boot_infra — start all required infrastructure
# ---------------------------------------------------------------------------
boot_infra() {
    log_step "Infrastructure boot"
    local space_dir="${1:-$SPACE_ROOT}"

    check_dependencies
    ensure_space "$space_dir"
    boot_core "$space_dir"
    log_ok "Infrastructure ready"
}

# ---------------------------------------------------------------------------
# cleanup — stop HelixTrack Core
# ---------------------------------------------------------------------------
cleanup() {
    log_step "Cleaning up"
    if [ -f "${SPACE_ROOT}/data/core.pid" ]; then
        kill "$(cat "${SPACE_ROOT}/data/core.pid")" 2>/dev/null || true
        rm -f "${SPACE_ROOT}/data/core.pid"
        log_ok "HelixTrack Core stopped"
    fi
}

trap cleanup EXIT
