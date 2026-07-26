#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/start.sh — bring the HelixOTA stack UP.
# -----------------------------------------------------------------------------
# Purpose:
#   Start the containerized stack (compose.svord.yml) via rootless podman compose
#   (§11.4.161). Default target is the REMOTE deploy host (over SSH, in
#   $HXOTA_REMOTE_DIR); pass --local to bring up a stack on this host instead.
#
# Usage:
#   bash scripts/remote_deploy/start.sh [--local] [--dry-run] [--product <id>]
#
# Inputs (env): lib/common.sh (deploy-env for --remote; HXOTA_COMPOSE_FILE, etc.).
# Outputs: real compose output (real run) OR the command that WOULD run (dry-run).
# Side-effects: starts containers on the target. Dry-run: none.
# Dependencies: rootless podman + compose front-end (local or remote); ssh (remote).
# Cross-references: §11.4.161 · §11.4.28 · §11.4.6 · §11.4.18. Doc: docs/remote_deploy/REMOTE_DEPLOY.md.
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
LOCAL=0; case " $HX_REST_ARGS " in *" --local "*) LOCAL=1 ;; esac

if [ "$LOCAL" = "1" ]; then
    _cc="$(hx_compose_cmd)" || hxdie "no rootless compose front-end on host (§11.4.161)"
    if hx_is_dry_run; then
        hxlog "DRY-RUN (local): $_cc -f $HXOTA_COMPOSE_FILE up -d"; exit 0
    fi
    hxlog "Starting LOCAL stack ..."
    # shellcheck disable=SC2086
    exec $_cc -f "$HXOTA_COMPOSE_FILE" up -d
fi

hx_load_deploy_env || exit $?
_remote="cd '$HXOTA_REMOTE_DIR' && \$(command -v podman-compose || echo 'podman compose') --env-file ./stack.env -f compose.svord.yml up -d"
hxlog "Starting REMOTE stack in $HXOTA_REMOTE_DIR ..."
hx_ssh_run "$_remote"
