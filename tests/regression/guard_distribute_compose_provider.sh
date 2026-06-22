#!/usr/bin/env bash
# =============================================================================
# guard_distribute_compose_provider.sh — §11.4.135 regression guard
# -----------------------------------------------------------------------------
# Bug guarded: distribute_stack.sh compose-provider selection (commit 8e5db50b).
#   On hosts where `podman compose version` exits 0 but DELEGATES to a broken
#   docker-compose shim (e.g. thinker), the OLD probe trusted that exit 0 and
#   picked "podman compose" — which then failed the deploy with
#   "http+docker ... Not supported URL scheme". The FIX PREFERS the standalone
#   `podman-compose` binary whenever it is present, only falling back to the
#   `podman compose` plugin when podman-compose is absent.
#
# Strategy: reproduce the embedded probe's selection logic against a FAKE PATH
#   containing BOTH a `podman` whose `compose version` succeeds AND a real
#   `podman-compose`. The probe MUST return "podman-compose".
#     RED  : OLD ordering (plugin checked first) returns "podman compose".
#     GREEN: FIXED ordering (standalone checked first) returns "podman-compose".
#   Plus a static assertion that the real distribute_stack.sh orders the
#   `command -v podman-compose` check BEFORE `podman compose version`.
#
# Honest boundary (§11.4.6): the probe in distribute_stack.sh runs INSIDE an ssh
# heredoc on the remote host, so we cannot drive the literal remote invocation
# locally. We faithfully re-implement the selection branch (the bug locus) and
# anti-bluff it with the fake-PATH RED→GREEN + a static order check on the file.
#
# Usage: bash tests/regression/guard_distribute_compose_provider.sh
# Exit 0 = guard GREEN.
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
DISTRIBUTE="${ROOT}/scripts/distribute_stack.sh"

fail() { echo "  GUARD-FAIL: $*" >&2; exit 1; }
pass() { echo "  ok: $*"; }

echo "[guard_distribute_compose_provider]"

# --- Build a fake PATH with both providers present ---------------------------
FAKEBIN="$(mktemp -d "${TMPDIR:-/tmp}/guard_compose.XXXXXX")"
cleanup() { rm -rf "$FAKEBIN"; }
trap cleanup EXIT

# Fake `podman`: `podman compose version` exits 0 (simulating the broken-but-
# exit-0 plugin); anything else also exits 0.
cat > "$FAKEBIN/podman" <<'PODMAN'
#!/usr/bin/env bash
# `podman compose version` -> exit 0 (delegates to broken shim, but exit 0)
if [ "$1" = "compose" ] && [ "$2" = "version" ]; then exit 0; fi
exit 0
PODMAN
# Fake standalone `podman-compose`: present + works.
cat > "$FAKEBIN/podman-compose" <<'PC'
#!/usr/bin/env bash
echo "podman-compose 1.0"; exit 0
PC
chmod +x "$FAKEBIN/podman" "$FAKEBIN/podman-compose"

# Restrict PATH to the fakes (+ coreutils) so command -v resolves our binaries.
PROBE_PATH="$FAKEBIN:/usr/bin:/bin"

# Selection logic variants (mirroring the script's embedded branch). ----------
old_probe() {  # OLD ordering: plugin first — the BUG
    PATH="$PROBE_PATH" bash -c '
        if command -v podman >/dev/null 2>&1; then
            if podman compose version >/dev/null 2>&1; then echo "podman compose"
            elif command -v podman-compose >/dev/null 2>&1; then echo "podman-compose"
            else echo NO_COMPOSE; fi
        else echo NO_PODMAN; fi'
}
fixed_probe() {  # FIXED ordering: standalone first
    PATH="$PROBE_PATH" bash -c '
        if command -v podman >/dev/null 2>&1; then
            if command -v podman-compose >/dev/null 2>&1; then echo "podman-compose"
            elif podman compose version >/dev/null 2>&1; then echo "podman compose"
            else echo NO_COMPOSE; fi
        else echo NO_PODMAN; fi'
}

# --- RED: old ordering picks the broken plugin -------------------------------
old_res="$(old_probe)"
if [[ "$old_res" == "podman compose" ]]; then
    echo "  RED-CONFIRMED: OLD ordering returns '$old_res' (the broken plugin — the bug)."
else
    fail "OLD ordering returned '$old_res' (expected 'podman compose') — harness broken."
fi

# --- GREEN: fixed ordering prefers standalone podman-compose -----------------
fixed_res="$(fixed_probe)"
[[ "$fixed_res" == "podman-compose" ]] \
    || fail "FIXED ordering returned '$fixed_res' (expected 'podman-compose') — provider bug is BACK."
pass "FIXED ordering returns 'podman-compose' when both providers present."

# --- Static guard: real script checks podman-compose BEFORE the plugin -------
[[ -f "$DISTRIBUTE" ]] || fail "scripts/distribute_stack.sh not found"
# Exclude comment lines (leading-whitespace '#') so a prose mention of
# `podman compose version` in a comment cannot skew the order check.
standalone_ln="$(grep -nE 'command -v podman-compose' "$DISTRIBUTE" | grep -vE ':[[:space:]]*#' | head -1 | cut -d: -f1)"
plugin_ln="$(grep -nE 'podman compose version' "$DISTRIBUTE" | grep -vE ':[[:space:]]*#' | head -1 | cut -d: -f1)"
[[ -n "$standalone_ln" && -n "$plugin_ln" ]] \
    || fail "distribute_stack.sh missing podman-compose/podman-compose-version probe lines."
[[ "$standalone_ln" -lt "$plugin_ln" ]] \
    || fail "distribute_stack.sh checks 'podman compose version' (line $plugin_ln) BEFORE 'command -v podman-compose' (line $standalone_ln) — provider-preference regression."
pass "scripts/distribute_stack.sh checks podman-compose (L$standalone_ln) before plugin (L$plugin_ln)."

echo "GUARD-GREEN: distribute compose-provider preference"
exit 0
