#!/usr/bin/env bash
# =============================================================================
# HelixCode → HelixTrack Desktop Launcher
# -----------------------------------------------------------------------------
# Purpose: Boot HelixTrack infrastructure and open Desktop client.
# Usage: ./helix_code/scripts/launchers/desktop.sh [--space-dir=<path>]
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
HELIX_OTA_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Locate HelixTrack desktop launcher
HELIXTRACK_DESKTOP="${HELIX_OTA_ROOT}/helix_track/scripts/launchers/desktop.sh"
if [ -x "$HELIXTRACK_DESKTOP" ]; then
    exec "$HELIXTRACK_DESKTOP" "$@"
fi

# Fallback
FALLBACK="${HELIX_OTA_ROOT}/../helix_track/helix_track/scripts/launchers/desktop.sh"
if [ -x "$FALLBACK" ]; then
    exec "$FALLBACK" "$@"
fi

echo "ERROR: HelixTrack desktop launcher not found."
exit 1
