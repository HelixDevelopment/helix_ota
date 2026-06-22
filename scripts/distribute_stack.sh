#!/usr/bin/env bash
# =============================================================================
# distribute_stack.sh — remote rootless-podman container distribution
# -----------------------------------------------------------------------------
# Purpose:
#   Deploy the helix-layer HelixTrack container stack onto a remote
#   distribution host over SSH using **rootless** `podman compose`
#   (§11.4.161). The mechanism is: probe SSH + rootless podman presence →
#   rsync a self-contained deploy bundle → run `podman compose up -d` ON the
#   remote → remote health-check → capture real output.
#
#   This is an EXPLICIT, on-demand action. It is NOT fired implicitly on
#   every commit. The per-commit HelixTrack sync path
#   (scripts/commit_all.sh::_helixtrack_ensure_running and
#   scripts/git_hooks/pre-commit) stays lightweight: it only needs a
#   reachable HelixTrack endpoint for the workable-items sync. It invokes a
#   heavy remote deploy ONLY when HELIX_DISTRIBUTE_ON_COMMIT=1 is set
#   (default OFF), so routine commits never trigger a remote deploy.
#
# Usage:
#   bash scripts/distribute_stack.sh [--dry-run] [--host HOST] [--user USER]
#                                    [--no-build-context] [--down]
#
#   --dry-run          Probe reachability + podman presence, print the rsync
#                      and compose commands that WOULD run, do NOT deploy.
#   --host HOST        Single host (overrides HELIXTRACK_REMOTE_HOST list).
#   --user USER        SSH user (overrides HELIXTRACK_REMOTE_USER).
#   --no-build-context Skip rsyncing the build context (assume the remote
#                      already has it / a prebuilt image).
#   --down             `podman compose down` on the remote instead of `up`.
#
# Inputs (env, all optional):
#   HELIXTRACK_REMOTE_HOST   Space-separated candidate hosts.
#                            Default: "thinker.local amber.local"
#   HELIXTRACK_REMOTE_USER   SSH user. Default: "milosvasic"
#   HELIX_DIST_REMOTE_DIR    Remote bundle dir. Default: "~/helix-dist"
#   HELIX_TRACK_SRC          Build-context source (sibling helix_track repo).
#                            Default: autodetect /Volumes/T7/Projects/helix_track
#                            then $PROJECT_ROOT/helix_track.
#
# Outputs:
#   Real deploy / probe output to stdout. Exit 0 on success or honest SKIP,
#   non-zero only on a genuine deploy failure against a podman-ready host.
#
# Side-effects:
#   On a real (non-dry-run) deploy: rsyncs a bundle to the remote host and
#   starts containers there. Honest SKIP (exit 0) when no host has rootless
#   podman — never a bluff PASS (§11.4.6).
#
# Dependencies:
#   ssh, rsync (host); rootless podman + (`podman compose` | `podman-compose`)
#   on the remote (§11.4.161 — NO sudo, NO rootful docker on the remote).
#
# Cross-references:
#   §11.4.161 rootless container runtime · §11.4.76 containers submodule ·
#   §11.4.28 helix-layer only (no project context inside the submodule) ·
#   §11.4.6 no-bluff honest SKIP · §11.4.18 companion doc
#   (docs/scripts/distribute_stack.md).
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# ---- args -------------------------------------------------------------------
DRY_RUN=0
SINGLE_HOST=""
ARG_USER=""
SKIP_BUILD_CTX=0
COMPOSE_ACTION="up"
while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run)          DRY_RUN=1 ;;
        --host)             SINGLE_HOST="${2:-}"; shift ;;
        --user)             ARG_USER="${2:-}"; shift ;;
        --no-build-context) SKIP_BUILD_CTX=1 ;;
        --down)             COMPOSE_ACTION="down" ;;
        -h|--help)          sed -n '2,40p' "$0"; exit 0 ;;
        *)                  echo "[DIST] unknown arg: $1" >&2; exit 2 ;;
    esac
    shift
done

DIST_USER="${ARG_USER:-${HELIXTRACK_REMOTE_USER:-milosvasic}}"
if [ -n "$SINGLE_HOST" ]; then
    DIST_CANDIDATES="$SINGLE_HOST"
else
    DIST_CANDIDATES="${HELIXTRACK_REMOTE_HOST:-thinker.local amber.local}"
fi
REMOTE_DIR="${HELIX_DIST_REMOTE_DIR:-~/helix-dist}"

# Blast-radius guard: rsync --delete into a dangerous remote dir (empty, root,
# bare $HOME, or bare ~) could wipe an entire home directory. Refuse honestly.
case "$REMOTE_DIR" in
    ""|"/"|"~"|'$HOME'|"$HOME")
        echo "[DIST] FATAL: REMOTE_DIR='$REMOTE_DIR' is unsafe (empty, /, ~, or \$HOME)." >&2
        echo "[DIST]        A --delete rsync into it would destroy the home directory. Aborting (§11.4.6/§9)." >&2
        exit 1 ;;
esac

COMPOSE_FILE="$PROJECT_ROOT/containers/compose.helixtrack.yml"

# Build context (the compose `helixtrack-core` service builds from
# ../helix_track/core/Application). In this checkout that path may be empty,
# so resolve the sibling helix_track repo as the real build context.
HT_SRC="${HELIX_TRACK_SRC:-}"
if [ -z "$HT_SRC" ]; then
    if [ -f "/Volumes/T7/Projects/helix_track/core/Application/Dockerfile" ]; then
        HT_SRC="/Volumes/T7/Projects/helix_track"
    elif [ -f "$PROJECT_ROOT/helix_track/core/Application/Dockerfile" ]; then
        HT_SRC="$PROJECT_ROOT/helix_track"
    fi
fi
# Spaces (per-project volume data) live in THIS project tree.
SPACES_SRC="$PROJECT_ROOT/helix_track/spaces"

log() { echo "[DIST] $*"; }

# ---- remote podman-presence probe ------------------------------------------
# Echoes "podman compose" or "podman-compose" on success; empty on absence.
probe_remote_compose() {
    local host="$1"
    ssh -o ConnectTimeout=6 -o BatchMode=yes "${DIST_USER}@${host}" '
        # Prefer rootless podman (§11.4.161). Within podman, PREFER the
        # standalone `podman-compose` over the `podman compose` plugin: on some
        # hosts (e.g. thinker) `podman compose version` exits 0 but DELEGATES to
        # a broken docker-compose shim that needs a Docker daemon (none under
        # rootless), failing the deploy with "http+docker ... Not supported URL
        # scheme". The standalone podman-compose is the reliable rootless tool.
        if command -v podman >/dev/null 2>&1; then
            if command -v podman-compose >/dev/null 2>&1; then echo "podman-compose"
            elif podman compose version >/dev/null 2>&1; then echo "podman compose"
            else echo NO_COMPOSE; fi
            exit 0
        fi
        # §11.4.161 documented exception (operator-authorized 2026-06-22, "use docker
        # if available"): a host without rootless podman (e.g. amber.local) may use
        # docker where docker is explicitly present. podman remains preferred above.
        if command -v docker >/dev/null 2>&1; then
            if docker compose version >/dev/null 2>&1; then echo "docker compose"
            elif command -v docker-compose >/dev/null 2>&1; then echo "docker-compose"
            else echo NO_COMPOSE; fi
            exit 0
        fi
        echo NO_PODMAN
    ' 2>/dev/null || echo "SSH_FAIL"
}

# ---- pick first reachable + podman-ready host ------------------------------
DIST_HOST=""
COMPOSE_CMD=""
for cand in $DIST_CANDIDATES; do
    log "Probing ${DIST_USER}@${cand} ..."
    if ! ssh -o ConnectTimeout=6 -o BatchMode=yes "${DIST_USER}@${cand}" 'echo ok' 2>/dev/null | grep -q ok; then
        log "  SSH unreachable — skipping $cand (honest skip, §11.4.6)"
        continue
    fi
    res="$(probe_remote_compose "$cand")"
    case "$res" in
        "podman compose"|"podman-compose")
            DIST_HOST="$cand"; COMPOSE_CMD="$res"
            log "  reachable + rootless podman ($res) — selected $cand"
            break ;;
        "docker compose"|"docker-compose")
            DIST_HOST="$cand"; COMPOSE_CMD="$res"
            log "  reachable + docker ($res) — selected $cand (§11.4.161 operator-authorized docker exception, e.g. amber.local)"
            break ;;
        NO_PODMAN)
            log "  SSH ok but no podman OR docker — skipping $cand honestly (install rootless podman, or docker where authorized; §11.4.161/§11.4.6)" ;;
        NO_COMPOSE)
            log "  podman present but no compose front-end — skipping $cand honestly" ;;
        *)
            log "  probe inconclusive ($res) — skipping $cand" ;;
    esac
done

if [ -z "$DIST_HOST" ]; then
    log "No reachable host has rootless podman ($DIST_CANDIDATES) — SKIP (no deploy, no bluff)."
    exit 0
fi

# ---- validate local assets --------------------------------------------------
if [ ! -f "$COMPOSE_FILE" ]; then
    log "compose file missing: $COMPOSE_FILE — cannot distribute. SKIP."
    exit 0
fi
if [ "$SKIP_BUILD_CTX" -eq 0 ] && [ -z "$HT_SRC" ]; then
    log "build context (helix_track/core/Application/Dockerfile) not found locally."
    log "  The compose 'helixtrack-core' service builds from it; without it the remote"
    log "  build would fail. Re-run with --no-build-context if the remote already has"
    log "  the context/image, or set HELIX_TRACK_SRC. SKIP (honest, §11.4.6)."
    exit 0
fi

# Bundle layout on the remote preserves the compose's relative paths:
#   $REMOTE_DIR/containers/compose.helixtrack.yml
#   $REMOTE_DIR/helix_track/core/Application/   (build context)
#   $REMOTE_DIR/helix_track/spaces/             (volume data)
RSYNC_OPTS="-az --delete --exclude='.git' --exclude='node_modules' --exclude='out/'"
# ADDITIVE opts for the spaces/ (per-project VOLUME DATA) sync: NO --delete, so
# an empty/partial local spaces/ never destroys remote volume data (§11.4.6/§9).
RSYNC_OPTS_NODELETE="${RSYNC_OPTS/--delete /}"

build_compose_remote_cmd() {
    if [ "$COMPOSE_ACTION" = "down" ]; then
        printf 'cd %s/containers && %s -f compose.helixtrack.yml down' "$REMOTE_DIR" "$COMPOSE_CMD"
    else
        # build BEFORE up: podman-compose 1.0.6 does NOT auto-build on `up`, so
        # without an explicit build it tries to PULL the locally-built image and
        # fails (real deploy 2026-06-22). docker compose auto-builds, but the
        # explicit build is harmless there and correct for podman-compose.
        printf 'cd %s/containers && %s -f compose.helixtrack.yml build && %s -f compose.helixtrack.yml up -d' "$REMOTE_DIR" "$COMPOSE_CMD" "$COMPOSE_CMD"
    fi
}

print_plan() {
    echo "  SSH target      : ${DIST_USER}@${DIST_HOST}"
    echo "  Remote compose  : $COMPOSE_CMD (rootless, §11.4.161)"
    echo "  Remote dir      : $REMOTE_DIR"
    echo "  --- rsync steps (would run) ---"
    echo "  ssh ${DIST_USER}@${DIST_HOST} 'mkdir -p $REMOTE_DIR/containers $REMOTE_DIR/helix_track/core/Application $REMOTE_DIR/helix_track/spaces'"
    echo "  rsync $RSYNC_OPTS $COMPOSE_FILE  ${DIST_USER}@${DIST_HOST}:$REMOTE_DIR/containers/"
    if [ "$SKIP_BUILD_CTX" -eq 0 ]; then
        echo "  rsync $RSYNC_OPTS $HT_SRC/core/Application/  ${DIST_USER}@${DIST_HOST}:$REMOTE_DIR/helix_track/core/Application/"
    else
        echo "  (build context rsync skipped: --no-build-context)"
    fi
    if [ -d "$SPACES_SRC" ]; then
        echo "  rsync $RSYNC_OPTS_NODELETE $SPACES_SRC/  ${DIST_USER}@${DIST_HOST}:$REMOTE_DIR/helix_track/spaces/  (additive — no --delete on volume data)"
    else
        echo "  (spaces dir absent: $SPACES_SRC — skipped)"
    fi
    echo "  --- remote compose (would run) ---"
    echo "  ssh ${DIST_USER}@${DIST_HOST} '$(build_compose_remote_cmd)'"
    echo "  --- remote health-check (would run) ---"
    echo "  ssh ${DIST_USER}@${DIST_HOST} 'curl -sf http://localhost:8080 >/dev/null && echo HEALTHY || echo UNHEALTHY'"
}

if [ "$DRY_RUN" -eq 1 ]; then
    log "DRY-RUN — no deploy will occur. Planned actions:"
    print_plan
    log "DRY-RUN complete (probe + plan only)."
    exit 0
fi

# ---- real deploy ------------------------------------------------------------
log "Deploying to ${DIST_USER}@${DIST_HOST} (rootless $COMPOSE_CMD)..."
# Pre-create the FULL nested dest paths — rsync does not auto-create deep dest
# dirs, so without these the core/Application + spaces rsyncs fail with
# `mkdir "..." failed: No such file or directory`.
ssh -o BatchMode=yes "${DIST_USER}@${DIST_HOST}" "mkdir -p $REMOTE_DIR/containers $REMOTE_DIR/helix_track/core/Application $REMOTE_DIR/helix_track/spaces"

# shellcheck disable=SC2086
eval rsync $RSYNC_OPTS "\"$COMPOSE_FILE\"" "\"${DIST_USER}@${DIST_HOST}:$REMOTE_DIR/containers/\""
if [ "$SKIP_BUILD_CTX" -eq 0 ]; then
    # shellcheck disable=SC2086
    eval rsync $RSYNC_OPTS "\"$HT_SRC/core/Application/\"" "\"${DIST_USER}@${DIST_HOST}:$REMOTE_DIR/helix_track/core/Application/\""
fi
if [ -d "$SPACES_SRC" ]; then
    # ADDITIVE sync (RSYNC_OPTS_NODELETE — NO --delete): spaces/ is per-project
    # VOLUME DATA; an empty/partial local spaces/ must NEVER wipe remote data.
    # shellcheck disable=SC2086
    eval rsync $RSYNC_OPTS_NODELETE "\"$SPACES_SRC/\"" "\"${DIST_USER}@${DIST_HOST}:$REMOTE_DIR/helix_track/spaces/\""
fi

log "Running remote rootless compose..."
ssh -o BatchMode=yes "${DIST_USER}@${DIST_HOST}" "$(build_compose_remote_cmd)"

if [ "$COMPOSE_ACTION" = "down" ]; then
    log "Stack stopped on $DIST_HOST."
    exit 0
fi

log "Remote health-check (up to 60s)..."
healthy=0
for i in $(seq 1 12); do
    if ssh -o BatchMode=yes "${DIST_USER}@${DIST_HOST}" 'curl -sf http://localhost:8080 >/dev/null 2>&1'; then
        log "Stack HEALTHY on $DIST_HOST after $((i*5))s."
        healthy=1
        break
    fi
    sleep 5
done
ssh -o BatchMode=yes "${DIST_USER}@${DIST_HOST}" "cd $REMOTE_DIR/containers && $COMPOSE_CMD -f compose.helixtrack.yml ps" 2>/dev/null || true
if [ "$healthy" -ne 1 ]; then
    log "Stack deployed but health-check did NOT pass in 60s on $DIST_HOST (genuine failure, not skipped)."
    exit 1
fi
exit 0
