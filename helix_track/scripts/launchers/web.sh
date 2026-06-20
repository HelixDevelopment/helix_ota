#!/usr/bin/env bash
# =============================================================================
# HelixTrack Web Launcher
# -----------------------------------------------------------------------------
# Purpose: Boot HelixTrack infrastructure and open the Web client in default
#          browser. Shows progress during loading (could take a while).
# Usage: ./helix_track/scripts/launchers/web.sh [--space-dir=<path>]
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
echo -e "${CYAN}║          HelixTrack Web Launcher                        ║${NC}"
echo -e "${CYAN}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}Boot sequence may take a moment — please wait...${NC}"
echo ""

boot_infra "$SPACE_DIR"

echo ""
log_step "Opening web client..."
WEB_CLIENT_URL="${SPACE_DIR}/config.json"
WEB_CLIENT_URL=$(python3 -c "import json; print(json.load(open('${WEB_CLIENT_URL}'))['web_client_url'])" 2>/dev/null || echo "http://localhost:4200")

# Try to open the default browser
if command -v open &>/dev/null; then
    open "$WEB_CLIENT_URL"
elif command -v xdg-open &>/dev/null; then
    xdg-open "$WEB_CLIENT_URL"
else
    log_info "Open your browser to: ${WEB_CLIENT_URL}"
fi

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  HelixTrack is running!                                 ║${NC}"
echo -e "${GREEN}║  Web:    ${WEB_CLIENT_URL}                         ${NC}"
echo -e "${GREEN}║  API:   http://localhost:8080                           ║${NC}"
echo -e "${GREEN}║  Space: ${SPACE_DIR}                              ${NC}"
echo -e "${GREEN}╚══════════════════════════════════════════════════════════╝${NC}"
echo ""

# Keep running until user Ctrl+C
log_info "Press Ctrl+C to stop HelixTrack and clean up."
wait
