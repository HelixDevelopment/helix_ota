#!/usr/bin/env bash
# =============================================================================
# guard_healthcheck_binary_present.sh — §11.4.135 regression guard
# -----------------------------------------------------------------------------
# Bug guarded: helixtrack-core compose healthcheck probe-binary absent from the
#   image (real finding 2026-06-22; Core Dockerfile fix d0f4bfb).
#   The compose healthcheck runs `curl -f http://localhost:8080/health`, but the
#   base image `golang:1.24-alpine` does NOT ship `curl`. The healthcheck binary
#   was therefore missing at runtime, so the container reported `(unhealthy)`
#   even though the app served `/health` with HTTP 200. FIX: the Core Dockerfile
#   now `apk add --no-cache sqlite-libs curl` so the probe binary exists.
#
# Invariant asserted (two coupled facts):
#   (1) compose-side  — containers/compose.helixtrack.yml's helixtrack-core
#       healthcheck `test:` command uses a KNOWN binary (`curl`). RED: a
#       healthcheck referencing a binary the image does not ship is the defect
#       class; the guard reproduces it with an inline `wget`-using fixture and
#       proves the probe-binary extractor rejects an unknown binary.
#   (2) Dockerfile-side (REAL cross-check, when sibling repo present) — the Core
#       Dockerfile installs that same binary via an `apk add` line.
#
# Honest boundary (§11.4.6):
#   * The compose-side check (1) is a SOURCE-ASSERTION on this repo's compose
#     file + a LOGIC-REPRODUCE (the unknown-binary RED fixture).
#   * The Dockerfile-side check (2) is a REAL cross-repo source-assertion, BUT
#     the Dockerfile lives in the SIBLING helix_track Core repo, NOT helix_ota.
#     The guard locates it at /Volumes/T7/Projects/helix_track/core/Application/
#     Dockerfile and, IFF present (`[ -f ]`), asserts it `apk add`s curl. If the
#     sibling repo is absent the cross-check SKIPs-with-reason (§11.4.3) and the
#     guard still GREENs on the compose-side invariant — never FAIL on absence.
#   * This guard does NOT boot a container; it asserts the compose+Dockerfile
#     contract that makes the healthcheck binary present. Live container-health
#     verification needs a real host (the distribute_stack.sh remote path).
#
# Usage: bash tests/regression/guard_healthcheck_binary_present.sh
# Exit 0 = guard GREEN (compose invariant held; Dockerfile cross-check PASS or SKIP).
# =============================================================================
set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE="${ROOT}/containers/compose.helixtrack.yml"
SIBLING_DOCKERFILE="/Volumes/T7/Projects/helix_track/core/Application/Dockerfile"

# Closed set of binaries known to be installed into the Core image (the probe
# binary MUST be one of these — else the healthcheck references something the
# image does not ship, the exact defect). `curl` is the fixed probe binary.
KNOWN_IMAGE_BINARIES="curl wget sh"

fail() { echo "  GUARD-FAIL: $*" >&2; exit 1; }
pass() { echo "  ok: $*"; }
skip() { echo "  SKIP-with-reason (§11.4.3): $*"; }

echo "[guard_healthcheck_binary_present]"

# --- probe_binary_of: extract the probe binary from a compose healthcheck `test:`
# line of the form  test: ["CMD", "curl", "-f", "http://..."]  (or CMD-SHELL).
# Echoes the binary token (the element AFTER "CMD"/"CMD-SHELL"); empty if none.
probe_binary_of() {
    printf '%s\n' "$1" \
        | sed -nE 's/.*\[[[:space:]]*"(CMD|CMD-SHELL)"[[:space:]]*,[[:space:]]*"([^"]+)".*/\2/p' \
        | head -1
}

# is_known: is $1 in the KNOWN_IMAGE_BINARIES set?
is_known() {
    local b="$1" k
    for k in $KNOWN_IMAGE_BINARIES; do [ "$b" = "$k" ] && return 0; done
    return 1
}

# --- §11.4.115 RED: a healthcheck using a NOT-installed binary FAILS the check -
# Inline fixture mimicking the broken state: probe binary absent from the image.
RED_FIXTURE='      test: ["CMD", "definitely-not-in-image", "-f", "http://localhost:8080/health"]'
red_bin="$(probe_binary_of "$RED_FIXTURE")"
[ "$red_bin" = "definitely-not-in-image" ] \
    || fail "RED harness broken: extracted probe binary '$red_bin' (expected 'definitely-not-in-image')."
if is_known "$red_bin"; then
    fail "RED broken: an unknown probe binary was treated as known — guard cannot catch a missing-binary healthcheck."
fi
echo "  RED-CONFIRMED: a healthcheck probe binary absent from the image FAILS the known-binary check (the bug)."

# --- §11.4.115 GREEN(polarity): the curl form PASSES the known-binary check ----
GREEN_FIXTURE='      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]'
green_bin="$(probe_binary_of "$GREEN_FIXTURE")"
is_known "$green_bin" \
    || fail "polarity broken: '$green_bin' not recognised as a known image binary — harness bug."
pass "a 'curl' healthcheck PASSES the known-binary check."

# --- GREEN (compose-side source-assertion): the REAL helixtrack-core healthcheck
[ -f "$COMPOSE" ] || fail "containers/compose.helixtrack.yml not found"

# Extract the helixtrack-core service block's healthcheck `test:` line. The
# service key is `helixtrack-core:`; the postgres service has its own (pg_isready)
# healthcheck, so we scope to the core service block only.
core_test_line="$(awk '
    /^[[:space:]]+helixtrack-core:[[:space:]]*$/ {incore=1; next}
    # leave the core block when a sibling service (same/lower indent, name:) starts
    incore && /^[[:space:]]{2}[a-zA-Z0-9_-]+:[[:space:]]*$/ {incore=0}
    incore && /[[:space:]]+test:[[:space:]]*\[/ {print; exit}
' "$COMPOSE")"
[ -n "$core_test_line" ] \
    || fail "helixtrack-core service has no healthcheck 'test:' line — healthcheck removed/changed."

core_bin="$(probe_binary_of "$core_test_line")"
[ -n "$core_bin" ] \
    || fail "could not extract probe binary from helixtrack-core healthcheck line: $core_test_line"
is_known "$core_bin" \
    || fail "helixtrack-core healthcheck uses probe binary '$core_bin' NOT in the known-installed set ($KNOWN_IMAGE_BINARIES) — the image may not ship it (unhealthy-despite-200 regression)."
[ "$core_bin" = "curl" ] \
    || fail "helixtrack-core healthcheck probe binary is '$core_bin', expected 'curl' (the fixed probe). If intentionally changed, update KNOWN_IMAGE_BINARIES AND the Dockerfile install AND this guard together (§11.4.120)."
pass "containers/compose.helixtrack.yml helixtrack-core healthcheck uses 'curl' (a known-installed binary)."

# --- Dockerfile-side REAL cross-check (sibling repo) — SKIP if absent ----------
if [ -f "$SIBLING_DOCKERFILE" ]; then
    # The probe binary MUST be installed via an `apk add` line (non-comment).
    if grep -E '^[[:space:]]*RUN[[:space:]]+apk[[:space:]]+add' "$SIBLING_DOCKERFILE" \
        | grep -vE '^[[:space:]]*#' \
        | grep -qwE "$core_bin"; then
        pass "sibling Core Dockerfile ($SIBLING_DOCKERFILE) installs '$core_bin' via 'apk add' (real cross-check)."
    else
        fail "sibling Core Dockerfile present but does NOT 'apk add' the probe binary '$core_bin' — healthcheck binary would be absent from the image (the original 'unhealthy-despite-200' defect)."
    fi
else
    skip "sibling Core Dockerfile not found at $SIBLING_DOCKERFILE — cross-repo curl-install check not run. Compose-side invariant held; the Dockerfile-side curl install lives in the helix_track Core repo and is asserted there when present."
fi

echo "GUARD-GREEN: healthcheck probe-binary present (compose curl + Dockerfile apk-add cross-check)"
exit 0
