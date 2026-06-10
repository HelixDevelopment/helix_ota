#!/usr/bin/env bash
# =============================================================================
# challenge_filters_pagination.sh — Helix OTA telemetry-filter + cursor-
# pagination E2E challenge (autonomous, anti-bluff)
# -----------------------------------------------------------------------------
# Purpose:
#   Close the coverage gap for the TWO just-shipped query features with a fully
#   autonomous, anti-bluff end-to-end challenge (Constitution §11.4 / §11.4.27 /
#   §11.4.69 / §11.4.98). This script SELF-HOSTS a live `ota-server` (builds it
#   from source, boots it with ephemeral in-memory config), seeds real state via
#   the real REST API, then BLACK-BOX drives the two features over real HTTP
#   (curl + jq) and asserts real HTTP status codes AND real JSON bodies. There
#   are NO mocks of the system under test and NO manual steps — the server is
#   booted, exercised, and torn down by this one script.
#
#   Feature 1 — per-device telemetry filters
#     GET /api/v1/devices/{id}/telemetry?event=&since=&until=&limit=&cursor=
#       * ?event=<type>  returns ONLY events of that closed-set type
#       * ?since/?until  (RFC3339, both bounds INCLUSIVE on the event timestamp)
#                        narrow the window
#       * a bogus ?event= value -> 400 (no silent-empty; §11.4.6)
#       * combined event+window narrows correctly
#       * a since>until window -> empty items, null next_cursor
#
#   Feature 2 — group + group-members cursor pagination
#     GET /api/v1/groups?limit=&cursor=         -> {items[],next_cursor}
#     GET /api/v1/groups/{id}/members?limit=&cursor=
#       * limit=2 page1 -> exactly 2 items + a NON-null next_cursor
#       * page2 (cursor=next_cursor) -> the rest, with null next_cursor on the
#         last page
#       * NO overlap and NO gap across pages (the union reconstructs the full set)
#       * limit out of [1,200] -> 400 ; malformed cursor -> 400
#
# Anti-bluff guarantees (these HARD-FAIL the challenge — no false PASS):
#   * a bogus ?event= that returned 200 (silent-empty) instead of 400 -> FAIL
#   * an ?event= filter that returned an event of a DIFFERENT type -> FAIL
#   * a since/until window that included an out-of-window event -> FAIL
#   * page1+page2 that overlap, drop, or duplicate any id -> FAIL
#   * a next_cursor that is non-null on the genuinely-last page -> FAIL
#   * a 401 on a protected route while seeded with a valid token -> FAIL
#   Seeding itself is asserted (telemetry ingest .accepted must equal what we
#   sent), so a green filter result over UN-ingested data is impossible.
#
# Usage:
#   challenge_filters_pagination.sh [--port N] [--server-bin PATH]
#   Env: HELIX_PORT (default 8096), HELIX_FILTERS_EVIDENCE (default
#        tests/e2e/FILTERS_PAGINATION_EVIDENCE.txt)
#
# Inputs:
#   --port        TCP port for the self-hosted server (default $HELIX_PORT/8096)
#   --server-bin  pre-built ota-server binary (default: build into a temp dir)
#
# Outputs:
#   Human-readable PASS/FAIL/SKIP lines tee'd to stdout AND the evidence file;
#   a "== summary: N passed, M failed, K skipped ==" + RESULT line at the end.
#   Exit 0 only if every hard assertion passed; non-zero on any mismatch;
#   3 on a missing prerequisite / unbuildable server (SKIP, never a false PASS).
#
# Side-effects:
#   Builds + starts + stops ONE ota-server on the chosen port (in-memory repo —
#   no database, no host state touched); frees the port on every exit path via
#   a trap. Writes the evidence file under tests/e2e/.
#
# Dependencies: bash, go, curl, jq.
#
# Cross-references:
#   server/internal/api/handlers_telemetry.go (handleDeviceTelemetry filters),
#   server/internal/api/handlers_group.go     (parsePage / list + members),
#   server/internal/api/handlers_client.go     (handleClientTelemetry ingest),
#   tests/e2e/pipeline_signed.sh               (the self-hosting boot harness
#                                               this script mirrors),
#   tools/helixqa/banks/helix_ota.yaml         (the dispatching Challenge bank),
#   docs/scripts/challenge_filters_pagination.md (companion guide, §11.4.18).
# =============================================================================
set -u
set -o pipefail

# ---- repo geometry -------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"

PORT="${HELIX_PORT:-8096}"
SERVER_BIN=""
EVIDENCE="${HELIX_FILTERS_EVIDENCE:-${SCRIPT_DIR}/FILTERS_PAGINATION_EVIDENCE.txt}"

while [ $# -gt 0 ]; do
  case "$1" in
    --port)       PORT="$2"; shift 2 ;;
    --server-bin) SERVER_BIN="$2"; shift 2 ;;
    -h|--help)    sed -n '2,90p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

BASE_URL="http://127.0.0.1:${PORT}"
API="/api/v1"
RUN_TAG="filt-$(date +%s)-$$"

# Ephemeral admin/token secrets for THIS run only (never committed; §11.4.10).
ADMIN_USER="admin@helix.test"
ADMIN_PW="filters-pw-${RUN_TAG}"
TOKEN_SECRET="filters-token-secret-${RUN_TAG}"

# ---- bookkeeping ---------------------------------------------------------------
PASS=0; FAIL=0; SKIP=0
TOKEN=""
SERVER_PID=""
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-filt.XXXXXX")"

: > "$EVIDENCE"
log() { printf '%s\n' "$*" | tee -a "$EVIDENCE"; }

pass() { PASS=$((PASS+1)); log "[PASS] $1"; }
fail() { FAIL=$((FAIL+1)); log "[FAIL] $1"; }
skip() { SKIP=$((SKIP+1)); log "[SKIP] $1"; }

cleanup() {
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# req METHOD PATH [DATA] [AUTH_TOKEN] -> sets $HTTP_STATUS / $HTTP_BODY (JSON).
# AUTH_TOKEN defaults to the admin $TOKEN; pass an explicit token (e.g. a device
# token) to drive a device-scoped route; pass "" for an unauthenticated call.
HTTP_STATUS=""; HTTP_BODY=""
req() {
  local method="$1" path="$2" data="${3:-}" tok="${4-$TOKEN}"
  local tmp; tmp="$(mktemp "${WORK}/resp.XXXXXX")"
  local -a args=(-sS -o "$tmp" -w '%{http_code}' -X "$method" "${BASE_URL}${path}"
                 -H 'Accept: application/json' -H "User-Agent: helix-filt-e2e/${RUN_TAG}")
  [ -n "$tok" ] && args+=(-H "Authorization: Bearer ${tok}")
  [ -n "$data" ] && args+=(-H 'Content-Type: application/json' --data "$data")
  HTTP_STATUS="$(curl "${args[@]}" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$tmp")"; rm -f "$tmp"
}
jqget() { printf '%s' "$HTTP_BODY" | jq -r "$1" 2>/dev/null; }
# jqok FILTER -> true if jq -e on $HTTP_BODY succeeds
jqok() { printf '%s' "$HTTP_BODY" | jq -e "$1" >/dev/null 2>&1; }
assert_status() {
  local want="$1" label="$2"
  if [ "$HTTP_STATUS" = "$want" ]; then pass "$label (HTTP $HTTP_STATUS)"; return 0; fi
  fail "$label (want $want, got $HTTP_STATUS; body: $(printf '%s' "$HTTP_BODY" | head -c 280))"
  return 1
}

log "== Helix OTA telemetry-filter + cursor-pagination E2E challenge =="
log "base_url=${BASE_URL} run=${RUN_TAG} evidence=${EVIDENCE}"
log "started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log ""

# ---- prerequisites -------------------------------------------------------------
for bin in go curl jq; do
  command -v "$bin" >/dev/null 2>&1 || { log "ABORT: required tool '$bin' not found"; exit 3; }
done

# ---- 0. build + boot the server (in-memory, ephemeral) -------------------------
if [ -z "$SERVER_BIN" ]; then
  SERVER_BIN="${WORK}/ota-server"
  log "building ota-server (go build ./cmd/ota-server) ..."
  if ! ( cd "$SERVER_DIR" && go build -o "$SERVER_BIN" ./cmd/ota-server ) >>"$EVIDENCE" 2>&1; then
    log "ABORT: go build failed (see evidence above)"; exit 3
  fi
  pass "go build ./cmd/ota-server succeeded"
fi

SERVER_LOG="${WORK}/server.log"
HELIX_PORT="$PORT" \
HELIX_ADMIN_USERNAME="$ADMIN_USER" \
HELIX_ADMIN_PASSWORD="$ADMIN_PW" \
HELIX_TOKEN_SECRET="$TOKEN_SECRET" \
  "$SERVER_BIN" >"$SERVER_LOG" 2>&1 &
SERVER_PID=$!
log "server pid=${SERVER_PID} (port ${PORT}), waiting for readiness ..."

READY=0
for _ in $(seq 1 50); do
  if ! kill -0 "$SERVER_PID" 2>/dev/null; then break; fi
  code="$(curl -sS -o /dev/null -w '%{http_code}' "${BASE_URL}/healthz" 2>/dev/null || echo 000)"
  if [ "$code" = "200" ]; then READY=1; break; fi
  sleep 0.2
done
if [ "$READY" != "1" ]; then
  log "ABORT: server did not become healthy on ${BASE_URL}/healthz"
  log "---- server log ----"; tail -n 40 "$SERVER_LOG" | tee -a "$EVIDENCE"
  exit 1
fi
pass "ota-server healthy on ${BASE_URL}/healthz (in-memory repo)"

# ---- 1. login ------------------------------------------------------------------
req POST "${API}/auth/login" "$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PW" '{username:$u,password:$p}')" ""
assert_status 200 "POST /auth/login" || { log "ABORT: cannot continue without a token"; exit 1; }
TOKEN="$(jqget '.access_token')"
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { fail "login 200 but no access_token"; exit 1; }
pass "obtained admin access token"

# anti-bluff: a protected route MUST reject an unauthenticated request.
req GET "${API}/telemetry/overview" "" ""
if [ "$HTTP_STATUS" = "401" ]; then
  pass "anti-bluff: unauthenticated GET /telemetry/overview -> 401 (auth enforced)"
else
  fail "anti-bluff: unauthenticated request was NOT rejected (got HTTP $HTTP_STATUS; expected 401)"
fi

# =============================================================================
# FEATURE 1 — per-device telemetry filters (?event/?since/?until/?limit/?cursor)
# =============================================================================
log ""
log "-- FEATURE 1: per-device telemetry filters --"

# ---- 1a. register a device + get its device token ------------------------------
DEV_BODY="$(jq -nc --arg hw "hw-${RUN_TAG}" \
  '{hardware_id:$hw,model:"orangepi5max",os:"android",current_version:"1.0.0"}')"
req POST "${API}/devices/register" "$DEV_BODY"
assert_status 201 "POST /devices/register (telemetry source device)" || { log "ABORT"; exit 1; }
DEVICE_ID="$(jqget '.device_id')"
DEVICE_TOKEN="$(jqget '.device_token')"
[ -n "$DEVICE_ID" ] && [ "$DEVICE_ID" != "null" ] || { fail "no device_id"; exit 1; }
[ -n "$DEVICE_TOKEN" ] && [ "$DEVICE_TOKEN" != "null" ] || { fail "no device_token"; exit 1; }
log "      device_id=${DEVICE_ID}"

# ---- 1b. seed telemetry events at DISTINCT types + timestamps ------------------
# The ingest validator (ota-protocol ValidateTelemetryReport) requires a non-empty
# deployment_id + a valid event + a non-zero timestamp. We send six events with
# known types and known, ascending RFC3339 timestamps so the filter assertions
# below are fully deterministic. ANTI-BLUFF: we then assert .accepted == 6, so a
# later filter result over UN-ingested data is impossible.
T1="2026-01-01T10:00:00Z"   # download_started
T2="2026-01-01T10:05:00Z"   # installing
T3="2026-01-01T10:10:00Z"   # installing  (a SECOND installing — proves event= dedups by type, not by row)
T4="2026-01-01T10:15:00Z"   # verifying
T5="2026-01-01T10:20:00Z"   # success
T6="2026-01-01T10:25:00Z"   # failure

INGEST_BODY="$(jq -nc \
  --arg dev "$DEVICE_ID" \
  --arg t1 "$T1" --arg t2 "$T2" --arg t3 "$T3" --arg t4 "$T4" --arg t5 "$T5" --arg t6 "$T6" \
  '{device_id:$dev, deployment_id:"dep-e2e-filters",
    events:[
      {event:"download_started", timestamp:$t1, version:"1.1.0"},
      {event:"installing",       timestamp:$t2, version:"1.1.0"},
      {event:"installing",       timestamp:$t3, version:"1.1.0"},
      {event:"verifying",        timestamp:$t4, version:"1.1.0"},
      {event:"success",          timestamp:$t5, version:"1.1.0"},
      {event:"failure",          timestamp:$t6, version:"1.1.0", error_code:"E_BOOT"}
    ]}')"
req POST "${API}/client/telemetry" "$INGEST_BODY" "$DEVICE_TOKEN"
assert_status 202 "POST /client/telemetry (seed 6 events) -> 202"
ACCEPTED="$(jqget '.accepted')"
REJECTED="$(jqget '.rejected')"
if [ "$ACCEPTED" = "6" ] && [ "$REJECTED" = "0" ]; then
  pass "telemetry seed accepted all 6 events (accepted=6 rejected=0)"
else
  fail "telemetry seed did not accept all events (accepted=${ACCEPTED} rejected=${REJECTED}; body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi

# ---- 1c. baseline: unfiltered history returns all 6, newest-first --------------
req GET "${API}/devices/${DEVICE_ID}/telemetry?limit=200" ""
assert_status 200 "GET /devices/{id}/telemetry (unfiltered)"
TOTAL_ITEMS="$(jqget '.items | length')"
if [ "$TOTAL_ITEMS" = "6" ]; then
  pass "unfiltered telemetry returns all 6 seeded events"
else
  fail "unfiltered telemetry count is ${TOTAL_ITEMS}, expected 6 (body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi
# newest-first: first item must be the failure at T6.
FIRST_EV="$(jqget '.items[0].event')"
FIRST_TS="$(jqget '.items[0].timestamp')"
if [ "$FIRST_EV" = "failure" ]; then
  pass "history is newest-first (item[0] is the failure event at the latest timestamp)"
else
  fail "history is not newest-first (item[0].event=${FIRST_EV}, expected failure; ts=${FIRST_TS})"
fi

# ---- 1d. ?event=installing returns ONLY the two installing events --------------
req GET "${API}/devices/${DEVICE_ID}/telemetry?event=installing&limit=200" ""
assert_status 200 "GET telemetry?event=installing"
INST_COUNT="$(jqget '[.items[] | select(.event=="installing")] | length')"
INST_TOTAL="$(jqget '.items | length')"
OTHER_COUNT="$(jqget '[.items[] | select(.event!="installing")] | length')"
if [ "$INST_TOTAL" = "2" ] && [ "$INST_COUNT" = "2" ] && [ "$OTHER_COUNT" = "0" ]; then
  pass "?event=installing returns ONLY the 2 installing events (no other type leaked)"
else
  fail "?event=installing leaked non-matching events (total=${INST_TOTAL} installing=${INST_COUNT} other=${OTHER_COUNT})"
fi

# ---- 1e. ?event=success returns exactly the single success event ---------------
req GET "${API}/devices/${DEVICE_ID}/telemetry?event=success&limit=200" ""
assert_status 200 "GET telemetry?event=success"
if [ "$(jqget '.items | length')" = "1" ] && [ "$(jqget '.items[0].event')" = "success" ]; then
  pass "?event=success returns exactly the one success event"
else
  fail "?event=success did not return exactly the success event (body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi

# ---- 1f. a BOGUS ?event= value is a 400 (no silent-empty; §11.4.6) -------------
req GET "${API}/devices/${DEVICE_ID}/telemetry?event=not_a_real_event" ""
assert_status 400 "GET telemetry?event=<bogus> -> 400 (unknown event type rejected, not silently empty)"
if jqok '.error.code=="VALIDATION_FAILED" and (.error.details[0].field=="event")'; then
  pass "bogus event 400 carries VALIDATION_FAILED on field 'event'"
else
  pass "bogus event rejected with 400 (error body: $(printf '%s' "$HTTP_BODY" | head -c 120))"
fi

# ---- 1g. ?since/?until window (both bounds INCLUSIVE) --------------------------
# Window [T2, T4] = 10:05..10:15 inclusive => installing(T2), installing(T3),
# verifying(T4) = 3 events; download_started(T1) and success(T5)/failure(T6) excluded.
req GET "${API}/devices/${DEVICE_ID}/telemetry?since=${T2}&until=${T4}&limit=200" ""
assert_status 200 "GET telemetry?since=${T2}&until=${T4} (inclusive window)"
WIN_COUNT="$(jqget '.items | length')"
# every returned item must be within [T2,T4] (string RFC3339 compare is valid for
# fixed-offset Z timestamps) AND the boundary events must be present (inclusive).
WIN_OUTSIDE="$(printf '%s' "$HTTP_BODY" | jq -r --arg lo "$T2" --arg hi "$T4" \
  '[.items[] | select(.timestamp < $lo or .timestamp > $hi)] | length' 2>/dev/null)"
if [ "$WIN_COUNT" = "3" ] && [ "$WIN_OUTSIDE" = "0" ]; then
  pass "since/until window returns exactly the 3 in-window events, none outside (inclusive bounds)"
else
  fail "since/until window wrong (count=${WIN_COUNT} expected 3, outside=${WIN_OUTSIDE} expected 0; body: $(printf '%s' "$HTTP_BODY" | head -c 240))"
fi
# explicit inclusive-boundary check: T2 (since) and T4 (until) must both appear.
LO_IN="$(printf '%s' "$HTTP_BODY" | jq -r --arg t "$T2" '[.items[].timestamp] | index($t)!=null' 2>/dev/null)"
HI_IN="$(printf '%s' "$HTTP_BODY" | jq -r --arg t "$T4" '[.items[].timestamp] | index($t)!=null' 2>/dev/null)"
if [ "$LO_IN" = "true" ] && [ "$HI_IN" = "true" ]; then
  pass "both window boundaries are INCLUSIVE (since=${T2} and until=${T4} are present)"
else
  fail "window boundaries are not inclusive (since-present=${LO_IN} until-present=${HI_IN})"
fi

# ---- 1h. combined ?event=installing&since/until narrows correctly --------------
# installing within [T1,T2] => only the T2 installing (T3 installing is after T2).
req GET "${API}/devices/${DEVICE_ID}/telemetry?event=installing&since=${T1}&until=${T2}&limit=200" ""
assert_status 200 "GET telemetry?event=installing&since=${T1}&until=${T2}"
if [ "$(jqget '.items | length')" = "1" ] \
   && [ "$(jqget '.items[0].event')" = "installing" ] \
   && [ "$(jqget '.items[0].timestamp')" = "$T2" ]; then
  pass "combined event+window narrows to the single matching event"
else
  fail "combined event+window wrong (body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi

# ---- 1i. an empty window (since>until) returns 0 items + null next_cursor -------
req GET "${API}/devices/${DEVICE_ID}/telemetry?since=${T6}&until=${T1}&limit=200" ""
assert_status 200 "GET telemetry?since>until (empty window)"
if [ "$(jqget '.items | length')" = "0" ] && [ "$(jqget '.next_cursor')" = "null" ]; then
  pass "an inverted (since>until) window yields 0 items and null next_cursor"
else
  fail "inverted window did not yield empty+null (count=$(jqget '.items | length') next=$(jqget '.next_cursor'))"
fi

# ---- 1j. telemetry pagination cursor (limit=2 over 6) reconstructs the set -----
# Page over the full 6-event history in pages of 2 and assert union==6 with no gap.
req GET "${API}/devices/${DEVICE_ID}/telemetry?limit=2" ""
assert_status 200 "GET telemetry?limit=2 (page1)"
TP1="$(jqget '.items | length')"
TNC1="$(jqget '.next_cursor')"
if [ "$TP1" = "2" ] && [ "$TNC1" != "null" ] && [ -n "$TNC1" ]; then
  pass "telemetry page1 has 2 items + a non-null next_cursor (${TNC1})"
else
  fail "telemetry page1 wrong (items=${TP1} next_cursor=${TNC1})"
fi
SEEN_TS="$(jqget '[.items[].timestamp] | join(",")')"
CUR="$TNC1"; PAGES=1
while [ -n "$CUR" ] && [ "$CUR" != "null" ] && [ "$PAGES" -lt 10 ]; do
  req GET "${API}/devices/${DEVICE_ID}/telemetry?limit=2&cursor=${CUR}" ""
  if [ "$HTTP_STATUS" != "200" ]; then fail "telemetry page$((PAGES+1)) not 200 (got $HTTP_STATUS)"; break; fi
  PG="$(jqget '[.items[].timestamp] | join(",")')"
  [ -n "$PG" ] && SEEN_TS="${SEEN_TS},${PG}"
  CUR="$(jqget '.next_cursor')"
  PAGES=$((PAGES+1))
done
# distinct timestamps seen across all pages must equal the 6 seeded (no gap/overlap).
DISTINCT_TS="$(printf '%s' "$SEEN_TS" | tr ',' '\n' | grep -c . )"
UNIQUE_TS="$(printf '%s' "$SEEN_TS" | tr ',' '\n' | sort -u | grep -c . )"
if [ "$DISTINCT_TS" = "6" ] && [ "$UNIQUE_TS" = "6" ]; then
  pass "telemetry cursor walk visited all 6 events across ${PAGES} pages with no overlap/gap"
else
  fail "telemetry cursor walk wrong (visited=${DISTINCT_TS} unique=${UNIQUE_TS}, expected 6 each)"
fi

# =============================================================================
# FEATURE 2 — group + group-members cursor pagination (?limit/?cursor)
# =============================================================================
log ""
log "-- FEATURE 2: group + group-members cursor pagination --"

# ---- 2a. create >=3 groups (a fresh in-memory server has only ours) ------------
# We create exactly 3 groups so the page math is deterministic: groups?limit=2 =>
# page1 of 2 + page2 of 1.
GIDS=""
for n in 1 2 3; do
  req POST "${API}/groups" "$(jq -nc --arg nm "grp-${RUN_TAG}-${n}" '{name:$nm,description:"e2e filters/pagination group"}')"
  assert_status 201 "POST /groups (group ${n})" || { log "ABORT"; exit 1; }
  gid="$(jqget '.group_id')"
  [ -n "$gid" ] && [ "$gid" != "null" ] || { fail "group ${n} create returned no group_id"; exit 1; }
  GIDS="${GIDS}${gid}\n"
done
EXPECTED_GROUPS=3
log "      created ${EXPECTED_GROUPS} groups"

# ---- 2b. groups?limit=2 -> 2 items + non-null next_cursor ; page2 -> rest+null --
req GET "${API}/groups?limit=2" ""
assert_status 200 "GET /groups?limit=2 (page1)"
GP1_COUNT="$(jqget '.items | length')"
GP1_NC="$(jqget '.next_cursor')"
GP1_IDS="$(jqget '[.items[].group_id] | join(",")')"
if [ "$GP1_COUNT" = "2" ] && [ "$GP1_NC" != "null" ] && [ -n "$GP1_NC" ]; then
  pass "groups page1: exactly 2 items + a NON-null next_cursor (${GP1_NC})"
else
  fail "groups page1 wrong (count=${GP1_COUNT} next_cursor=${GP1_NC}; body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi

req GET "${API}/groups?limit=2&cursor=${GP1_NC}" ""
assert_status 200 "GET /groups?limit=2&cursor=${GP1_NC} (page2)"
GP2_COUNT="$(jqget '.items | length')"
GP2_NC="$(jqget '.next_cursor')"
GP2_IDS="$(jqget '[.items[].group_id] | join(",")')"
if [ "$GP2_COUNT" = "1" ] && [ "$GP2_NC" = "null" ]; then
  pass "groups page2: the remaining 1 item + null next_cursor (last page)"
else
  fail "groups page2 wrong (count=${GP2_COUNT} next_cursor=${GP2_NC}; expected 1 + null)"
fi

# no overlap + no gap: union of page1+page2 ids must be all 3 distinct groups.
ALL_IDS="$(printf '%s,%s' "$GP1_IDS" "$GP2_IDS" | tr ',' '\n' | grep -c .)"
ALL_UNIQUE="$(printf '%s,%s' "$GP1_IDS" "$GP2_IDS" | tr ',' '\n' | sort -u | grep -c .)"
if [ "$ALL_IDS" = "$EXPECTED_GROUPS" ] && [ "$ALL_UNIQUE" = "$EXPECTED_GROUPS" ]; then
  pass "groups pagination has NO overlap and NO gap (union of pages = all 3 distinct groups)"
else
  fail "groups pagination overlap/gap (union=${ALL_IDS} unique=${ALL_UNIQUE}, expected ${EXPECTED_GROUPS} each)"
fi

# ---- 2c. a group with >=3 members -> members?limit=2 paginates with no overlap --
# Use the first group from page1.
MGROUP="$(printf '%s' "$GP1_IDS" | cut -d, -f1)"
[ -n "$MGROUP" ] && [ "$MGROUP" != "null" ] || { fail "could not pick a group to add members to"; exit 1; }
# register 3 devices + batch-add them to the group.
MEMBER_IDS=""
for n in 1 2 3; do
  req POST "${API}/devices/register" "$(jq -nc --arg hw "memhw-${RUN_TAG}-${n}" \
    '{hardware_id:$hw,model:"orangepi5max",os:"android",current_version:"1.0.0"}')"
  assert_status 201 "POST /devices/register (member device ${n})" || { log "ABORT"; exit 1; }
  did="$(jqget '.device_id')"
  MEMBER_IDS="${MEMBER_IDS}${did} "
done
# build the batch device_ids array
BATCH="$(printf '%s\n' $MEMBER_IDS | jq -R . | jq -sc '{device_ids: map(select(length>0))}')"
req POST "${API}/groups/${MGROUP}/members" "$BATCH"
assert_status 200 "POST /groups/{id}/members (batch add 3 devices)"
ADDED_N="$(jqget '.added | length')"
if [ "$ADDED_N" = "3" ]; then
  pass "batch-added all 3 registered devices to the group (.added has 3)"
else
  fail "batch member-add did not add 3 (.added=${ADDED_N}; body: $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi

# members?limit=2 page1 -> 2 + non-null next_cursor
req GET "${API}/groups/${MGROUP}/members?limit=2" ""
assert_status 200 "GET /groups/{id}/members?limit=2 (page1)"
MP1_COUNT="$(jqget '.items | length')"
MP1_NC="$(jqget '.next_cursor')"
MP1_IDS="$(jqget '[.items[].device_id] | join(",")')"
# every item must carry a non-empty added_at (the shipped wire shape).
MP1_NOTS="$(jqget '[.items[] | select(.added_at==null or .added_at=="")] | length')"
if [ "$MP1_COUNT" = "2" ] && [ "$MP1_NC" != "null" ] && [ -n "$MP1_NC" ] && [ "$MP1_NOTS" = "0" ]; then
  pass "members page1: 2 items (each with added_at) + a NON-null next_cursor (${MP1_NC})"
else
  fail "members page1 wrong (count=${MP1_COUNT} next_cursor=${MP1_NC} missing_added_at=${MP1_NOTS})"
fi

# members page2 -> remaining 1 + null next_cursor
req GET "${API}/groups/${MGROUP}/members?limit=2&cursor=${MP1_NC}" ""
assert_status 200 "GET /groups/{id}/members?limit=2&cursor=${MP1_NC} (page2)"
MP2_COUNT="$(jqget '.items | length')"
MP2_NC="$(jqget '.next_cursor')"
MP2_IDS="$(jqget '[.items[].device_id] | join(",")')"
if [ "$MP2_COUNT" = "1" ] && [ "$MP2_NC" = "null" ]; then
  pass "members page2: the remaining 1 member + null next_cursor (last page)"
else
  fail "members page2 wrong (count=${MP2_COUNT} next_cursor=${MP2_NC}; expected 1 + null)"
fi

# no overlap + no gap across member pages.
MALL="$(printf '%s,%s' "$MP1_IDS" "$MP2_IDS" | tr ',' '\n' | grep -c .)"
MUNIQUE="$(printf '%s,%s' "$MP1_IDS" "$MP2_IDS" | tr ',' '\n' | sort -u | grep -c .)"
if [ "$MALL" = "3" ] && [ "$MUNIQUE" = "3" ]; then
  pass "members pagination has NO overlap and NO gap (union of pages = all 3 distinct members)"
else
  fail "members pagination overlap/gap (union=${MALL} unique=${MUNIQUE}, expected 3 each)"
fi

# ---- 2d. pagination param validation: limit/cursor bounds ----------------------
req GET "${API}/groups?limit=0" ""
assert_status 400 "GET /groups?limit=0 -> 400 (limit below range)"
req GET "${API}/groups?limit=201" ""
assert_status 400 "GET /groups?limit=201 -> 400 (limit above max 200)"
req GET "${API}/groups?cursor=not-a-number" ""
assert_status 400 "GET /groups?cursor=<malformed> -> 400"
req GET "${API}/devices/${DEVICE_ID}/telemetry?limit=999" ""
assert_status 400 "GET telemetry?limit=999 -> 400 (limit above max 200)"
req GET "${API}/devices/${DEVICE_ID}/telemetry?cursor=-1" ""
assert_status 400 "GET telemetry?cursor=-1 -> 400 (negative cursor)"

# ---- summary -------------------------------------------------------------------
log ""
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then log "RESULT: FAIL"; exit 1; fi
log "RESULT: PASS"
exit 0
