#!/usr/bin/env bash
# =============================================================================
# security_probes_filters.sh — Helix OTA filter/pagination security probes
# -----------------------------------------------------------------------------
# Purpose:
#   Black-box (anti-bluff) security probes for the TWO just-shipped query-param
#   surfaces, asserting the new filters + pagination do NOT open authz /
#   ownership / injection / trust-boundary regressions:
#     - GET /api/v1/devices/{id}/telemetry  ?event ?since ?until ?limit ?cursor
#     - GET /api/v1/groups                  ?limit ?cursor
#     - GET /api/v1/groups/{id}/members     ?limit ?cursor
#   Every probe boots a REAL ota-server over real HTTP and HARD-asserts the
#   actual status code (Constitution §11.4 / §11.4.27 / §11.4.123). A genuine
#   defect FAILs — nothing is papered over. Mirrors security_probes.sh harness.
#
#   This is a SEPARATE script (not appended to security_probes.sh) so the
#   existing 37/0 tally is undisturbed (§11.4.84 working-tree quiescence spirit:
#   no edit to a known-green file).
#
# Probes (each HARD-asserted):
#   H. Ownership survives the new FILTERS — device A reading device B's
#        telemetry WITH ?event=success is still 403 (filter must not bypass
#        the ownership check, which runs BEFORE param parsing); device A reading
#        its OWN filtered telemetry is 200.
#   I. Closed-set ?event validation — unknown ?event=bogus -> 400 (NOT a silent
#        empty 200 that could mask an injection / typo'd enum).
#   J. Injection in cursor/since/event/limit values -> 400 validation, never
#        500 and never a leaked path / stack.
#   K. Pagination does not downgrade authz / leak cross-tenant data — a viewer
#        paginating /groups still only gets group views; ?limit cannot escalate
#        a device token onto a groups route (still 403).
#   L. limit out of range (0 / 999 / negative) -> 400 on every paginated route.
#
# Usage:
#   security_probes_filters.sh [--port N] [--external --base-url URL --password PW]
#   Env: HELIX_PORT (8097 here), HELIX_SECURITY_EVIDENCE (RUN_EVIDENCE_FILTERS.txt)
#
# Dependencies: bash, curl, jq, go (self-host mode).
#
# Cross-references:
#   server/internal/api/handlers_telemetry.go (ownership 403 + event/since/until
#     + limit/cursor validation),
#   server/internal/api/handlers_group.go (parsePage limit/cursor validation),
#   server/internal/api/server.go (route role wiring),
#   tests/security/security_probes.sh (sibling base suite — same harness).
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"

PORT="${HELIX_PORT:-8097}"
EXTERNAL=0
BASE_URL=""
ADMIN_USER="${HELIX_ADMIN_USERNAME:-admin@helix.test}"
ADMIN_PW="${HELIX_ADMIN_PASSWORD:-}"
EVIDENCE="${HELIX_SECURITY_EVIDENCE:-${SCRIPT_DIR}/RUN_EVIDENCE_FILTERS.txt}"
API="/api/v1"
RUN_TAG="secf-$(date +%s)-$$"

while [ $# -gt 0 ]; do
  case "$1" in
    --port)      PORT="$2"; shift 2 ;;
    --external)  EXTERNAL=1; shift ;;
    --base-url)  BASE_URL="$2"; shift 2 ;;
    --username)  ADMIN_USER="$2"; shift 2 ;;
    --password)  ADMIN_PW="$2"; shift 2 ;;
    -h|--help)   sed -n '2,55p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

[ -z "$BASE_URL" ] && BASE_URL="http://127.0.0.1:${PORT}"
BASE_URL="${BASE_URL%/}"

PASS=0; FAIL=0; SKIP=0
TOKEN=""; SERVER_PID=""
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-secf.XXXXXX")"

: > "$EVIDENCE"
log() { printf '%s\n' "$*" | tee -a "$EVIDENCE"; }
pass() { PASS=$((PASS+1)); log "[PASS] $1"; }
fail() { FAIL=$((FAIL+1)); log "[FAIL] $1"; }
skip() { SKIP=$((SKIP+1)); log "[SKIP] $1"; }

cleanup() {
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true; wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

HTTP_STATUS=""; HTTP_BODY=""
# req METHOD PATH [DATA] [TOKEN] [CONTENT_TYPE]
req() {
  local method="$1" path="$2" data="${3:-}" tok="${4:-}" ctype="${5:-application/json}"
  local tmp; tmp="$(mktemp)"
  # -g disables curl URL globbing so [ ] $ in query params are sent literally
  # (otherwise curl errors with "bad range specification" -> a script-bug 000,
  # which would be a §11.4.1 FAIL-bluff rather than a real server result).
  local -a args=(-gsS -o "$tmp" -w '%{http_code}' -X "$method" "${BASE_URL}${path}" -H 'Accept: application/json')
  [ -n "$tok" ] && args+=(-H "Authorization: Bearer ${tok}")
  [ -n "$data" ] && args+=(-H "Content-Type: ${ctype}" --data "$data")
  HTTP_STATUS="$(curl "${args[@]}" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$tmp")"; rm -f "$tmp"
}
jqget() { printf '%s' "$HTTP_BODY" | jq -r "$1" 2>/dev/null; }

# assert_in WANTLIST LABEL — pass if HTTP_STATUS is one of the space-sep WANTLIST.
assert_in() {
  local wantlist="$1" label="$2" got="$HTTP_STATUS" w
  for w in $wantlist; do
    if [ "$got" = "$w" ]; then pass "$label (HTTP $got)"; return 0; fi
  done
  fail "$label (want one of [$wantlist], got $got; body: $(printf '%s' "$HTTP_BODY" | head -c 220))"
  return 1
}
# assert_not5xx LABEL — capture the no-crash / no-leak invariant explicitly.
assert_not5xx() {
  local label="$1" got="$HTTP_STATUS"
  case "$got" in
    5*) fail "$label — server returned $got (crash/leak surface; body: $(printf '%s' "$HTTP_BODY" | head -c 220))"; return 1 ;;
    000) fail "$label — connection failed (got 000)"; return 1 ;;
  esac
  pass "$label (no 5xx; HTTP $got)"
}
# assert_no_leak LABEL NEEDLE — fail if the body echoes an injected token
# (e.g. a filesystem path / SQL fragment) — a leak/echo surface, §11.4.69.
assert_no_leak() {
  local label="$1" needle="$2"
  if printf '%s' "$HTTP_BODY" | grep -qF "$needle"; then
    fail "$label — response body echoed injected token '$needle' (leak surface; body: $(printf '%s' "$HTTP_BODY" | head -c 220))"
    return 1
  fi
  pass "$label (injected token not echoed in body)"
}

log "== Helix OTA filter/pagination security probe suite =="
log "base_url=${BASE_URL} run=${RUN_TAG} evidence=${EVIDENCE}"
log "started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log ""

for bin in curl jq; do command -v "$bin" >/dev/null 2>&1 || { log "ABORT: '$bin' not found"; exit 3; }; done

# ---- boot a self-hosted server (unless --external) ----------------------------
if [ "$EXTERNAL" = "0" ]; then
  command -v go >/dev/null 2>&1 || { log "ABORT: go not found (need it to self-host; or use --external)"; exit 3; }
  ADMIN_PW="selfhost-pw-${RUN_TAG}"
  SERVER_BIN="${WORK}/ota-server"
  log "building ota-server ..."
  ( cd "$SERVER_DIR" && go build -o "$SERVER_BIN" ./cmd/ota-server ) >>"$EVIDENCE" 2>&1 \
    || { log "ABORT: go build failed"; exit 3; }
  HELIX_PORT="$PORT" HELIX_ADMIN_USERNAME="$ADMIN_USER" HELIX_ADMIN_PASSWORD="$ADMIN_PW" \
  HELIX_TOKEN_SECRET="secf-token-secret-${RUN_TAG}" \
    "$SERVER_BIN" >"${WORK}/server.log" 2>&1 &
  SERVER_PID=$!
  READY=0
  for _ in $(seq 1 50); do
    kill -0 "$SERVER_PID" 2>/dev/null || break
    [ "$(curl -sS -o /dev/null -w '%{http_code}' "${BASE_URL}/healthz" 2>/dev/null || echo 000)" = "200" ] && { READY=1; break; }
    sleep 0.2
  done
  [ "$READY" = "1" ] || { log "ABORT: server not healthy"; tail -n 30 "${WORK}/server.log" | tee -a "$EVIDENCE"; exit 1; }
  pass "self-hosted ota-server healthy on ${BASE_URL}"
else
  [ -n "$ADMIN_PW" ] || { log "ABORT: --external requires --password / HELIX_ADMIN_PASSWORD"; exit 3; }
  req GET "/healthz"; [ "$HTTP_STATUS" = "200" ] || { log "ABORT: external server not reachable"; exit 1; }
  pass "external ota-server reachable on ${BASE_URL}"
fi

# ---- obtain an admin token (used to provision fixtures, not the probe subject) --
req POST "${API}/auth/login" "$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PW" '{username:$u,password:$p}')" ""
[ "$HTTP_STATUS" = "200" ] || { log "ABORT: admin login failed (HTTP $HTTP_STATUS)"; exit 1; }
TOKEN="$(jqget '.access_token')"
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { log "ABORT: no admin token"; exit 1; }
pass "admin login established (fixtures only)"

# Provision TWO devices so we have two device-scoped tokens for ownership tests.
reg_device() {
  local hw="$1"
  req POST "${API}/devices/register" "$(jq -nc --arg hw "$hw" \
    '{hardware_id:$hw,model:"OrangePi5Max",os:"android",current_version:"1.0.0"}')" "$TOKEN"
}
reg_device "secfdev-A-${RUN_TAG}"
DEV_A_ID="$(jqget '.device_id')"; DEV_A_TOK="$(jqget '.device_token')"
reg_device "secfdev-B-${RUN_TAG}"
DEV_B_ID="$(jqget '.device_id')"; DEV_B_TOK="$(jqget '.device_token')"
if [ -z "$DEV_A_TOK" ] || [ "$DEV_A_TOK" = "null" ] || [ -z "$DEV_B_TOK" ] || [ "$DEV_B_TOK" = "null" ]; then
  log "ABORT: could not provision two device tokens"; exit 1
fi
log "      device A=${DEV_A_ID}  device B=${DEV_B_ID}"

# A viewer-capable token for the "listing returns only group data" probe. The
# self-hosted build seeds exactly ONE static user (admin, carrying the viewer
# role — cmd/ota-server/main.go) and exposes NO runtime user-creation API, so
# there is no viewer-ONLY identity to mint here. The admin token IS the lowest-
# privilege viewer-capable identity available on this build; K1 uses it to prove
# the pagination param yields a group-shaped body (no authz downgrade / leak).
# The role-ESCALATION angle (a token cannot gain access it lacks via a param) is
# proven independently by K2/K3 with a device token. When run --external against
# a directory that has a viewer-only user, set HELIX_VIEWER_USERNAME /
# HELIX_VIEWER_PASSWORD to exercise K1 with a true viewer.
VIEWER_TOK=""
V_USER="${HELIX_VIEWER_USERNAME:-}"; V_PW="${HELIX_VIEWER_PASSWORD:-}"
if [ -n "$V_USER" ] && [ -n "$V_PW" ]; then
  req POST "${API}/auth/login" "$(jq -nc --arg u "$V_USER" --arg p "$V_PW" '{username:$u,password:$p}')" ""
  [ "$HTTP_STATUS" = "200" ] && VIEWER_TOK="$(jqget '.access_token')"
fi
# Fall back to the viewer-capable admin token so K1 always runs (never a bluff SKIP).
if [ -z "$VIEWER_TOK" ] || [ "$VIEWER_TOK" = "null" ]; then VIEWER_TOK="$TOKEN"; fi
log ""

# =============================================================================
# H. OWNERSHIP SURVIVES THE NEW FILTERS — a ?event= must NOT bypass the 403.
# =============================================================================
log "--- H. ownership: telemetry filters must not bypass the per-device 403 ---"
# H1: device A reading device B's telemetry WITH a filter is STILL 403.
req GET "${API}/devices/${DEV_B_ID}/telemetry?event=success" "" "$DEV_A_TOK"
assert_in "403" "H1 device A GET device B telemetry ?event=success -> 403 (filter does not bypass ownership)"
# H2: a since/until window also cannot bypass ownership.
req GET "${API}/devices/${DEV_B_ID}/telemetry?since=2020-01-01T00:00:00Z&until=2030-01-01T00:00:00Z&limit=5" "" "$DEV_A_TOK"
assert_in "403" "H2 device A GET device B telemetry ?since&until&limit -> 403 (window does not bypass ownership)"
# H3: control — device A reading its OWN filtered telemetry is 200 (filter works
#     for the legitimate owner; proves H1/H2 are ownership, not a blanket reject).
req GET "${API}/devices/${DEV_A_ID}/telemetry?event=success&limit=10" "" "$DEV_A_TOK"
assert_in "200" "H3 device A GET its OWN telemetry ?event=success&limit=10 -> 200 (filter allowed for owner)"
# H4: the ownership check must precede param parsing — an INVALID filter on
#     ANOTHER device's telemetry is still 403 (not 400), i.e. no information
#     leak about validity before the authz decision.
req GET "${API}/devices/${DEV_B_ID}/telemetry?event=bogus" "" "$DEV_A_TOK"
assert_in "403" "H4 device A GET device B telemetry ?event=bogus -> 403 (authz precedes validation; no leak)"

# =============================================================================
# I. CLOSED-SET ?event VALIDATION — unknown -> 400, never silent-empty 200.
# =============================================================================
log ""
log "--- I. ?event closed-set validation: unknown event must be 400, not empty 200 ---"
# I1: owner with an unknown ?event must get 400 (silently-empty 200 would mask a
#     typo'd enum / probing for a hidden value — §11.4.6 no-guessing).
req GET "${API}/devices/${DEV_A_ID}/telemetry?event=bogus" "" "$DEV_A_TOK"
assert_in "400" "I1 owner GET telemetry ?event=bogus -> 400 (closed set, not silent-empty 200)"
assert_not5xx "I1b unknown ?event did not crash"
# I2: a known event value IS accepted (200) — proves I1 is validation, not a
#     blanket reject of the ?event param.
req GET "${API}/devices/${DEV_A_ID}/telemetry?event=installing" "" "$DEV_A_TOK"
assert_in "200" "I2 owner GET telemetry ?event=installing (known enum) -> 200"

# =============================================================================
# J. INJECTION in cursor/since/event/limit values -> 400 validation, never 5xx.
# =============================================================================
log ""
log "--- J. injection in filter/pagination values must be 400, never 5xx/leak ---"
# J1: path traversal in ?cursor (owner, so authz passes -> param validation runs).
req GET "${API}/devices/${DEV_A_ID}/telemetry?cursor=..%2F..%2Fetc%2Fpasswd" "" "$DEV_A_TOK"
assert_in "400" "J1 ?cursor=../../etc/passwd -> 400 (non-integer cursor rejected)"
assert_not5xx "J1b traversal cursor did not crash"
assert_no_leak "J1c traversal cursor not echoed" "/etc/passwd"
# J2: SQL-ish in ?since (must fail RFC3339 parse -> 400, never reach a store query).
req GET "${API}/devices/${DEV_A_ID}/telemetry?since=%27%3B%20DROP%20TABLE%20telemetry%3B--" "" "$DEV_A_TOK"
assert_in "400" "J2 ?since='; DROP TABLE -> 400 (not RFC3339)"
assert_not5xx "J2b SQL-ish since did not crash"
assert_no_leak "J2c SQL-ish since not echoed" "DROP TABLE"
# J3: XSS-ish in ?event (closed set -> 400; stored/echoed nowhere).
req GET "${API}/devices/${DEV_A_ID}/telemetry?event=%3Cscript%3Ealert(1)%3C%2Fscript%3E" "" "$DEV_A_TOK"
assert_in "400" "J3 ?event=<script>alert(1)</script> -> 400 (closed set)"
assert_not5xx "J3b XSS-ish event did not crash"
assert_no_leak "J3c XSS-ish event not reflected" "<script>"
# J4: injection in ?cursor on the GROUPS route (parsePage path) -> 400, no 5xx.
req GET "${API}/groups?cursor=..%2F..%2Fetc" "" "$TOKEN"
assert_in "400" "J4 GET /groups ?cursor=../../etc -> 400 (parsePage rejects non-int)"
assert_not5xx "J4b groups traversal cursor did not crash"
assert_no_leak "J4c groups traversal cursor not echoed" "../../etc"
# J5: injection in ?limit on the GROUPS route -> 400, no 5xx.
req GET "${API}/groups?limit=1%3BSELECT" "" "$TOKEN"
assert_in "400" "J5 GET /groups ?limit=1;SELECT -> 400 (non-int limit rejected)"
assert_not5xx "J5b groups injection limit did not crash"

# =============================================================================
# K. PAGINATION must not downgrade authz / leak cross-tenant data.
# =============================================================================
log ""
log "--- K. pagination params cannot escalate authz or leak cross-tenant data ---"
# K1: a viewer-capable identity paginating /groups only ever sees group views
#     (no authz downgrade via ?limit) — the body is a {items:[...],next_cursor}
#     group list, nothing device/telemetry-shaped.
req GET "${API}/groups?limit=1" "" "$VIEWER_TOK"
assert_in "200" "K1 viewer-capable GET /groups ?limit=1 -> 200 (may list)"
if printf '%s' "$HTTP_BODY" | jq -e 'has("items") and has("next_cursor")' >/dev/null 2>&1; then
  pass "K1b /groups ?limit body is a group list (items/next_cursor) — no authz downgrade / cross-tenant leak via ?limit"
else
  fail "K1b /groups ?limit body not a group-list shape (body: $(printf '%s' "$HTTP_BODY" | head -c 220))"
fi
# K2: a DEVICE token (lowest role) cannot reach /groups even WITH a pagination
#     param — ?limit must not be a backdoor onto a route the role can't access.
req GET "${API}/groups?limit=5&cursor=0" "" "$DEV_A_TOK"
assert_in "403" "K2 device token GET /groups ?limit=5&cursor=0 -> 403 (param does not escalate role)"
# K3: device token cannot reach /groups/{id}/members via a pagination param either.
req GET "${API}/groups/any-group/members?limit=5" "" "$DEV_A_TOK"
assert_in "403" "K3 device token GET /groups/*/members ?limit=5 -> 403 (param does not escalate role)"
# K4: unauthenticated paginated /groups is still 401 (pagination is not a public
#     read backdoor).
req GET "${API}/groups?limit=10" "" ""
assert_in "401" "K4 unauth GET /groups ?limit=10 -> 401 (pagination is not a public backdoor)"

# =============================================================================
# L. limit OUT OF RANGE (0 / 999 / negative) -> 400 on every paginated route.
# =============================================================================
log ""
log "--- L. limit out of range must be 400 on every paginated route ---"
# Telemetry route ([1,200]).
req GET "${API}/devices/${DEV_A_ID}/telemetry?limit=0" "" "$DEV_A_TOK"
assert_in "400" "L1 telemetry ?limit=0 -> 400 (below range)"
req GET "${API}/devices/${DEV_A_ID}/telemetry?limit=999" "" "$DEV_A_TOK"
assert_in "400" "L2 telemetry ?limit=999 -> 400 (above range)"
req GET "${API}/devices/${DEV_A_ID}/telemetry?limit=-5" "" "$DEV_A_TOK"
assert_in "400" "L3 telemetry ?limit=-5 -> 400 (negative)"
# Groups route ([1,200]).
req GET "${API}/groups?limit=0" "" "$TOKEN"
assert_in "400" "L4 /groups ?limit=0 -> 400 (below range)"
req GET "${API}/groups?limit=999" "" "$TOKEN"
assert_in "400" "L5 /groups ?limit=999 -> 400 (above range)"
req GET "${API}/groups?limit=-5" "" "$TOKEN"
assert_in "400" "L6 /groups ?limit=-5 -> 400 (negative)"
# Control: an IN-RANGE limit is accepted (200) — proves L1-L6 are range checks,
# not a blanket reject of the ?limit param.
req GET "${API}/groups?limit=200" "" "$TOKEN"
assert_in "200" "L7 /groups ?limit=200 (max in-range) -> 200 (boundary accepted)"
req GET "${API}/devices/${DEV_A_ID}/telemetry?limit=1" "" "$DEV_A_TOK"
assert_in "200" "L8 telemetry ?limit=1 (min in-range) -> 200 (boundary accepted)"
# Members route validation needs a real group id — create one as admin (fixture).
req POST "${API}/groups" "$(jq -nc --arg n "secf-page-${RUN_TAG}" '{name:$n}')" "$TOKEN"
PAGE_GID="$(jqget '.group_id')"
if [ -n "$PAGE_GID" ] && [ "$PAGE_GID" != "null" ]; then
  req GET "${API}/groups/${PAGE_GID}/members?limit=0" "" "$TOKEN"
  assert_in "400" "L9 /groups/{id}/members ?limit=0 -> 400 (below range)"
  req GET "${API}/groups/${PAGE_GID}/members?limit=999" "" "$TOKEN"
  assert_in "400" "L10 /groups/{id}/members ?limit=999 -> 400 (above range)"
  req GET "${API}/groups/${PAGE_GID}/members?limit=10" "" "$TOKEN"
  assert_in "200" "L11 /groups/{id}/members ?limit=10 (in-range) -> 200 (boundary accepted)"
  req DELETE "${API}/groups/${PAGE_GID}" "" "$TOKEN" >/dev/null 2>&1 || true
else
  skip "L9-L11 members range checks (could not provision a fixture group)"
fi

# ---- summary -------------------------------------------------------------------
log ""
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then log "RESULT: FAIL"; exit 1; fi
log "RESULT: PASS"
exit 0
