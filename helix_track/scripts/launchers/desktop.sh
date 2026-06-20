#!/usr/bin/env bash
# =============================================================================
# HelixTrack Desktop Launcher
# -----------------------------------------------------------------------------
# Purpose: Boot HelixTrack infrastructure and open the Desktop client.
#          Shows progress during loading (could take a while).
# Usage: ./helix_track/scripts/launchers/desktop.sh [--space-dir=<path>]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${SCRIPT_DIR}/common.sh"

# Parse args
SPACE_DIR="${SPACE_ROOT}"
for arg in "$@"; do
    case "$arg" in
        --space-dir=*) SPACE_DIR="${arg#*=}" ;;
        --help|-h)
            echo "Usage: $0 [--space-dir=<path>]"
            echo "  --space-dir   Path to space directory (default: ${SPACE_ROOT})"
            exit 0
            ;;
    esac
done

echo ""
echo -e "${CYAN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}║      HelixTrack Desktop Launcher                        ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}Boot sequence may take a moment — please wait...${NC}"
echo ""

boot_infra "$SPACE_DIR"

echo ""
log_step "Starting Desktop client..."

DESKTOP_DIR="${HELIX_TRACK_CODE}/desktop_client"
if [ -d "$DESKTOP_DIR" ]; then
    if command -v npm &>/dev/null; then
        log_info "Starting Desktop client from ${DESKTOP_DIR}..."
        (cd "$DESKTOP_DIR" && npm start) &
        DESKTOP_PID=$!
        log_ok "Desktop client starting (PID: ${DESKTOP_PID})"
    else
        log_warn "npm not available — cannot start Desktop client automatically"
        log_info "Manual start: cd ${DESKTOP_DIR} && npm start"
    fi
else
    log_warn "Desktop client directory not found at ${DESKTOP_DIR}"
    log_info "Opening web client instead..."
    if command -v open &>/dev/null; then
        open "http://localhost:4200"
    else
        log_info "Open browser to http://localhost:4200"
    fi
fi

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  HelixTrack is running!                                 ║${NC}"
echo -e "${GREEN}║  API:   http://localhost:8080                           ║${NC}"
echo -e "${GREEN}║  Space: ${SPACE_DIR}                              ${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""

log_info "Press Ctrl+C to stop HelixTrack and clean up."
wait
