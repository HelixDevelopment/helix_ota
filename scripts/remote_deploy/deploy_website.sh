#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/deploy_website.sh — build + deploy the marketing website.
# -----------------------------------------------------------------------------
# Purpose:
#   Build the website (Angular SPA, submodules/website) and stage its production
#   dist into the remote bundle's ./srv/website (the proxy serves it on hxota.com;
#   hxota.dev can 302-redirect to it per QD5). Static assets are bind-mounted
#   read-only — a content refresh needs no image rebuild, only a proxy reload.
#
# Usage:
#   bash scripts/remote_deploy/deploy_website.sh [--dry-run] [--product <id>] [--skip-build]
#
# Inputs (env): lib/common.sh; deploy-env. Overridable via HXOTA_WEBSITE_DIR
#   (default submodules/website), HXOTA_WEBSITE_DIST (default
#   dist/helix_ota_website/browser), HXOTA_WEBSITE_BUILD (default "npm ci && npm run build").
# Outputs: build/rsync/reload output OR the dry-run plan.
# Side-effects (real run): runs the website build, rsyncs dist to the remote srv,
#   reloads the proxy container.
# Dependencies: node/npm (host, for build); ssh, rsync; rootless podman (remote).
# Cross-references: §11.4.28 · §11.4.161 · §11.4.6 · §11.4.18 · §11.4.190 (web quality — proof is a separate on-host render/SEO-audit item).
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
[ "${HX_WANT_HELP:-0}" = "1" ] && { sed -n '2,32p' "$0"; exit 0; }
SKIP_BUILD=0; case " $HX_REST_ARGS " in *" --skip-build "*) SKIP_BUILD=1 ;; esac

hx_load_deploy_env || exit $?
_wdir="$HXOTA_PROJECT_ROOT/${HXOTA_WEBSITE_DIR:-submodules/website}"
_wdist="${HXOTA_WEBSITE_DIST:-dist/helix_ota_website/browser}"
_wbuild="${HXOTA_WEBSITE_BUILD:-npm ci && npm run build}"

hxlog "=== deploy_website (product=$HXOTA_PRODUCT) ==="

if [ "$SKIP_BUILD" = "1" ]; then
    hxlog "--skip-build: reusing existing website dist"
elif hx_is_dry_run; then
    hxlog "DRY-RUN would build website:"; printf '  ( cd %s && %s )\n' "$_wdir" "$_wbuild" >&2
elif [ -d "$_wdir" ]; then
    hxlog "Building website ..."; ( cd "$_wdir" && sh -c "$_wbuild" )
else
    hxwarn "website source dir absent ($_wdir) — SKIP build (§11.4.6)"
fi

hxlog "Staging website dist into the remote bundle ./srv/website ..."
hx_ssh_run "mkdir -p '$HXOTA_REMOTE_DIR/srv/website'"
hx_rsync_up "$_wdir/$_wdist/" "$HXOTA_REMOTE_DIR/srv/website/" "--delete"

hxlog "Reloading proxy to serve the refreshed website ..."
hx_compose_remote "restart" "proxy"
hxlog "deploy_website complete."
