#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/restart.sh — restart the HelixOTA stack (down then up).
# -----------------------------------------------------------------------------
# Purpose:
#   Idempotent restart: `down` (removes stale containers so an `up` re-reads the
#   image/config — podman-compose `up` does NOT recreate a running container) then
#   `up -d`. Named volumes persist. Default REMOTE; --local for a local stack.
#
# Usage:
#   bash scripts/remote_deploy/restart.sh [--local] [--dry-run] [--product <id>]
#
# Inputs (env): lib/common.sh.  Outputs: compose output / dry-run print.
# Side-effects: recreates the stack's containers on the target (volumes kept).
# Dependencies: rootless podman + compose (local or remote); ssh (remote).
# Cross-references: §11.4.161 · §11.4.6 · §11.4.18.
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
LOCAL=0; case " $HX_REST_ARGS " in *" --local "*) LOCAL=1 ;; esac

if [ "$LOCAL" = "1" ]; then
    _cc="$(hx_compose_cmd)" || hxdie "no rootless compose front-end on host (§11.4.161)"
    if hx_is_dry_run; then
        hxlog "DRY-RUN (local): $_cc -f $HXOTA_COMPOSE_FILE down; $_cc -f $HXOTA_COMPOSE_FILE up -d"; exit 0
    fi
    hxlog "Restarting LOCAL stack ..."
    # shellcheck disable=SC2086
    $_cc -f "$HXOTA_COMPOSE_FILE" down 2>/dev/null || true
    # shellcheck disable=SC2086
    exec $_cc -f "$HXOTA_COMPOSE_FILE" up -d
fi

hx_load_deploy_env || exit $?
_remote="cd '$HXOTA_REMOTE_DIR' && CC=\$(command -v podman-compose || echo 'podman compose') && \$CC --env-file ./stack.env -f compose.svord.yml down 2>/dev/null; \$CC --env-file ./stack.env -f compose.svord.yml up -d"
hxlog "Restarting REMOTE stack in $HXOTA_REMOTE_DIR ..."
hx_ssh_run "$_remote"
