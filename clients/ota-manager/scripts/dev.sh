#!/usr/bin/env bash
# clients/ota-manager/scripts/dev.sh
#
# Purpose: Start both the Vite dev server and the ota-server for local
#          development, with both processes running in the foreground.
#
# Usage:   bash scripts/dev.sh
#
# Inputs:  None
#
# Outputs: Console output from both the Vite dev server and the Go server.
#
# Side-effects:
#   - Starts the Vite dev server (port 5173 by default)
#   - Starts the ota-server (port 8080 by default)
#   - Both processes run in the background; use Ctrl+C to stop both
#
# Dependencies:
#   - pnpm (at project root)
#   - Go toolchain (at ../../server)
#   - ota-manager dependencies installed (pnpm install)
#
# Cross-references:
#   - Dockerfile (clients/ota-manager/Dockerfile)
#   - docker/ota-manager.docker-compose.yml
#   - clients/ota-manager/server-integration.md
#
# Last verified: 2026-06-19

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
CLIENT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$CLIENT_DIR/../.." && pwd)"
SERVER_DIR="$REPO_ROOT/server"

echo "=== OTA Manager — Development Environment ==="
echo "  Client:     ${CLIENT_DIR}"
echo "  Server:     ${SERVER_DIR}"
echo "  Vite:       http://localhost:5173"
echo "  API:        http://localhost:8080/api/v1"
echo ""

# Verify prerequisites.
if ! command -v pnpm &>/dev/null; then
    echo "ERROR: pnpm is not installed. Run: corepack enable && pnpm --version"
    exit 1
fi

if ! command -v go &>/dev/null; then
    echo "ERROR: Go is not installed."
    exit 1
fi

# Trap Ctrl+C to clean up both background processes.
cleanup() {
    echo ""
    echo "Shutting down..."
    kill "$VITE_PID" 2>/dev/null || true
    kill "$GO_PID" 2>/dev/null || true
    wait "$VITE_PID" 2>/dev/null || true
    wait "$GO_PID" 2>/dev/null || true
    echo "Done."
    exit 0
}
trap cleanup SIGINT SIGTERM

# Start the ota-server (Go) in the background.
echo "Starting ota-server (Go)..."
cd "$SERVER_DIR"
HELIX_ADMIN_PASSWORD="${HELIX_ADMIN_PASSWORD:-admin}" \
    go run ./cmd/ota-server &
GO_PID=$!

# Start the Vite dev server in the background.
echo "Starting Vite dev server..."
cd "$CLIENT_DIR"
pnpm dev &
VITE_PID=$!

echo ""
echo "Both servers are starting.  Press Ctrl+C to stop both."
echo "  Vite: http://localhost:5173  (proxies /api -> :8080)"
echo "  API:  http://localhost:8080/api/v1"
echo ""

# Wait for either process to exit.
wait -n "$VITE_PID" "$GO_PID" 2>/dev/null || true
cleanup
