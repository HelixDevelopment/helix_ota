#!/usr/bin/env bash
# =============================================================================
# guard_distribute_redeploy_idempotent.sh — §11.4.135 regression guard
# -----------------------------------------------------------------------------
# Bug guarded: distribute_stack.sh re-deploy idempotency (real finding 2026-06-22).
#   The OLD remote compose command was `<compose> build && <compose> up -d` with
#   NO `down` first. podman-compose `up` does NOT recreate an already-running
#   container ("name already in use"), so a SECOND deploy silently REUSED the
#   OLD image — the freshly-built image never went live. The FIX makes the
#   non-`down` branch emit `<compose> down 2>/dev/null; <compose> build &&
#   <compose> up -d` so the stale container is torn down BEFORE rebuild+up.
#   Named volumes (postgres data) persist across `down`, so this is data-safe.
#
# Strategy:
#   GREEN (source-assertion): the REAL build_compose_remote_cmd non-down branch
#     in scripts/distribute_stack.sh contains, in order, `down` BEFORE `build`
#     BEFORE `up -d` (ordered substring check on the actual function body,
#     comment lines excluded so prose cannot skew the order).
#   RED  (logic-reproduce):  the OLD `build && up -d`-only command string is fed
#     to the SAME ordered-substring check, which MUST FAIL (no `down`), and the
#     NEW `down; build && up -d` form MUST PASS — a §11.4.115 polarity proof the
#     check genuinely catches the defect's negation.
#
# Honest boundary (§11.4.6): this is a SOURCE-ASSERTION guard for the script's
#   emitted command + a LOGIC-REPRODUCE of the ordering check. The real remote
#   behaviour (podman-compose actually reusing a stale container) needs a live
#   host inside the ssh path and is NOT exercised here. We faithfully assert the
#   command the script emits AND prove the ordering check rejects the old form.
#
# Usage: bash tests/regression/guard_distribute_redeploy_idempotent.sh
# Exit 0 = guard GREEN.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DISTRIBUTE="${ROOT}/scripts/distribute_stack.sh"

fail() { echo "  GUARD-FAIL: $*" >&2; exit 1; }
pass() { echo "  ok: $*"; }

echo "[guard_distribute_redeploy_idempotent]"

# --- Ordered-substring predicate: does CMD contain down BEFORE build BEFORE up?
# Operates on a single-line command string. Returns 0 (down→build→up ordered),
# else non-zero. This is the logic under test for the §11.4.115 polarity proof.
down_before_build_before_up() {
    local cmd="$1"
    local down_pos build_pos up_pos
    # Pure-shell first-occurrence index (portable; no awk filename pitfalls).
    down_pos="$(_idx "$cmd" "down")"
    build_pos="$(_idx "$cmd" "build")"
    up_pos="$(_idx "$cmd" "up -d")"
    [ "$down_pos" -gt 0 ] || return 1
    [ "$build_pos" -gt 0 ] || return 1
    [ "$up_pos" -gt 0 ] || return 1
    [ "$down_pos" -lt "$build_pos" ] || return 1
    [ "$build_pos" -lt "$up_pos" ] || return 1
    return 0
}

# _idx HAYSTACK NEEDLE -> 1-based index of first occurrence, or 0 if absent.
_idx() {
    local hay="$1" needle="$2" pre
    case "$hay" in
        *"$needle"*) pre="${hay%%"$needle"*}"; printf '%s' "$(( ${#pre} + 1 ))" ;;
        *) printf '0' ;;
    esac
}

# --- §11.4.115 RED: the OLD command (no `down`) MUST FAIL the ordering check ---
OLD_CMD='cd /opt/x/containers && podman-compose -f compose.helixtrack.yml build && podman-compose -f compose.helixtrack.yml up -d'
if down_before_build_before_up "$OLD_CMD"; then
    fail "RED broken: OLD 'build && up -d'-only command PASSED the down→build→up check — guard cannot catch the defect."
fi
echo "  RED-CONFIRMED: OLD '<compose> build && up -d' (no down) FAILS the down→build→up ordering check (the bug)."

# --- §11.4.115 GREEN(polarity): the NEW command MUST PASS the ordering check ---
NEW_CMD='cd /opt/x/containers && podman-compose -f compose.helixtrack.yml down 2>/dev/null; podman-compose -f compose.helixtrack.yml build && podman-compose -f compose.helixtrack.yml up -d'
down_before_build_before_up "$NEW_CMD" \
    || fail "polarity broken: NEW 'down; build && up -d' command FAILED the ordering check — harness bug."
pass "NEW 'down; build && up -d' command PASSES the down→build→up ordering check."

# --- GREEN (source-assertion): the REAL script emits down BEFORE build BEFORE up
[ -f "$DISTRIBUTE" ] || fail "scripts/distribute_stack.sh not found"

# Extract the build_compose_remote_cmd function body, drop comment lines
# (leading-whitespace '#'), and keep only the non-`down`-action branch's
# printf — i.e. the line that actually emits the re-deploy command.
fn_body="$(awk '
    /^build_compose_remote_cmd\(\)[[:space:]]*\{/ {inf=1}
    inf {print}
    inf && /^\}/ {exit}
' "$DISTRIBUTE")"
[ -n "$fn_body" ] || fail "could not extract build_compose_remote_cmd() from distribute_stack.sh"

# The re-deploy printf is the one containing both `build` and `up -d`. Strip
# comment lines first so a prose mention cannot satisfy the check.
redeploy_line="$(printf '%s\n' "$fn_body" \
    | grep -vE '^[[:space:]]*#' \
    | grep -E 'build' | grep -E 'up -d' | head -1)"
[ -n "$redeploy_line" ] \
    || fail "build_compose_remote_cmd has no non-comment line emitting both 'build' and 'up -d' — re-deploy command missing/changed."

down_before_build_before_up "$redeploy_line" \
    || fail "build_compose_remote_cmd re-deploy command does NOT order 'down' before 'build' before 'up -d' — idempotency regression. Line: $redeploy_line"
pass "scripts/distribute_stack.sh build_compose_remote_cmd emits down → build → up -d (idempotent re-deploy)."

echo "GUARD-GREEN: distribute re-deploy idempotency (down-before-build-before-up)"
exit 0
