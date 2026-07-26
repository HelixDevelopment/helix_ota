#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/stop.sh — bring the HelixOTA stack DOWN.
# -----------------------------------------------------------------------------
# Purpose:
#   Stop the containerized stack via rootless podman compose (§11.4.161). Named
#   volumes (postgres/minio data) PERSIST across `down` — data is NOT destroyed.
#   Default target REMOTE; --local for a local stack.
#
# Usage:
#   bash scripts/remote_deploy/stop.sh [--local] [--dry-run] [--product <id>]
#
# Inputs (env): lib/common.sh.  Outputs: compose output / dry-run print.
# Side-effects: stops+removes the stack's containers on the target (volumes kept).
# Dependencies: rootless podman + compose (local or remote); ssh (remote).
# Cross-references: §11.4.161 · §11.4.6 (volumes preserved) · §11.4.18.
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
LOCAL=0; case " $HX_REST_ARGS " in *" --local "*) LOCAL=1 ;; esac

if [ "$LOCAL" = "1" ]; then
    _cc="$(hx_compose_cmd)" || hxdie "no rootless compose front-end on host (§11.4.161)"
    if hx_is_dry_run; then hxlog "DRY-RUN (local): $_cc -f $HXOTA_COMPOSE_FILE down"; exit 0; fi
    hxlog "Stopping LOCAL stack (named volumes preserved) ..."
    # shellcheck disable=SC2086
    exec $_cc -f "$HXOTA_COMPOSE_FILE" down
fi

hx_load_deploy_env || exit $?
_remote="cd '$HXOTA_REMOTE_DIR' && \$(command -v podman-compose || echo 'podman compose') --env-file ./stack.env -f compose.svord.yml down"
hxlog "Stopping REMOTE stack in $HXOTA_REMOTE_DIR (named volumes preserved) ..."
hx_ssh_run "$_remote"
