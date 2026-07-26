#!/usr/bin/env bash
# =============================================================================
# scripts/remote_deploy/lib/common.sh — shared library for the HelixOTA remote
# deployment + stack-lifecycle scripts.
# -----------------------------------------------------------------------------
# Purpose:
#   Provide the common, project-AGNOSTIC (§11.4.28) machinery every remote-deploy
#   and lifecycle script reuses: deploy-env resolution (config injection, never a
#   hardcoded path), credential-safe logging (§11.4.10 — never echo a secret),
#   rootless-podman helpers (§11.4.161), SSH/rsync/SFTP helpers with a real DRY-RUN
#   mode (print the command that WOULD run, never connect), multi-product resolution
#   (Svord -> {ATMOSphere, Mistiq/VADER}), and host-tool preflight.
#
#   This library is SOURCED, never executed. It carries NO project-specific literal
#   (no ATMOSphere path, no device serial, no product-specific behaviour) — every
#   product/site value is DATA injected via env or a per-product env file (§11.4.28
#   decoupling; §11.4.177 project-agnostic tooling).
#
# Usage (from a script):
#   HXOTA_RD_DIR="$(cd "$(dirname "$0")" && pwd)"
#   . "$HXOTA_RD_DIR/lib/common.sh"
#   hx_parse_common_args "$@"; set -- "$HX_REST_ARGS"   # (scripts re-split as needed)
#   hx_load_deploy_env
#
# Inputs (env, all optional unless a script requires them):
#   HXOTA_DEPLOY_ENV     Path to the gitignored deploy env file holding connection
#                        creds (§11.4.10). Resolution order when unset:
#                          1. ./scripts/testing/secrets/.hxota_deploy.env   (cwd)
#                          2. $HOME/.config/hxota/deploy.env
#                          3. (error — points operator at the .example template)
#   HXOTA_PRODUCT        Product-family id (default: svord).
#   HXOTA_PRODUCTS       Space-separated product ids the stack serves
#                        (default: "atmosphere mistiq_vader"). DATA, never baked in.
#   HXOTA_REMOTE_DIR     Remote bundle dir (default: $HXOTA_DEPLOY_HOME/hxota-stack).
#   HXOTA_COMPOSE_FILE   Consumer compose (default: deploy/svord/compose.svord.yml).
#   HXOTA_SSH_USE_PASSWORD  1 => password auth via SSHPASS env (never argv); default 0
#                           (SSH KEY preferred — QD2). Requires `sshpass` on the host.
#   HXOTA_SSH_KEY        Optional private-key path for key auth.
#   HXOTA_DRY_RUN        1 => dry-run (also --dry-run). Prints, never connects.
#   HXOTA_REDACT         1 => redact host/port/user/secrets in printed output
#                        (default 1). Real connects still use the real values.
#   HXOTA_COMPOSE_CMD    Override the rootless compose front-end (default: autodetect
#                        podman-compose | `podman compose`; §11.4.161).
#
# Deploy-env keys consumed (NAMES only, §11.4.10 — values NEVER printed):
#   ADDRESS_HXOTA, ADDRESS_HXOTA_IPV6, SSH_PORT_HXOTA, HXOTA_DEPLOY_USER,
#   HXOTA_DEPLOY_HOME, HXOTA_DEPLOY_PASSWORD, HXOTA_ROOT_USER, HXOTA_ROOT_PASSWORD,
#   HXOTA_SFTP_REPO.
#
# Outputs:
#   Log lines to stderr ([HXOTA] prefix). Helper functions return non-zero on
#   genuine failure; honest SKIP (exit 0) when a dependency is legitimately absent
#   (§11.4.6 — never a bluff PASS).
#
# Side-effects:
#   In a REAL (non-dry-run) run the SSH/rsync/SFTP helpers contact the remote host.
#   In dry-run they only print. No credential is ever written to stdout/stderr/log.
#
# Dependencies:
#   POSIX sh + coreutils; ssh, rsync, sftp (host); rootless podman + a compose
#   front-end for the local build helpers (§11.4.161). `sshpass` only when
#   HXOTA_SSH_USE_PASSWORD=1.
#
# Cross-references:
#   §11.4.10 credentials · §11.4.28 decoupling · §11.4.161 rootless podman ·
#   §11.4.177 project-agnostic tooling · §11.4.18 script documentation ·
#   §11.4.67 target-shell parseability (POSIX-syntactic; passes sh -n AND bash -n) ·
#   §11.4.6 honest SKIP. Companion doc: docs/remote_deploy/REMOTE_DEPLOY.md.
# =============================================================================

# ---- resolve project root (this lib lives at scripts/remote_deploy/lib/) -----
HXOTA_LIB_DIR="$(cd "$(dirname "$0")" >/dev/null 2>&1 && pwd)"
# When sourced, $0 is the sourcing script (scripts/remote_deploy/<x>.sh); its
# dir is scripts/remote_deploy/. The project root is two levels up.
HXOTA_RD_DIR="${HXOTA_RD_DIR:-$HXOTA_LIB_DIR}"
HXOTA_PROJECT_ROOT="$(cd "$HXOTA_RD_DIR/../.." >/dev/null 2>&1 && pwd)"

# ---- defaults (all overridable via env; DATA, never project literals) -------
HXOTA_PRODUCT="${HXOTA_PRODUCT:-svord}"
HXOTA_PRODUCTS="${HXOTA_PRODUCTS:-atmosphere mistiq_vader}"
HXOTA_COMPOSE_FILE="${HXOTA_COMPOSE_FILE:-$HXOTA_PROJECT_ROOT/deploy/svord/compose.svord.yml}"
HXOTA_SSH_USE_PASSWORD="${HXOTA_SSH_USE_PASSWORD:-0}"
HXOTA_DRY_RUN="${HXOTA_DRY_RUN:-0}"
HXOTA_REDACT="${HXOTA_REDACT:-1}"
HX_REST_ARGS=""

# ---- logging (never emits a credential) -------------------------------------
hxlog()  { printf '[HXOTA] %s\n'  "$*" >&2; }
hxwarn() { printf '[HXOTA][WARN] %s\n' "$*" >&2; }
hxerr()  { printf '[HXOTA][ERROR] %s\n' "$*" >&2; }
hxdie()  { hxerr "$*"; exit 1; }

# ---- common arg parsing (shared across all scripts) -------------------------
# Recognises: --dry-run, --product <id>, --redact/--no-redact, -h|--help.
# Everything else is appended (space-joined) into $HX_REST_ARGS for the caller.
hx_parse_common_args() {
    HX_REST_ARGS=""
    while [ $# -gt 0 ]; do
        case "$1" in
            --dry-run)   HXOTA_DRY_RUN=1 ;;
            --product)   HXOTA_PRODUCT="${2:-}"; shift ;;
            --redact)    HXOTA_REDACT=1 ;;
            --no-redact) HXOTA_REDACT=0 ;;
            -h|--help)   HX_WANT_HELP=1 ;;
            *)           if [ -z "$HX_REST_ARGS" ]; then HX_REST_ARGS="$1"; else HX_REST_ARGS="$HX_REST_ARGS $1"; fi ;;
        esac
        shift
    done
}

hx_is_dry_run() { [ "$HXOTA_DRY_RUN" = "1" ]; }

# ---- deploy-env resolution + load (§11.4.10, §11.4.28 config injection) ------
# Resolves the gitignored deploy env file, sources it, and validates the minimum
# connection keys are present. NEVER prints a value. Fails closed with a clear,
# credential-free message when the file/keys are missing.
hx_resolve_deploy_env() {
    if [ -n "${HXOTA_DEPLOY_ENV:-}" ]; then
        printf '%s\n' "$HXOTA_DEPLOY_ENV"; return 0
    fi
    if [ -f "./scripts/testing/secrets/.hxota_deploy.env" ]; then
        printf '%s\n' "./scripts/testing/secrets/.hxota_deploy.env"; return 0
    fi
    if [ -f "$HOME/.config/hxota/deploy.env" ]; then
        printf '%s\n' "$HOME/.config/hxota/deploy.env"; return 0
    fi
    return 1
}

hx_load_deploy_env() {
    _env=""
    _env="$(hx_resolve_deploy_env)" || {
        hxerr "No deploy-env file found. Set HXOTA_DEPLOY_ENV=<path>, or place it at"
        hxerr "  ./scripts/testing/secrets/.hxota_deploy.env  OR  \$HOME/.config/hxota/deploy.env"
        hxerr "Template: scripts/remote_deploy/.hxota_deploy.env.example (fill in a gitignored copy)."
        return 2
    }
    if [ ! -f "$_env" ]; then
        hxerr "Deploy-env file does not exist: $_env"; return 2
    fi
    # Permission hygiene warning (§11.4.10 — chmod 600 expected). Non-fatal.
    _perm="$(stat -c '%a' "$_env" 2>/dev/null || echo '?')"
    case "$_perm" in
        600|400) : ;;
        \?)      : ;;
        *)       hxwarn "Deploy-env $_env has mode $_perm; §11.4.10 recommends 600." ;;
    esac
    # shellcheck disable=SC1090
    . "$_env"
    HXOTA_DEPLOY_ENV="$_env"
    # Minimum keys for any remote op.
    _missing=""
    [ -n "${ADDRESS_HXOTA:-}" ]     || _missing="$_missing ADDRESS_HXOTA"
    [ -n "${SSH_PORT_HXOTA:-}" ]    || _missing="$_missing SSH_PORT_HXOTA"
    [ -n "${HXOTA_DEPLOY_USER:-}" ] || _missing="$_missing HXOTA_DEPLOY_USER"
    if [ -n "$_missing" ]; then
        hxerr "Deploy-env $_env is missing required key(s):$_missing"
        return 2
    fi
    HXOTA_REMOTE_DIR="${HXOTA_REMOTE_DIR:-${HXOTA_DEPLOY_HOME:-/home/$HXOTA_DEPLOY_USER}/hxota-stack}"
    hxlog "Deploy-env loaded ($_env); connection keys present (values redacted)."
    return 0
}

# ---- SSH target display (redacted; §11.4.10) --------------------------------
# Returns the ssh target for PRINTING ONLY. When HXOTA_REDACT=1 it returns
# variable-name placeholders so no host/port/user leaks into logs/dry-run output.
hx_ssh_target_display() {
    if [ "$HXOTA_REDACT" = "1" ]; then
        printf '%s' '<HXOTA_DEPLOY_USER>@<ADDRESS_HXOTA>'
    else
        printf '%s' "${HXOTA_DEPLOY_USER:-?}@${ADDRESS_HXOTA:-?}"
    fi
}
hx_ssh_port_display() {
    if [ "$HXOTA_REDACT" = "1" ]; then printf '%s' '<SSH_PORT_HXOTA>'; else printf '%s' "${SSH_PORT_HXOTA:-?}"; fi
}

# ---- build the ssh option prefix (real connect) -----------------------------
# Echoes the ssh options string (NOT the target). Values are used verbatim by the
# real-connect path; they are NEVER placed in argv alongside a password.
hx_ssh_opts() {
    _o="-o BatchMode=yes -o ConnectTimeout=${HXOTA_SSH_TIMEOUT:-10} -p ${SSH_PORT_HXOTA}"
    if [ -n "${HXOTA_SSH_KEY:-}" ]; then _o="$_o -i ${HXOTA_SSH_KEY}"; fi
    printf '%s' "$_o"
}

# ---- run a remote command (real) OR print it (dry-run) ----------------------
# $1 = remote shell command (single string). In dry-run: prints the redacted
# command shape and returns 0 WITHOUT connecting. In real mode: executes over ssh.
# Password auth (opt-in) passes the secret via the SSHPASS env var, so it NEVER
# appears in argv / `ps` output (§11.4.10).
hx_ssh_run() {
    _cmd="$1"
    if hx_is_dry_run; then
        hxlog "DRY-RUN ssh (would run, not connecting):"
        printf '  ssh %s -o BatchMode=yes -o ConnectTimeout=%s %s %s\n' \
            "$(hx_ssh_port_display_flag)" '<timeout>' "$(hx_ssh_target_display)" "'$_cmd'" >&2
        return 0
    fi
    if [ "$HXOTA_SSH_USE_PASSWORD" = "1" ]; then
        command -v sshpass >/dev/null 2>&1 || hxdie "HXOTA_SSH_USE_PASSWORD=1 but 'sshpass' is not installed (prefer SSH key auth — QD2)."
        # shellcheck disable=SC2086
        SSHPASS="${HXOTA_DEPLOY_PASSWORD:-}" sshpass -e ssh $(hx_ssh_opts) "${HXOTA_DEPLOY_USER}@${ADDRESS_HXOTA}" "$_cmd"
    else
        # shellcheck disable=SC2086
        ssh $(hx_ssh_opts) "${HXOTA_DEPLOY_USER}@${ADDRESS_HXOTA}" "$_cmd"
    fi
}

# helper: render the -p flag for the redacted dry-run print
hx_ssh_port_display_flag() { printf '%s' "-p $(hx_ssh_port_display)"; }

# ---- rsync a local path to the remote (real) OR print it (dry-run) -----------
# $1 = local source, $2 = remote dest (relative to the remote user home unless
# absolute). Uses -az; extra opts via $3.
hx_rsync_up() {
    _src="$1"; _dst="$2"; _extra="${3:-}"
    _rdisp="$(hx_ssh_target_display):$_dst"
    if hx_is_dry_run; then
        hxlog "DRY-RUN rsync (would run, not connecting):"
        printf '  rsync -az %s -e "ssh %s" %s %s\n' "$_extra" "$(hx_ssh_port_display_flag)" "$_src" "$_rdisp" >&2
        return 0
    fi
    if [ "$HXOTA_SSH_USE_PASSWORD" = "1" ]; then
        command -v sshpass >/dev/null 2>&1 || hxdie "HXOTA_SSH_USE_PASSWORD=1 but 'sshpass' is not installed."
        # shellcheck disable=SC2086
        SSHPASS="${HXOTA_DEPLOY_PASSWORD:-}" rsync -az $_extra -e "sshpass -e ssh $(hx_ssh_opts)" "$_src" "${HXOTA_DEPLOY_USER}@${ADDRESS_HXOTA}:$_dst"
    else
        # shellcheck disable=SC2086
        rsync -az $_extra -e "ssh $(hx_ssh_opts)" "$_src" "${HXOTA_DEPLOY_USER}@${ADDRESS_HXOTA}:$_dst"
    fi
}

# ---- rootless compose front-end autodetect (§11.4.161) ----------------------
hx_compose_cmd() {
    if [ -n "${HXOTA_COMPOSE_CMD:-}" ]; then printf '%s' "$HXOTA_COMPOSE_CMD"; return 0; fi
    if command -v podman-compose >/dev/null 2>&1; then printf '%s' 'podman-compose'; return 0; fi
    if command -v podman >/dev/null 2>&1 && podman compose version >/dev/null 2>&1; then printf '%s' 'podman compose'; return 0; fi
    return 1
}

# ---- remote rootless compose front-end (probed over ssh) --------------------
# Echoes the front-end available ON the remote, or a NO_* marker. Honest, never
# assumes (§11.4.6). Dry-run prints the probe it WOULD run.
hx_remote_compose_cmd() {
    _probe='if command -v podman >/dev/null 2>&1; then if command -v podman-compose >/dev/null 2>&1; then echo podman-compose; elif podman compose version >/dev/null 2>&1; then echo "podman compose"; else echo NO_COMPOSE; fi; else echo NO_PODMAN; fi'
    if hx_is_dry_run; then
        hxlog "DRY-RUN remote compose probe (would run, not connecting):"
        printf '  ssh %s %s %s\n' "$(hx_ssh_port_display_flag)" "$(hx_ssh_target_display)" "'$_probe'" >&2
        printf '%s' 'DRY_RUN'
        return 0
    fi
    hx_ssh_run "$_probe" 2>/dev/null || printf '%s' 'SSH_FAIL'
}

# ---- host-tool preflight ----------------------------------------------------
hx_require_host_cmd() {
    for _c in "$@"; do
        command -v "$_c" >/dev/null 2>&1 || hxdie "required host tool not found: $_c"
    done
}

# ---- product resolution (Svord -> {atmosphere, mistiq_vader}) ---------------
# Loads the per-product env file (DATA) for the active HXOTA_PRODUCT if present.
# The stack itself is product-agnostic; a product file only injects channel/site
# seeding values. Absent product file is a non-fatal SKIP (§11.4.6).
hx_load_product_env() {
    _pf="$HXOTA_PROJECT_ROOT/deploy/svord/products/${HXOTA_PRODUCT}.env"
    if [ -f "$_pf" ]; then
        # shellcheck disable=SC1090
        . "$_pf"
        hxlog "Product env loaded: $HXOTA_PRODUCT ($_pf)."
    else
        hxlog "No product env for '$HXOTA_PRODUCT' at $_pf — using stack defaults (SKIP, §11.4.6)."
    fi
}

hx_each_product() { printf '%s\n' $HXOTA_PRODUCTS; }

# ---- compose validate (local, rootless, no network) -------------------------
hx_compose_config_check() {
    _cc="$(hx_compose_cmd)" || { hxwarn "no rootless compose front-end locally — skip config check (§11.4.161)"; return 0; }
    [ -f "$HXOTA_COMPOSE_FILE" ] || hxdie "compose file missing: $HXOTA_COMPOSE_FILE"
    hxlog "Validating compose ($_cc config -f $HXOTA_COMPOSE_FILE) ..."
    # shellcheck disable=SC2086
    $_cc -f "$HXOTA_COMPOSE_FILE" config >/dev/null
}

# ---- remote bundle preparation + push ---------------------------------------
# Ensure the remote bundle dir exists and push the compose + nginx conf + certs
# scaffolding so the remote can bring the stack up. Dry-run prints the plan.
hx_remote_prepare() {
    _svord="$HXOTA_PROJECT_ROOT/deploy/svord"
    hx_ssh_run "mkdir -p '$HXOTA_REMOTE_DIR/nginx' '$HXOTA_REMOTE_DIR/certs' '$HXOTA_REMOTE_DIR/srv'"
    hx_rsync_up "$_svord/compose.svord.yml"        "$HXOTA_REMOTE_DIR/compose.svord.yml"
    hx_rsync_up "$_svord/nginx/hxota-proxy.conf"   "$HXOTA_REMOTE_DIR/nginx/hxota-proxy.conf"
}

# ---- remote runtime stack.env (RUNTIME secrets only, never ssh/root creds) --
# Writes the runtime secrets the compose needs to a local temp (chmod 600),
# rsyncs to $HXOTA_REMOTE_DIR/stack.env, then removes the local temp. NEVER
# echoes a value (§11.4.10). Fails closed if a required runtime secret is unset.
hx_write_remote_stack_env() {
    if hx_is_dry_run; then
        hxlog "DRY-RUN would write remote stack.env (runtime keys: HXOTA_IMAGE_TAG, HXOTA_PG_*, HXOTA_MINIO_*, HXOTA_TOKEN_SECRET, HXOTA_ARTIFACT_PUBKEY, HXOTA_HTTP_PORT/HXOTA_HTTPS_PORT) and rsync it to $HXOTA_REMOTE_DIR/stack.env (chmod 600)."
        return 0
    fi
    # Fail-closed on the mandatory runtime secrets (QD6 — runtime-secret source).
    _rmiss=""
    [ -n "${HXOTA_PG_PASSWORD:-}" ]   || _rmiss="$_rmiss HXOTA_PG_PASSWORD"
    [ -n "${HXOTA_TOKEN_SECRET:-}" ]  || _rmiss="$_rmiss HXOTA_TOKEN_SECRET"
    [ -n "${HXOTA_MINIO_USER:-}" ]    || _rmiss="$_rmiss HXOTA_MINIO_USER"
    [ -n "${HXOTA_MINIO_PASSWORD:-}" ]|| _rmiss="$_rmiss HXOTA_MINIO_PASSWORD"
    if [ -n "$_rmiss" ]; then
        hxerr "Missing runtime secret(s) in the deploy env:$_rmiss (QD6 — see REMOTE_DEPLOY.md). Refusing to bring up a half-configured / insecure stack (§11.4.6, fail-closed)."
        return 2
    fi
    _tmp="$(mktemp)"; chmod 600 "$_tmp"
    {
        printf 'HXOTA_IMAGE_TAG=%s\n'        "${HXOTA_IMAGE_TAG:-svord}"
        printf 'HXOTA_PG_USER=%s\n'          "${HXOTA_PG_USER:-helix}"
        printf 'HXOTA_PG_DB=%s\n'            "${HXOTA_PG_DB:-helix_ota}"
        printf 'HXOTA_PG_PASSWORD=%s\n'      "${HXOTA_PG_PASSWORD}"
        printf 'HXOTA_MINIO_USER=%s\n'       "${HXOTA_MINIO_USER}"
        printf 'HXOTA_MINIO_PASSWORD=%s\n'   "${HXOTA_MINIO_PASSWORD}"
        printf 'HXOTA_TOKEN_SECRET=%s\n'     "${HXOTA_TOKEN_SECRET}"
        printf 'HXOTA_ARTIFACT_PUBKEY=%s\n'  "${HXOTA_ARTIFACT_PUBKEY:-}"
        printf 'HXOTA_ARTIFACT_BASE_URL=%s\n' "${HXOTA_ARTIFACT_BASE_URL:-https://hxota.dev/artifacts}"
        printf 'HXOTA_HTTP_PORT=%s\n'        "${HXOTA_HTTP_PORT:-8080}"
        printf 'HXOTA_HTTPS_PORT=%s\n'       "${HXOTA_HTTPS_PORT:-8443}"
        printf 'HXOTA_MAX_INFLIGHT=%s\n'     "${HXOTA_MAX_INFLIGHT:-256}"
    } > "$_tmp"
    hx_rsync_up "$_tmp" "$HXOTA_REMOTE_DIR/stack.env" "--chmod=F600"
    rm -f "$_tmp"
    hxlog "remote stack.env written (runtime secrets; values redacted)."
}

# ---- remote compose action (up/down/ps/build) for a service or the whole stack
# $1 = action (up -d | down | ps | build | restart), $2 = optional service name.
hx_compose_remote() {
    _action="$1"; _svc="${2:-}"
    _cc='CC=$(command -v podman-compose || echo "podman compose")'
    hx_ssh_run "cd '$HXOTA_REMOTE_DIR' && $_cc && \$CC --env-file ./stack.env -f compose.svord.yml $_action $_svc"
}

# ---- live sink-side health confirmation (§11.4.13) --------------------------
# Probe the proxy /healthz + the API /readyz ON the remote (loopback). Honest
# verdict — never claims HEALTHY without a real 200 (§11.4.6).
hx_remote_health_confirm() {
    _hp="${HXOTA_HTTP_PORT:-8080}"
    _probe="echo '--- proxy /healthz ---'; (curl -sf http://127.0.0.1:$_hp/healthz >/dev/null 2>&1 && echo PROXY_HEALTHY) || echo PROXY_UNHEALTHY; echo '--- api /readyz ---'; (curl -sf http://127.0.0.1:$_hp/api/v1/../readyz >/dev/null 2>&1 || curl -sf http://127.0.0.1:8080/readyz >/dev/null 2>&1 && echo API_READY) || echo API_NOT_READY"
    hx_ssh_run "$_probe"
}
