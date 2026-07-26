#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/deploy_dashboards.sh — build + deploy the console/dashboards.
# -----------------------------------------------------------------------------
# Purpose:
#   Build the operator console (ota-manager SPA) + the secondary dashboard SPA,
#   stage their production dists into the remote bundle's ./srv/{console,dashboard}
#   (the proxy serves them at hxota.dev/ and hxota.dev/dashboard/), then reload the
#   proxy so the new assets are served. Static assets are bind-mounted read-only,
#   so a content refresh needs no image rebuild.
#
# Usage:
#   bash scripts/remote_deploy/deploy_dashboards.sh [--dry-run] [--product <id>] [--skip-build]
#
# Inputs (env): lib/common.sh; deploy-env. Build dirs/commands overridable via
#   HXOTA_CONSOLE_DIR (default clients/ota-manager), HXOTA_DASHBOARD_DIR (default
#   dashboard), HXOTA_CONSOLE_BUILD (default "pnpm install --frozen-lockfile && pnpm build"),
#   HXOTA_DASHBOARD_BUILD (default "npm ci && npm run build").
# Outputs: build/rsync/reload output OR the dry-run plan.
# Side-effects (real run): runs the SPA builds, rsyncs dist/ to the remote srv,
#   reloads the proxy container.
# Dependencies: node/pnpm/npm (host, for build); ssh, rsync; rootless podman (remote).
# Cross-references: §11.4.28 · §11.4.161 · §11.4.6 · §11.4.18 · §11.4.190 (web quality — proof is a separate on-host render/audit item).
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
[ "${HX_WANT_HELP:-0}" = "1" ] && { sed -n '2,32p' "$0"; exit 0; }
SKIP_BUILD=0; case " $HX_REST_ARGS " in *" --skip-build "*) SKIP_BUILD=1 ;; esac

hx_load_deploy_env || exit $?
_cdir="$HXOTA_PROJECT_ROOT/${HXOTA_CONSOLE_DIR:-clients/ota-manager}"
_ddir="$HXOTA_PROJECT_ROOT/${HXOTA_DASHBOARD_DIR:-dashboard}"
_cbuild="${HXOTA_CONSOLE_BUILD:-pnpm install --frozen-lockfile && pnpm build}"
_dbuild="${HXOTA_DASHBOARD_BUILD:-npm ci && npm run build}"

hxlog "=== deploy_dashboards (product=$HXOTA_PRODUCT) ==="

_build_one() {
    _dir="$1"; _cmd="$2"; _name="$3"
    if [ "$SKIP_BUILD" = "1" ]; then hxlog "--skip-build: reusing existing $_name dist"; return 0; fi
    if hx_is_dry_run; then hxlog "DRY-RUN would build $_name:"; printf '  ( cd %s && %s )\n' "$_dir" "$_cmd" >&2; return 0; fi
    [ -d "$_dir" ] || { hxwarn "$_name source dir absent ($_dir) — SKIP build (§11.4.6)"; return 0; }
    hxlog "Building $_name ..."
    ( cd "$_dir" && sh -c "$_cmd" )
}

_build_one "$_cdir" "$_cbuild" "console (ota-manager)"
_build_one "$_ddir" "$_dbuild" "dashboard"

hxlog "Staging dists into the remote bundle ./srv ..."
hx_ssh_run "mkdir -p '$HXOTA_REMOTE_DIR/srv/console' '$HXOTA_REMOTE_DIR/srv/dashboard'"
hx_rsync_up "$_cdir/dist/" "$HXOTA_REMOTE_DIR/srv/console/"   "--delete"
hx_rsync_up "$_ddir/dist/" "$HXOTA_REMOTE_DIR/srv/dashboard/" "--delete"

hxlog "Reloading proxy to serve refreshed dashboards ..."
hx_compose_remote "restart" "proxy"
hxlog "deploy_dashboards complete."
