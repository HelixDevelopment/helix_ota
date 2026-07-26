#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/status.sh — report the HelixOTA stack status + health.
# -----------------------------------------------------------------------------
# Purpose:
#   Show container state (`compose ps`) and probe the proxy health endpoint on the
#   target. Default REMOTE (over SSH); --local for a local stack. Read-only —
#   never mutates the stack. Honest health verdict (§11.4.6): HEALTHY only if the
#   probe genuinely returns 200.
#
# Usage:
#   bash scripts/remote_deploy/status.sh [--local] [--dry-run] [--product <id>]
#
# Inputs (env): lib/common.sh (HXOTA_HTTP_PORT for the health probe, default 8080).
# Outputs: `compose ps` output + a HEALTHY/UNHEALTHY line.  Side-effects: none.
# Dependencies: rootless podman + compose (local or remote); ssh (remote); wget/curl on target.
# Cross-references: §11.4.161 · §11.4.6 · §11.4.13 (sink-side health) · §11.4.18.
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
LOCAL=0; case " $HX_REST_ARGS " in *" --local "*) LOCAL=1 ;; esac
_hp="${HXOTA_HTTP_PORT:-8080}"

if [ "$LOCAL" = "1" ]; then
    _cc="$(hx_compose_cmd)" || hxdie "no rootless compose front-end on host (§11.4.161)"
    if hx_is_dry_run; then
        hxlog "DRY-RUN (local): $_cc -f $HXOTA_COMPOSE_FILE ps ; curl -sf http://127.0.0.1:$_hp/healthz"; exit 0
    fi
    hxlog "LOCAL stack status:"
    # shellcheck disable=SC2086
    $_cc -f "$HXOTA_COMPOSE_FILE" ps || true
    if curl -sf "http://127.0.0.1:$_hp/healthz" >/dev/null 2>&1; then hxlog "proxy: HEALTHY (127.0.0.1:$_hp)"; else hxwarn "proxy: UNHEALTHY / not up (127.0.0.1:$_hp)"; fi
    exit 0
fi

hx_load_deploy_env || exit $?
_remote="cd '$HXOTA_REMOTE_DIR' && CC=\$(command -v podman-compose || echo 'podman compose') && \$CC --env-file ./stack.env -f compose.svord.yml ps; echo '--- health ---'; (curl -sf http://127.0.0.1:$_hp/healthz >/dev/null 2>&1 && echo HEALTHY) || echo UNHEALTHY"
hxlog "REMOTE stack status ($HXOTA_REMOTE_DIR):"
hx_ssh_run "$_remote"
