#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/https_certs.sh — HTTPS cert lifecycle via the lets_encrypt
# submodule (issue / renew / rotate for hxota.dev + hxota.com).
# -----------------------------------------------------------------------------
# Purpose:
#   Obtain / renew / rotate TLS certificates for the console + website domains
#   using the vasic-digital/lets_encrypt submodule, stage them into
#   deploy/svord/certs/<domain>/{fullchain.pem,privkey.pem} (rsynced to the remote
#   bundle + bind-mounted into the nginx proxy), and after rotation run
#   LE_RELOAD_CMD to reload the web/proxy container so the new cert is served.
#
#   DECOUPLING (§11.4.28 / §11.4.6): the lets_encrypt submodule is NOT yet
#   incorporated on disk. This wrapper is CONFIG-INJECTED — it invokes the
#   submodule at $LETS_ENCRYPT_HOME via $HXOTA_LE_ENTRYPOINT and honestly SKIPS
#   when absent. The exact submodule CLI is UNCONFIRMED until incorporation — set
#   HXOTA_LE_ENTRYPOINT to the real entrypoint once the submodule README is read
#   (do NOT guess, §11.4.6).
#
# Usage:
#   bash scripts/remote_deploy/https_certs.sh {issue|renew|rotate} [--dry-run] [--product <id>]
#
# Inputs (env):
#   LETS_ENCRYPT_HOME   Path to the incorporated lets_encrypt submodule.
#   HXOTA_LE_ENTRYPOINT UNCONFIRMED default: $LETS_ENCRYPT_HOME/lets_encrypt.sh
#   HXOTA_LE_DOMAINS    Domains (default: "hxota.dev hxota.com").
#   HXOTA_LE_EMAIL      ACME account email (operator-provided; QD).
#   HXOTA_CERT_DIR      Local cert stage dir (default deploy/svord/certs).
#   LE_RELOAD_CMD       Reload command after rotation (default: restart the remote proxy).
# Outputs: cert-issue output / dry-run plan.  Side-effects (real run): writes
#   certs into HXOTA_CERT_DIR, rsyncs to the remote, reloads the proxy.
# Dependencies: the lets_encrypt submodule (absent => honest SKIP); ssh, rsync.
# Cross-references: §11.4.28 · §11.4.6 · §11.4.161 · §11.4.10 · §11.4.18 · §11.4.99 (confirm the CLI against latest submodule docs before live use).
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
[ "${HX_WANT_HELP:-0}" = "1" ] && { sed -n '2,40p' "$0"; exit 0; }

ACTION="$(printf '%s' "$HX_REST_ARGS" | awk '{print $1}')"
case "$ACTION" in issue|renew|rotate) : ;; *) hxdie "usage: https_certs.sh {issue|renew|rotate} [--dry-run]" ;; esac

_le_home="${LETS_ENCRYPT_HOME:-}"
_le_entry="${HXOTA_LE_ENTRYPOINT:-${_le_home}/lets_encrypt.sh}"   # UNCONFIRMED default
_domains="${HXOTA_LE_DOMAINS:-hxota.dev hxota.com}"
_cert_dir="${HXOTA_CERT_DIR:-$HXOTA_PROJECT_ROOT/deploy/svord/certs}"

hxlog "=== https_certs $ACTION (domains: $_domains) ==="

# Honest availability gate (§11.4.6): the submodule is not yet incorporated.
if [ -z "$_le_home" ] || [ ! -d "$_le_home" ]; then
    hxwarn "lets_encrypt submodule not incorporated (LETS_ENCRYPT_HOME unset/absent)."
    hxwarn "HTTPS cert wiring is DESIGNED, not yet live. To wire it:"
    hxwarn "  1) incorporate git@github.com:vasic-digital/lets_encrypt.git (§11.4.28 — design in REMOTE_DEPLOY.md)"
    hxwarn "  2) set LETS_ENCRYPT_HOME=<path> and HXOTA_LE_ENTRYPOINT=<real CLI> (confirm vs its README, §11.4.6/§11.4.99)"
    hxlog "SKIP (no bluff): would '$ACTION' certs for [$_domains] into $_cert_dir via the lets_encrypt submodule."
    exit 0
fi

if hx_is_dry_run; then
    hxlog "DRY-RUN would run (UNCONFIRMED CLI — confirm before live):"
    for d in $_domains; do
        printf '  %s %s --domain %s --email <HXOTA_LE_EMAIL> --out %s/%s\n' "$_le_entry" "$ACTION" "$d" "$_cert_dir" "$d" >&2
    done
    printf '  # then rsync %s -> remote + reload proxy (LE_RELOAD_CMD)\n' "$_cert_dir" >&2
    exit 0
fi

# Real invocation (only reached when the submodule IS present).
[ -x "$_le_entry" ] || hxdie "lets_encrypt entrypoint not executable: $_le_entry (set HXOTA_LE_ENTRYPOINT; confirm vs README §11.4.6)."
mkdir -p "$_cert_dir"
for d in $_domains; do
    hxlog "cert $ACTION for $d ..."
    "$_le_entry" "$ACTION" --domain "$d" --email "${HXOTA_LE_EMAIL:?set HXOTA_LE_EMAIL}" --out "$_cert_dir/$d"
done

# Push certs to the remote bundle + reload the proxy.
hx_load_deploy_env || exit $?
hx_ssh_run "mkdir -p '$HXOTA_REMOTE_DIR/certs'"
hx_rsync_up "$_cert_dir/" "$HXOTA_REMOTE_DIR/certs/" "--chmod=D700,F600"
if [ -n "${LE_RELOAD_CMD:-}" ]; then
    hxlog "running LE_RELOAD_CMD ..."; hx_ssh_run "$LE_RELOAD_CMD"
else
    hxlog "reloading proxy (default LE_RELOAD_CMD) ..."; hx_compose_remote "restart" "proxy"
fi
hxlog "https_certs $ACTION complete."
