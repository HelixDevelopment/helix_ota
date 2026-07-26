#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/deploy.sh — MAIN remote deployment orchestrator.
# -----------------------------------------------------------------------------
# Purpose:
#   One-shot deploy of the ENTIRE HelixOTA stack + website to the remote host:
#     preflight -> infrastructure -> API -> dashboards -> website -> HTTPS certs
#     -> bring up the proxy -> LIVE confirm/validate/verify (health probes) -> report.
#   Deploys ALL services and the website. Delegates each tier to its component
#   script so logic lives once. SSH as $HXOTA_DEPLOY_USER@$ADDRESS_HXOTA:$SSH_PORT_HXOTA
#   (from the gitignored deploy env, §11.4.10 — never echoed), rootless podman
#   compose on the remote (§11.4.161).
#
# Usage:
#   bash scripts/remote_deploy/deploy.sh [--dry-run] [--product <id>] [--skip-certs]
#                                        [--skip-build]
#
#   --dry-run     Print every action that WOULD run; do NOT connect (anti-bluff).
#   --product     Product family / target (default svord).
#   --skip-certs  Skip the HTTPS cert step (e.g. certs already valid).
#   --skip-build  Reuse existing SPA/API builds (skip the build steps).
#
# Inputs (env): lib/common.sh; the deploy env (connection + runtime secrets).
# Outputs: end-to-end deploy output + a final HEALTHY/UNHEALTHY verdict.
# Side-effects (real run): full remote deploy — builds, rsyncs, starts containers,
#   issues certs. Dry-run: none (prints only).
# Dependencies: go/node/pnpm/npm (host builds); ssh, rsync; rootless podman (remote).
# Cross-references: §11.4.10 · §11.4.13 (live sink-side confirm) · §11.4.28 ·
#   §11.4.108 (runtime signature = live health) · §11.4.161 · §11.4.6 · §11.4.18.
#   Companion doc: docs/remote_deploy/REMOTE_DEPLOY.md (incl. QD1-QD6 operator decisions).
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
[ "${HX_WANT_HELP:-0}" = "1" ] && { sed -n '2,40p' "$0"; exit 0; }
SKIP_CERTS=0; case " $HX_REST_ARGS " in *" --skip-certs "*) SKIP_CERTS=1 ;; esac
SKIP_BUILD_ARG=""; case " $HX_REST_ARGS " in *" --skip-build "*) SKIP_BUILD_ARG="--skip-build" ;; esac

# Export so the delegated component scripts inherit the run mode (§11.4.28).
export HXOTA_DRY_RUN HXOTA_PRODUCT HXOTA_REDACT HXOTA_DEPLOY_ENV

hx_load_deploy_env || exit $?
hx_load_product_env

hxlog "================================================================"
hxlog " HelixOTA remote deploy — product=$HXOTA_PRODUCT  dry_run=$HXOTA_DRY_RUN"
hxlog " target: $(hx_ssh_target_display) : $(hx_ssh_port_display)   (redacted)"
hxlog "================================================================"

# --- 0. preflight: host tools + remote reachability --------------------------
hx_require_host_cmd ssh rsync
hxlog "[0/6] preflight — probing remote reachability + rootless compose ..."
_rc="$(hx_remote_compose_cmd)"
case "$_rc" in
    DRY_RUN)    hxlog "  (dry-run: reachability probe printed above)" ;;
    NO_PODMAN)  hxdie "remote has NO rootless podman (§11.4.161) — install before deploy." ;;
    NO_COMPOSE) hxdie "remote podman present but no compose front-end — install podman-compose." ;;
    SSH_FAIL)   hxdie "SSH FAILED — check the deploy env + host reachability (§11.4.6)." ;;
    *)          hxlog "  remote OK — compose front-end: $_rc" ;;
esac

# _run_step LABEL SCRIPT [ARGS...] — run a component script with its own args.
_run_step() {
    _n="$1"; _s="$2"; shift 2
    hxlog "$_n"
    bash "$HXOTA_RD_DIR/$_s" "$@"
}

# --- 1. infrastructure (postgres + minio + stack.env + bundle) ---------------
_run_step "[1/6] infrastructure ..." "deploy_infrastructure.sh"

# --- 2. API (cross-compile + build + up ota-server) --------------------------
_run_step "[2/6] API (ota-server) ..." "deploy_api.sh"

# --- 3. dashboards (console + dashboard SPAs) --------------------------------
# shellcheck disable=SC2086
_run_step "[3/6] dashboards ..." "deploy_dashboards.sh" $SKIP_BUILD_ARG

# --- 4. website --------------------------------------------------------------
# shellcheck disable=SC2086
_run_step "[4/6] website ..." "deploy_website.sh" $SKIP_BUILD_ARG

# --- 5. HTTPS certs (lets_encrypt submodule; SKIP if not incorporated) -------
if [ "$SKIP_CERTS" = "1" ]; then
    hxlog "[5/6] HTTPS certs — skipped (--skip-certs)"
else
    _run_step "[5/6] HTTPS certs (issue) ..." "https_certs.sh" issue
fi

# --- 6. bring up the proxy (whole stack) + LIVE confirm ----------------------
hxlog "[6/6] bringing up the full stack (proxy + all services) ..."
hx_compose_remote "up -d"

hxlog "LIVE confirm / validate / verify (sink-side health, §11.4.13) ..."
hx_remote_health_confirm

hxlog "================================================================"
if hx_is_dry_run; then
    hxlog " DRY-RUN complete — NO host was contacted, NO deploy performed."
    hxlog " Every action above is what WOULD run. Anti-bluff: nothing faked (§11.4.6)."
else
    hxlog " Deploy pipeline finished. Verify the HEALTHY/READY markers above;"
    hxlog " an UNHEALTHY/NOT_READY marker is a real failure to investigate, NOT a pass."
fi
hxlog "================================================================"
