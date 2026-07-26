#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/publish_artifacts.sh — upload the latest flashable image +
# OTA archives + hash files to the protected dashboard download area via SFTP.
# -----------------------------------------------------------------------------
# Purpose:
#   Publish release artifacts (the flashable image, the HelixOTA OTA archive(s),
#   and their external hash/signature sidecars) into the protected download area
#   the dashboards expose, using the vasic-digital/sftp submodule as the transport.
#
#   DECOUPLING (§11.4.28 / §11.4.6): the sftp submodule is NOT yet incorporated on
#   disk. This wrapper is CONFIG-INJECTED — it invokes the submodule at
#   $HXOTA_SFTP_HOME via $HXOTA_SFTP_ENTRYPOINT and honestly SKIPS when absent, or
#   falls back to the plain `sftp` client for the DESIGNED command shape. The exact
#   submodule CLI is UNCONFIRMED until incorporation (do NOT guess, §11.4.6).
#
# Usage:
#   bash scripts/remote_deploy/publish_artifacts.sh --src <dir> [--dest <remote-path>] \
#        [--dry-run] [--product <id>]
#
# Inputs (env):
#   HXOTA_SFTP_REPO       sftp submodule repo (git@github.com:vasic-digital/sftp.git).
#   HXOTA_SFTP_HOME       Path to the incorporated sftp submodule (absent => SKIP).
#   HXOTA_SFTP_ENTRYPOINT UNCONFIRMED default: $HXOTA_SFTP_HOME/sftp_upload.sh
#   HXOTA_ARTIFACT_DEST   Remote protected download dir
#                         (default: $HXOTA_REMOTE_DIR/srv/downloads).
# Outputs: upload output / dry-run plan (with per-file hashes computed locally).
# Side-effects (real run): uploads files to the remote download area.
# Dependencies: the sftp submodule (absent => SKIP) OR plain `sftp`; sha256sum.
# Cross-references: §11.4.28 · §11.4.6 · §11.4.10 · §11.4.13 (sink-side proof) · §11.4.18.
# =============================================================================
set -euo pipefail
HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"
[ "${HX_WANT_HELP:-0}" = "1" ] && { sed -n '2,40p' "$0"; exit 0; }

# parse --src / --dest out of the remaining args
SRC=""; DEST=""
set -- $HX_REST_ARGS
while [ $# -gt 0 ]; do
    case "$1" in
        --src)  SRC="${2:-}"; shift ;;
        --dest) DEST="${2:-}"; shift ;;
        *) : ;;
    esac
    shift
done
[ -n "$SRC" ] || hxdie "usage: publish_artifacts.sh --src <dir> [--dest <remote-path>]"

hx_load_deploy_env || exit $?
DEST="${DEST:-${HXOTA_ARTIFACT_DEST:-$HXOTA_REMOTE_DIR/srv/downloads}}"
_sftp_home="${HXOTA_SFTP_HOME:-}"
_sftp_entry="${HXOTA_SFTP_ENTRYPOINT:-${_sftp_home}/sftp_upload.sh}"   # UNCONFIRMED default

hxlog "=== publish_artifacts (src=$SRC dest=$DEST product=$HXOTA_PRODUCT) ==="
[ -d "$SRC" ] || hxdie "artifacts source dir does not exist: $SRC"

# Enumerate artifacts + compute hashes locally (sink-side integrity proof, §11.4.13).
hxlog "artifacts to publish (with local sha256):"
for f in "$SRC"/*; do
    [ -f "$f" ] || continue
    if command -v sha256sum >/dev/null 2>&1; then
        _h="$(sha256sum "$f" | awk '{print $1}')"
    else
        _h="(sha256sum unavailable)"
    fi
    printf '  %s  %s\n' "$_h" "$(basename "$f")" >&2
done

# Honest availability gate (§11.4.6).
if [ -z "$_sftp_home" ] || [ ! -d "$_sftp_home" ]; then
    hxwarn "sftp submodule not incorporated (HXOTA_SFTP_HOME unset/absent)."
    hxwarn "Artifact publishing is DESIGNED, not yet live. To wire it:"
    hxwarn "  1) incorporate ${HXOTA_SFTP_REPO:-git@github.com:vasic-digital/sftp.git} (§11.4.28 — design in REMOTE_DEPLOY.md)"
    hxwarn "  2) set HXOTA_SFTP_HOME=<path> and HXOTA_SFTP_ENTRYPOINT=<real CLI> (confirm vs README, §11.4.6)"
    if hx_is_dry_run; then
        hxlog "DRY-RUN designed transport (plain sftp shape, UNCONFIRMED submodule CLI):"
        printf '  sftp -P %s %s <<< "put -r %s/* %s"\n' "$(hx_ssh_port_display)" "$(hx_ssh_target_display)" "$SRC" "$DEST" >&2
    fi
    hxlog "SKIP (no bluff): would upload $SRC/* -> $DEST via the sftp submodule."
    exit 0
fi

if hx_is_dry_run; then
    hxlog "DRY-RUN would run (UNCONFIRMED submodule CLI — confirm before live):"
    printf '  %s --src %s --dest %s --host <ADDRESS_HXOTA> --port <SSH_PORT_HXOTA> --user <HXOTA_DEPLOY_USER>\n' "$_sftp_entry" "$SRC" "$DEST" >&2
    exit 0
fi

[ -x "$_sftp_entry" ] || hxdie "sftp entrypoint not executable: $_sftp_entry (set HXOTA_SFTP_ENTRYPOINT; confirm vs README §11.4.6)."
hxlog "uploading artifacts via the sftp submodule ..."
"$_sftp_entry" --src "$SRC" --dest "$DEST" --host "$ADDRESS_HXOTA" --port "$SSH_PORT_HXOTA" --user "$HXOTA_DEPLOY_USER"
hxlog "publish_artifacts complete."
