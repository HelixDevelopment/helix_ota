#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/deploy_api.sh — build + deploy the Go control-plane API.
# -----------------------------------------------------------------------------
# Purpose:
#   Cross-compile the static `ota-server` binary on the build host (where the
#   §11.4.28 sibling `replace` directives resolve — server/Dockerfile header),
#   stage it at server/.docker-bin/ota-server, rsync the server/ build context to
#   the remote, build the image there, and bring up the ota-server service
#   (Postgres-backed). The API is reverse-proxied at hxota.dev/api/v1 by the proxy.
#
# Usage:
#   bash scripts/remote_deploy/deploy_api.sh [--dry-run] [--product <id>]
#
# Inputs (env): lib/common.sh; deploy-env. GOOS/GOARCH override via
#   HXOTA_API_GOARCH (default arm64 — matches an aarch64 Linux server; set amd64
#   for an x86_64 host).
# Outputs: build/rsync/compose output OR the dry-run plan.
# Side-effects (real run): writes server/.docker-bin/ota-server (gitignored),
#   rsyncs the server/ context, builds + starts the ota-server container.
# Dependencies: go (host, for cross-compile); ssh, rsync; rootless podman (remote).
# Cross-references: §11.4.28 (sibling replaces) · §11.4.10 · §11.4.161 · §11.4.6 · §11.4.18.
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
[ "${HX_WANT_HELP:-0}" = "1" ] && { sed -n '2,30p' "$0"; exit 0; }

hx_load_deploy_env || exit $?
_goarch="${HXOTA_API_GOARCH:-arm64}"
_server="$HXOTA_PROJECT_ROOT/server"

hxlog "=== deploy_api (product=$HXOTA_PRODUCT, GOARCH=$_goarch) ==="

# --- 1. cross-compile the static binary (host) -------------------------------
if hx_is_dry_run; then
    hxlog "DRY-RUN would cross-compile:"
    printf '  ( cd %s && CGO_ENABLED=0 GOOS=linux GOARCH=%s go build -o .docker-bin/ota-server ./cmd/ota-server )\n' "$_server" "$_goarch" >&2
else
    command -v go >/dev/null 2>&1 || hxdie "go toolchain not found on the build host — required to cross-compile ota-server (§11.4.28 sibling replaces resolve here)."
    mkdir -p "$_server/.docker-bin"
    hxlog "Cross-compiling ota-server (CGO_ENABLED=0 GOOS=linux GOARCH=$_goarch) ..."
    ( cd "$_server" && CGO_ENABLED=0 GOOS=linux GOARCH="$_goarch" go build -o .docker-bin/ota-server ./cmd/ota-server )
    [ -f "$_server/.docker-bin/ota-server" ] || hxdie "cross-compile did not produce .docker-bin/ota-server."
fi

# --- 2. push the build context (incl .docker-bin) ----------------------------
hx_ssh_run "mkdir -p '$HXOTA_REMOTE_DIR/server'"
hx_rsync_up "$_server/" "$HXOTA_REMOTE_DIR/server/" "--delete --exclude=.git --exclude=node_modules"

# --- 3. build + up the ota-server service on the remote ----------------------
hxlog "Building + starting ota-server on the remote ..."
hx_compose_remote "build" "ota-server"
hx_compose_remote "up -d" "ota-server"
hxlog "deploy_api complete."
