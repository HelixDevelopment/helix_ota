#!/usr/bin/env bash
#
# boot_real_system.sh — boot the REAL Helix OTA full system (ota-server backed
# by a REAL PostgreSQL) on a rootless-podman host, and prove it live.
#
# Purpose
#   The FOUNDATIONAL real-system enabler (F-CLUSTER). Brings up
#   server/deploy/system.compose.yml — the ACTUAL control plane wired to a REAL
#   PostgreSQL (HELIX_DATABASE_URL set => pgx Repository, NOT the in-memory
#   store) — waits for /readyz -> 200, and prints the live base URL. This is the
#   SINGLE entry point higher test suites (integration / e2e / full-automation /
#   security / stress / chaos per §11.4.27) call to obtain a live instance, so
#   they hit one real system instead of each re-booting infra.
#
#   The on-demand-infra invariant (§11.4.76 / §11.4.161): the boot IS the test
#   entry point — no operator runs `podman-compose up` by hand. Rootless podman
#   only; no sudo / root / host networking (§11.4.161 / §12).
#
# Usage
#   bash tests/lib/boot_real_system.sh              # boot on the default target
#   bash tests/lib/boot_real_system.sh --up         # explicit boot (same as bare)
#   bash tests/lib/boot_real_system.sh --down        # tear the stack down + clean
#   TARGET=user@host bash tests/lib/boot_real_system.sh
#
# Inputs (environment, all optional — sane defaults)
#   TARGET          ssh target running rootless podman (default: thinker.local
#                   as user from $REMOTE_USER, default milosvasic)
#   REMOTE_USER     ssh user when TARGET has no user@ (default: milosvasic)
#   PROJECT         compose project name (default: helix-ota-system) — distinct
#                   from the integration suite so the two never contend (§11.4.119)
#   API_HOST_PORT   host port the API is published on, on the remote (default 18080)
#   READY_TIMEOUT   seconds to wait for /readyz -> 200 (default 90)
#   SYSTEM_ARTIFACT_PUBKEY  base64 ed25519 pubkey (default: a throwaway test key
#                   generated per boot; never committed)
#
#   CALLER-PUBKEY MODE (opt-in, §11.4.69 — lets the caller sign artifacts the
#   LIVE server accepts, so signed-pipeline / trust-boundary tests can drive the
#   real upload path instead of SKIPping):
#     HELIX_SYSTEM_SIGNING_KEY  path to a caller-supplied ed25519 PRIVATE PEM.
#                   When set, the harness derives the raw 32-byte PUBLIC key from
#                   it and sets the server's HELIX_ARTIFACT_PUBKEY to THAT public
#                   key — the caller keeps the private half and can produce
#                   signatures the live server's config-trusted key verifies. The
#                   harness NEVER reads, transmits, or stores the private key
#                   (only the derived public half crosses to the remote).
#     SYSTEM_ARTIFACT_PUBKEY    alternatively, the caller may pass the base64 raw
#                   ed25519 public key directly (when it generated the keypair
#                   itself) — this passthrough is honored unchanged.
#   Default (neither set): a throwaway keypair is generated per boot and the
#   private half is discarded (no caller can sign accepted artifacts — the legacy
#   behavior, intact). Caller-pubkey mode is therefore strictly opt-in.
#
# Outputs / Side-effects
#   --up   : cross-compiles a static linux/amd64 ota-server on the HOST (the
#            §11.4.28 sibling `replace` directives resolve on the host), rsyncs a
#            minimal build bundle to the remote, `podman-compose build` + `up -d`,
#            waits for /readyz, prints `BASE_URL=http://<remote>:<port>` on stdout.
#   --down : `podman-compose down -v` for the project on the remote + removes the
#            remote bundle dir + the built image (§11.4.14 cleanup). Leaves the
#            integration suite's own resources untouched (project-name scoped).
#
# Dependencies: ssh + rsync to a rootless-podman host (podman + podman-compose),
#   Go 1.26 toolchain on the host.
# Cross-references: server/deploy/system.compose.yml, server/Dockerfile,
#   server/cmd/ota-server/main.go, server/internal/config/config.go.

set -euo pipefail

# --- resolve paths ------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"
DEPLOY_DIR="${SERVER_DIR}/deploy"
COMPOSE_FILE="${DEPLOY_DIR}/system.compose.yml"

# --- config (env-overridable) -------------------------------------------------
REMOTE_USER="${REMOTE_USER:-milosvasic}"
TARGET="${TARGET:-${REMOTE_USER}@thinker.local}"
case "${TARGET}" in *@*) ;; *) TARGET="${REMOTE_USER}@${TARGET}" ;; esac
REMOTE_HOST="${TARGET#*@}"

PROJECT="${PROJECT:-helix-ota-system}"
API_HOST_PORT="${API_HOST_PORT:-18080}"
READY_TIMEOUT="${READY_TIMEOUT:-90}"

# Remote staging dir for the build bundle (binary + Dockerfile + compose).
REMOTE_BUNDLE="/home/${REMOTE_USER}/.helix-ota-system-stack"
# Local staging dir the Dockerfile COPYs the binary from (gitignored).
CP_STAGE="${SERVER_DIR}/.docker-bin"

SSH="ssh -o BatchMode=yes -o ConnectTimeout=15"

log() { printf '%s %s\n' "[$(date -u +%H:%M:%SZ)]" "$*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

# Build the remote `podman-compose` command for our project + file. The compose
# file lives at REMOTE_BUNDLE/system.compose.yml; build context (..) resolves to
# REMOTE_BUNDLE, where we also stage the Dockerfile + .docker-bin/ota-server.
remote_compose() {
    # shellcheck disable=SC2029  # we intentionally expand locally.
    $SSH "${TARGET}" \
        "cd '${REMOTE_BUNDLE}/deploy' && SYSTEM_ARTIFACT_PUBKEY='${SYSTEM_ARTIFACT_PUBKEY:-}' podman-compose -p '${PROJECT}' -f system.compose.yml $*"
}

# ==============================================================================
# teardown
# ==============================================================================
do_down() {
    log "DOWN: tearing down project '${PROJECT}' on ${TARGET}"
    # down -v removes the project's containers + named volumes (project-scoped:
    # the integration suite's own project is untouched, §11.4.119).
    $SSH "${TARGET}" \
        "cd '${REMOTE_BUNDLE}/deploy' 2>/dev/null && podman-compose -p '${PROJECT}' -f system.compose.yml down -v" \
        >/dev/null 2>&1 || true
    # Belt-and-suspenders: remove any lingering project containers by name prefix.
    $SSH "${TARGET}" \
        "podman ps -a --filter 'label=io.podman.compose.project=${PROJECT}' -q | xargs -r podman rm -f" \
        >/dev/null 2>&1 || true
    # Remove the built image + bundle dir.
    $SSH "${TARGET}" "podman image rm -f ota-control-plane:system" >/dev/null 2>&1 || true
    $SSH "${TARGET}" "rm -rf '${REMOTE_BUNDLE}'" >/dev/null 2>&1 || true
    rm -rf "${CP_STAGE}" 2>/dev/null || true
    log "DOWN: complete (project '${PROJECT}' removed; integration suite untouched)."
}

# ==============================================================================
# boot
# ==============================================================================
do_up() {
    log "UP: booting REAL system (ota-server + real PostgreSQL) on ${TARGET}"

    # --- preflight: remote reachable + rootless podman ------------------------
    $SSH "${TARGET}" 'podman --version && podman-compose --version' >/dev/null 2>&1 \
        || fail "remote podman/podman-compose not reachable on ${TARGET}"

    # Idempotent re-run: force-clean any leftover stack BEFORE building up. On a
    # re-run, podman-compose's internal recreate cannot remove a postgres that a
    # prior run's ota-server still depends on ("has dependent containers" /
    # "name already in use", exit 125). do_down force-removes by project label
    # (dependency-order-safe), so `up` always starts from a clean slate.
    log "PRE-CLEAN: force-removing any prior '${PROJECT}' stack"
    do_down >/dev/null 2>&1 || true

    # Detect remote arch so we cross-compile the matching binary.
    REMOTE_ARCH="$($SSH "${TARGET}" 'uname -m' 2>/dev/null || echo unknown)"
    case "${REMOTE_ARCH}" in
        x86_64|amd64) GOARCH=amd64 ;;
        aarch64|arm64) GOARCH=arm64 ;;
        *) fail "unsupported remote arch '${REMOTE_ARCH}'" ;;
    esac
    log "remote arch=${REMOTE_ARCH} => cross-compiling GOOS=linux GOARCH=${GOARCH}"

    # --- resolve the artifact pubkey the live server will trust ---------------
    # Precedence (§11.4.6 deterministic):
    #   (1) CALLER-PUBKEY MODE: HELIX_SYSTEM_SIGNING_KEY points at a caller's
    #       ed25519 PRIVATE PEM => derive its raw 32-byte PUBLIC key here and
    #       trust THAT. The caller keeps the private half + signs accepted
    #       artifacts. Only the public half ever leaves the caller's control.
    #   (2) SYSTEM_ARTIFACT_PUBKEY passed directly (caller generated the keypair).
    #   (3) Default: throwaway per-boot pubkey, private half discarded (legacy).
    if [ -n "${HELIX_SYSTEM_SIGNING_KEY:-}" ]; then
        [ -r "${HELIX_SYSTEM_SIGNING_KEY}" ] \
            || fail "caller-pubkey mode: HELIX_SYSTEM_SIGNING_KEY='${HELIX_SYSTEM_SIGNING_KEY}' is not a readable file"
        SYSTEM_ARTIFACT_PUBKEY="$(_pubkey_b64_from_priv_pem "${HELIX_SYSTEM_SIGNING_KEY}")" \
            || fail "caller-pubkey mode: could not derive ed25519 public key from HELIX_SYSTEM_SIGNING_KEY (need openssl >=3 ed25519)"
        export SYSTEM_ARTIFACT_PUBKEY
        log "CALLER-PUBKEY MODE: live server trusts the public key derived from the caller's signing key (private half stays with caller)"
    elif [ -z "${SYSTEM_ARTIFACT_PUBKEY:-}" ]; then
        # Generate an ed25519 keypair; export only the base64 raw 32-byte pubkey.
        SYSTEM_ARTIFACT_PUBKEY="$(_gen_ed25519_pubkey_b64)" \
            || log "note: could not generate test pubkey; HELIX_ARTIFACT_PUBKEY stays empty (tolerated)"
        export SYSTEM_ARTIFACT_PUBKEY
    else
        log "CALLER-PUBKEY MODE: using caller-supplied SYSTEM_ARTIFACT_PUBKEY directly"
    fi

    # --- cross-compile static linux/<arch> ota-server on the host -------------
    mkdir -p "${CP_STAGE}"
    log "BUILD: cross-compile static ota-server (host — sibling replaces resolve here)"
    ( cd "${SERVER_DIR}" \
      && CGO_ENABLED=0 GOOS=linux GOARCH="${GOARCH}" go build -trimpath -ldflags="-s -w" \
           -o "${CP_STAGE}/ota-server" ./cmd/ota-server )
    [ -x "${CP_STAGE}/ota-server" ] || fail "ota-server binary not produced"
    log "built: $(ls -la "${CP_STAGE}/ota-server" | awk '{print $5, $9}')"

    # --- stage a minimal build bundle + rsync to the remote -------------------
    # The bundle mirrors what the compose build needs: the Dockerfile + the
    # staged binary at .docker-bin/ + the compose file under deploy/.
    local stage; stage="$(mktemp -d)"
    mkdir -p "${stage}/.docker-bin" "${stage}/deploy"
    cp "${SERVER_DIR}/Dockerfile" "${stage}/Dockerfile"
    cp "${CP_STAGE}/ota-server" "${stage}/.docker-bin/ota-server"
    cp "${COMPOSE_FILE}" "${stage}/deploy/system.compose.yml"

    log "SYNC: rsync build bundle -> ${TARGET}:${REMOTE_BUNDLE}"
    $SSH "${TARGET}" "mkdir -p '${REMOTE_BUNDLE}'" || fail "cannot create remote bundle dir"
    rsync -a --delete -e "${SSH}" "${stage}/" "${TARGET}:${REMOTE_BUNDLE}/" \
        || fail "rsync of build bundle failed"
    rm -rf "${stage}"

    # --- bring the stack up ---------------------------------------------------
    # podman-compose 1.0.6 does NOT enforce `depends_on: condition:
    # service_healthy` — it would start both containers at once, and ota-server
    # hard-exits (log.Fatalf) if postgres is not yet accepting connections (the
    # ~8s postgres init window). So we orchestrate ordering EXPLICITLY: start
    # postgres, wait for it to actually accept connections, THEN start ota-server.
    log "COMPOSE: podman-compose build (project '${PROJECT}')"
    remote_compose build || fail "podman-compose build failed (see remote output above)"

    log "COMPOSE: up -d postgres (first), then wait for it to accept connections"
    remote_compose up -d postgres || fail "podman-compose up postgres failed"

    log "PG-WAIT: waiting up to 60s for postgres to accept connections"
    local pg_deadline=$(( $(date +%s) + 60 )) pg_ready=0
    while [ "$(date +%s)" -lt "${pg_deadline}" ]; do
        if $SSH "${TARGET}" "podman exec ${PROJECT}_postgres_1 pg_isready -U helix -d helix_ota" >/dev/null 2>&1; then
            pg_ready=1
            log "PG-WAIT: postgres accepting connections."
            break
        fi
        sleep 2
    done
    [ "${pg_ready}" -eq 1 ] || fail "postgres never became ready within 60s"

    log "COMPOSE: up -d ota-server (postgres now ready => server connect+Migrate succeeds)"
    remote_compose up -d ota-server || fail "podman-compose up ota-server failed"

    # --- wait for /readyz -> 200 ----------------------------------------------
    log "READY: waiting up to ${READY_TIMEOUT}s for /readyz -> 200"
    local base="http://${REMOTE_HOST}:${API_HOST_PORT}"
    local deadline=$(( $(date +%s) + READY_TIMEOUT ))
    local ready=0 code body
    while [ "$(date +%s)" -lt "${deadline}" ]; do
        # Probe from the remote host loopback (the published host port), which is
        # what an external suite would hit too. Use curl on the remote.
        code="$($SSH "${TARGET}" "curl -s -o /dev/null -w '%{http_code}' --max-time 4 'http://127.0.0.1:${API_HOST_PORT}/readyz'" 2>/dev/null || echo 000)"
        if [ "${code}" = "200" ]; then
            body="$($SSH "${TARGET}" "curl -s --max-time 4 'http://127.0.0.1:${API_HOST_PORT}/readyz'" 2>/dev/null || true)"
            ready=1
            log "READY: /readyz -> 200 ${body}"
            break
        fi
        sleep 2
    done
    if [ "${ready}" -ne 1 ]; then
        log "ota-server never reached /readyz=200 (last code=${code}); container logs:"
        $SSH "${TARGET}" "podman logs ${PROJECT}_ota-server_1 2>&1 | tail -40" >&2 || true
        fail "real system not ready within ${READY_TIMEOUT}s"
    fi

    # The live base URL on stdout (the single contract for higher suites).
    printf 'BASE_URL=%s\n' "http://127.0.0.1:${API_HOST_PORT}"
    log "UP: complete. Live base URL (on ${REMOTE_HOST}): http://127.0.0.1:${API_HOST_PORT}"
    log "    higher suites reach it via the remote loopback (ssh) or ${base} if the port is exposed."
}

# Generate a base64 raw ed25519 public key (32 bytes) using openssl, matching
# what config.Load expects (base64.StdEncoding of the raw key). Falls back to a
# python3 generator. Prints the base64 string on stdout; non-zero on failure.
_gen_ed25519_pubkey_b64() {
    if command -v python3 >/dev/null 2>&1; then
        python3 - <<'PY' 2>/dev/null && return 0
import base64, os
from cryptography.hazmat.primitives.asymmetric.ed25519 import Ed25519PrivateKey
from cryptography.hazmat.primitives import serialization
try:
    k = Ed25519PrivateKey.generate().public_key()
    raw = k.public_bytes(serialization.Encoding.Raw, serialization.PublicFormat.Raw)
    print(base64.standard_b64encode(raw).decode())
except Exception:
    raise SystemExit(1)
PY
    fi
    # openssl fallback: 32 random bytes b64 (config tolerates any 32-byte value;
    # the upload path is not exercised by the boot smoke).
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 32 && return 0
    fi
    return 1
}

# Derive the base64 raw 32-byte ed25519 PUBLIC key from a caller-supplied PRIVATE
# PEM (caller-pubkey mode). The raw public key = last 32 bytes of the DER
# SubjectPublicKeyInfo (same extraction the signed-pipeline test uses). The
# private key is read ONLY locally to compute the public half; it is never
# transmitted to the remote (§11.4.10). Prints the base64 pubkey on stdout;
# non-zero on failure. Verifies the extracted key is exactly 32 bytes (a wrong
# length would silently break verification — §11.4.6 no guessing).
_pubkey_b64_from_priv_pem() {
    _pk_priv="$1"
    command -v openssl >/dev/null 2>&1 || return 1
    # base64-encode the raw key INSIDE the pipe (text, no NUL bytes) — capturing
    # raw DER bytes into a shell variable would strip NULs and corrupt the key.
    _pk_b64="$(openssl pkey -in "${_pk_priv}" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 | tr -d '\n')" || return 1
    # Validate the decoded key is exactly 32 bytes (§11.4.6 no guessing — a wrong
    # length would silently break verification).
    _pk_len="$(printf '%s' "${_pk_b64}" | base64 -d 2>/dev/null | wc -c | tr -d ' ')"
    [ "${_pk_len}" = "32" ] || return 1
    printf '%s' "${_pk_b64}"
}

# ==============================================================================
# dispatch
# ==============================================================================
ACTION="${1:---up}"
case "${ACTION}" in
    --up|up|"") do_up ;;
    --down|down) do_down ;;
    *) fail "unknown action '${ACTION}' (expected --up or --down)" ;;
esac
