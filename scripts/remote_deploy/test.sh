#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/test.sh — local validation / self-test of the HelixOTA
# remote-deploy scaffold (anti-bluff evidence harness).
# -----------------------------------------------------------------------------
# Purpose:
#   Prove the scaffold is REAL and consistent WITHOUT contacting any live host
#   (§11.4.6 honest, §11.4.108 source/artifact layer). Runs, in order:
#     1. host-tool preflight (ssh, rsync)
#     2. sh -n + bash -n on EVERY remote-deploy script (§11.4.67 parseability)
#     3. rootless `podman compose config` on the consumer compose (§11.4.161)
#     4. deploy-env RESOLUTION report (present/absent — never connects, never
#        echoes a value, §11.4.10)
#     5. lets_encrypt + sftp submodule availability (present/absent -> honest SKIP)
#   Exits non-zero on any genuine failure (parse error, invalid compose). A
#   legitimately-absent optional dependency is a SKIP, not a FAIL (§11.4.6).
#
# Usage:
#   bash scripts/remote_deploy/test.sh [--no-redact]
#
# Inputs (env): see lib/common.sh. HXOTA_DEPLOY_ENV optionally points at the
#   gitignored deploy env (only its RESOLUTION is reported here; no connect).
#
# Outputs: a PASS/FAIL/SKIP report to stderr; exit 0 iff no FAIL.
#
# Side-effects: none (no remote connection, no container run). Read-only.
#
# Dependencies: bash, sh, ssh, rsync (host); rootless podman + compose front-end
#   for the compose config check (skipped honestly if absent).
#
# Cross-references: §11.4.6 · §11.4.10 · §11.4.28 · §11.4.67 · §11.4.108 ·
#   §11.4.161. Companion doc: docs/remote_deploy/REMOTE_DEPLOY.md.
# =============================================================================
set -euo pipefail

HXOTA_RD_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
. "$HXOTA_RD_DIR/lib/common.sh"
hx_parse_common_args "$@"

FAILS=0
PASSES=0
SKIPS=0
_pass() { PASSES=$((PASSES+1)); hxlog "PASS: $*"; }
_fail() { FAILS=$((FAILS+1));  hxerr "FAIL: $*"; }
_skip() { SKIPS=$((SKIPS+1));  hxlog "SKIP: $*"; }

# --- 1. host-tool preflight ---------------------------------------------------
for _c in ssh rsync; do
    if command -v "$_c" >/dev/null 2>&1; then _pass "host tool present: $_c"; else _fail "host tool MISSING: $_c"; fi
done

# --- 2. sh -n + bash -n on every remote-deploy script ------------------------
for _f in "$HXOTA_RD_DIR"/*.sh "$HXOTA_RD_DIR"/lib/*.sh; do
    [ -f "$_f" ] || continue
    _b="$(basename "$_f")"
    if bash -n "$_f" 2>/dev/null; then _pass "bash -n $_b"; else _fail "bash -n $_b"; fi
    if sh   -n "$_f" 2>/dev/null; then _pass "sh -n   $_b"; else _fail "sh -n   $_b"; fi
done

# --- 3. rootless compose config check ----------------------------------------
if _cc="$(hx_compose_cmd)"; then
    if [ -f "$HXOTA_COMPOSE_FILE" ]; then
        # shellcheck disable=SC2086
        if HXOTA_PG_PASSWORD=x HXOTA_MINIO_USER=x HXOTA_MINIO_PASSWORD=x HXOTA_TOKEN_SECRET=x \
             $_cc -f "$HXOTA_COMPOSE_FILE" config >/dev/null 2>/tmp/hxota_compose_config.$$; then
            _pass "compose config valid ($_cc): $(basename "$HXOTA_COMPOSE_FILE")"
        else
            _fail "compose config INVALID ($_cc): $(basename "$HXOTA_COMPOSE_FILE") — $(head -3 /tmp/hxota_compose_config.$$ 2>/dev/null | tr '\n' ' ')"
        fi
        rm -f /tmp/hxota_compose_config.$$ 2>/dev/null || true
    else
        _fail "compose file missing: $HXOTA_COMPOSE_FILE"
    fi
else
    _skip "no rootless compose front-end on host — compose config check skipped (§11.4.161/§11.4.6)"
fi

# --- 4. deploy-env resolution report (no connect, no echo) -------------------
if _env="$(hx_resolve_deploy_env 2>/dev/null)"; then
    if [ -f "$_env" ]; then
        _pass "deploy-env resolved (present, values redacted): $_env"
    else
        _skip "deploy-env resolved to a non-existent path: $_env (operator provisions it)"
    fi
else
    _skip "no deploy-env found — set HXOTA_DEPLOY_ENV or provision the gitignored file (template: .hxota_deploy.env.example)"
fi

# --- 5. lets_encrypt + sftp submodule availability ---------------------------
if [ -n "${LETS_ENCRYPT_HOME:-}" ] && [ -d "${LETS_ENCRYPT_HOME:-/nonexistent}" ]; then
    _pass "lets_encrypt submodule available: $LETS_ENCRYPT_HOME"
else
    _skip "lets_encrypt submodule not incorporated yet (design-doc'd) — HTTPS wiring is DESIGNED, not yet live (§11.4.6)"
fi
if [ -n "${HXOTA_SFTP_HOME:-}" ] && [ -d "${HXOTA_SFTP_HOME:-/nonexistent}" ]; then
    _pass "sftp submodule available: $HXOTA_SFTP_HOME"
else
    _skip "sftp submodule not incorporated yet (design-doc'd) — artifact upload is DESIGNED, not yet live (§11.4.6)"
fi

# --- summary -----------------------------------------------------------------
hxlog "----------------------------------------------------------------"
hxlog "test.sh summary: PASS=$PASSES  FAIL=$FAILS  SKIP=$SKIPS"
if [ "$FAILS" -gt 0 ]; then
    hxerr "test.sh: $FAILS failure(s) — scaffold NOT clean."
    exit 1
fi
hxlog "test.sh: scaffold validation GREEN (no live host contacted)."
exit 0
