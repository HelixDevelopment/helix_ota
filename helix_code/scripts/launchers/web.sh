#!/usr/bin/env bash
# =============================================================================
# HelixCode → HelixTrack Web Launcher
# -----------------------------------------------------------------------------
# Purpose: Open HelixTrack web client in default browser. Boots HelixTrack
#          infrastructure if needed.
# Usage: ./helix_code/scripts/launchers/web.sh [--space-dir=<path>]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HELIX_OTA_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Locate HelixTrack launcher
HELIXTRACK_WEB="${HELIX_OTA_ROOT}/helix_track/scripts/launchers/web.sh"
if [ -x "$HELIXTRACK_WEB" ]; then
    exec "$HELIXTRACK_WEB" "$@"
fi

# Fallback: check sibling Projects/helix_track/
FALLBACK="${HELIX_OTA_ROOT}/../helix_track/helix_track/scripts/launchers/web.sh"
if [ -x "$FALLBACK" ]; then
    exec "$FALLBACK" "$@"
fi

echo "ERROR: HelixTrack launcher not found."
echo "Expected at: ${HELIXTRACK_WEB}"
echo "Or:          ${FALLBACK}"
exit 1
