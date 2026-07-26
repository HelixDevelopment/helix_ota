#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/deploy_infrastructure.sh — deploy the stateful infra.
# -----------------------------------------------------------------------------
# Purpose:
#   Provision the remote infrastructure tier: prepare the remote bundle dir, push
#   the compose + nginx config, write the runtime stack.env (§11.4.10 — never
#   echoed, runtime secrets only), and bring up PostgreSQL + MinIO (the persistence
#   + artifact-blob services). The reverse-proxy + web tier come up in deploy.sh
#   AFTER the API + dashboards + website are staged.
#
# Usage:
#   bash scripts/remote_deploy/deploy_infrastructure.sh [--dry-run] [--product <id>]
#
# Inputs (env): lib/common.sh; deploy-env (connection + runtime secrets).
# Outputs: prep/rsync/compose output OR the dry-run plan.
# Side-effects (real run): creates the remote bundle dir, rsyncs compose+nginx+
#   stack.env, starts postgres+minio containers (rootless). Volumes persist.
# Dependencies: ssh, rsync (host); rootless podman + compose (remote).
# Cross-references: §11.4.10 · §11.4.28 · §11.4.161 · §11.4.6 · §11.4.18.
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
[ "${HX_WANT_HELP:-0}" = "1" ] && { sed -n '2,30p' "$0"; exit 0; }

hx_load_deploy_env || exit $?
hx_load_product_env

hxlog "=== deploy_infrastructure (product=$HXOTA_PRODUCT) ==="
hx_remote_prepare
hx_write_remote_stack_env || exit $?

hxlog "Probing remote rootless compose front-end ..."
_rc="$(hx_remote_compose_cmd)"
case "$_rc" in
    DRY_RUN)  : ;;   # dry-run already printed the probe
    NO_PODMAN)  hxdie "remote host has NO rootless podman (§11.4.161) — install it before deploy." ;;
    NO_COMPOSE) hxdie "remote podman present but no compose front-end — install podman-compose." ;;
    SSH_FAIL)   hxdie "SSH to the remote host FAILED (check the deploy env / reachability)." ;;
    *)          hxlog "remote compose front-end: $_rc" ;;
esac

hxlog "Bringing up infrastructure services (postgres, minio) ..."
hx_compose_remote "up -d" "postgres"
hx_compose_remote "up -d" "minio"
hxlog "deploy_infrastructure complete."
