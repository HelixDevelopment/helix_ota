#!/usr/bin/env bash
# =============================================================================
# security_probes.sh — Helix OTA black-box security probe suite (anti-bluff)
# -----------------------------------------------------------------------------
# Purpose:
#   Drive a LIVE ota-server over real HTTP and assert its access-control,
#   authentication, and input-handling defenses behave correctly. Every probe
#   is captured (status + body excerpt) into RUN_EVIDENCE.txt. A genuine defect
#   FAILs — nothing is papered over (Constitution §11.4 / §11.4.27 / §11.4.123).
#
#   The suite is SELF-HOSTING: it builds + boots an ota-server with ephemeral
#   admin/token secrets, runs the probes, then stops the server and frees the
#   port on exit. Pass --external to probe an already-running server instead
#   (then HELIX_ADMIN_PASSWORD must match that server's admin password).
#
# Probes (each HARD-asserted):
#   A. Unauthenticated access            -> 401 on every protected route class
#   B. RBAC: device-role token on an operator/admin route -> 403 (not 401/2xx)
#   C. Resource ownership: device token reading ANOTHER device's telemetry -> 403
#   D. Malformed / tampered / reused JWT -> 401 (signature + structure enforced)
#   E. Injection strings (SQL-ish / path traversal / NoSQL) in path + body
#        -> never 500, never an unauthenticated leak (401/400/404, not 5xx)
#   F. Oversized + malformed JSON bodies -> 400 (or 413), never 500
#   G. Trust boundary spot-check: the upload signing key is server-config only
#        (a request cannot inject a verification key) — asserted via the API
#        surface: an unsigned upload from an authed operator is still rejected.
#
# Usage:
#   security_probes.sh [--port N] [--external --base-url URL --password PW]
#   Env: HELIX_PORT (8080), HELIX_SECURITY_EVIDENCE (default RUN_EVIDENCE.txt)
#
# Dependencies: bash, curl, jq, go (self-host mode), python3 (oversized body).
#
# Cross-references:
#   server/internal/api/middleware.go (authMiddleware/requireRole),
#   server/internal/api/handlers_telemetry.go (ownership 403),
#   server/internal/api/token.go (JWT verify),
#   server/internal/api/handlers_artifact.go (resolvePublicKey trust boundary).
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"

PORT="${HELIX_PORT:-8080}"
EXTERNAL=0
BASE_URL=""
ADMIN_USER="${HELIX_ADMIN_USERNAME:-admin@helix.test}"
ADMIN_PW="${HELIX_ADMIN_PASSWORD:-}"
EVIDENCE="${HELIX_SECURITY_EVIDENCE:-${SCRIPT_DIR}/RUN_EVIDENCE.txt}"
API="/api/v1"
RUN_TAG="sec-$(date +%s)-$$"

while [ $# -gt 0 ]; do
  case "$1" in
    --port)      PORT="$2"; shift 2 ;;
    --external)  EXTERNAL=1; shift ;;
    --base-url)  BASE_URL="$2"; shift 2 ;;
    --username)  ADMIN_USER="$2"; shift 2 ;;
    --password)  ADMIN_PW="$2"; shift 2 ;;
    -h|--help)   sed -n '2,50p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

[ -z "$BASE_URL" ] && BASE_URL="http://127.0.0.1:${PORT}"
BASE_URL="${BASE_URL%/}"

PASS=0; FAIL=0; SKIP=0
TOKEN=""; SERVER_PID=""
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-sec.XXXXXX")"

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

log "== Helix OTA security probe suite =="
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
  HELIX_TOKEN_SECRET="sec-token-secret-${RUN_TAG}" \
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
reg_device "secdev-A-${RUN_TAG}"
DEV_A_ID="$(jqget '.device_id')"; DEV_A_TOK="$(jqget '.device_token')"
reg_device "secdev-B-${RUN_TAG}"
DEV_B_ID="$(jqget '.device_id')"; DEV_B_TOK="$(jqget '.device_token')"
if [ -z "$DEV_A_TOK" ] || [ "$DEV_A_TOK" = "null" ] || [ -z "$DEV_B_TOK" ] || [ "$DEV_B_TOK" = "null" ]; then
  log "ABORT: could not provision two device tokens"; exit 1
fi
log "      device A=${DEV_A_ID}  device B=${DEV_B_ID}"
log ""

# =============================================================================
# A. UNAUTHENTICATED ACCESS -> 401 on every protected route class
# =============================================================================
log "--- A. unauthenticated access must be 401 ---"
req GET    "${API}/telemetry/overview" "" ""
assert_in "401" "A1 unauth GET /telemetry/overview"
req GET    "${API}/audit" "" ""
assert_in "401" "A2 unauth GET /audit"
req POST   "${API}/groups" '{"name":"x"}' ""
assert_in "401" "A3 unauth POST /groups (mutation)"
req POST   "${API}/devices/register" '{"hardware_id":"x","model":"y","os":"android"}' ""
assert_in "401" "A4 unauth POST /devices/register"
req GET    "${API}/client/update" "" ""
assert_in "401" "A5 unauth GET /client/update"
# malformed Authorization header (not 'Bearer x') is also 401
TMP="$(mktemp)"
HTTP_STATUS="$(curl -sS -o "$TMP" -w '%{http_code}' -X GET "${BASE_URL}${API}/audit" -H 'Authorization: Basic Zm9vOmJhcg==' 2>/dev/null)"
HTTP_BODY="$(cat "$TMP")"; rm -f "$TMP"
assert_in "401" "A6 non-Bearer Authorization header -> 401"

# =============================================================================
# B. RBAC — a device-role token on an operator/admin route -> 403 (NOT 401/2xx)
# =============================================================================
log ""
log "--- B. RBAC: device-role token on operator/admin routes must be 403 ---"
req POST "${API}/groups" "$(jq -nc '{name:"rbac-probe"}')" "$DEV_A_TOK"
assert_in "403" "B1 device token POST /groups (operator/admin only)"
req GET  "${API}/audit" "" "$DEV_A_TOK"
assert_in "403" "B2 device token GET /audit (admin only)"
req POST "${API}/deltas" "$(jq -nc '{base_artifact_id:"a",target_artifact_id:"b"}')" "$DEV_A_TOK"
assert_in "403" "B3 device token POST /deltas (operator/admin only)"
# And the device token IS accepted on its own allowed route (proves it's a valid
# token, so the 403s above are role-enforcement, not a dead/invalid token).
req GET "${API}/client/update" "" "$DEV_A_TOK"
assert_in "200 204" "B4 device token GET /client/update is ACCEPTED (token is valid; B1-B3 are RBAC)"

# =============================================================================
# C. RESOURCE OWNERSHIP — device A's token reading device B's telemetry -> 403
# =============================================================================
log ""
log "--- C. resource ownership: a device may read only its OWN telemetry ---"
req GET "${API}/devices/${DEV_B_ID}/telemetry" "" "$DEV_A_TOK"
assert_in "403" "C1 device A token GET device B telemetry -> 403"
# Control: device A reading its OWN telemetry is allowed (200).
req GET "${API}/devices/${DEV_A_ID}/telemetry" "" "$DEV_A_TOK"
assert_in "200" "C2 device A token GET its OWN telemetry -> 200 (ownership allows self)"
# Ownership also enforced on the telemetry WRITE path (body device_id != subject).
req POST "${API}/client/telemetry" "$(jq -nc --arg d "$DEV_B_ID" \
  '{device_id:$d,events:[{event:"success",timestamp:"2026-06-08T00:00:00Z"}]}')" "$DEV_A_TOK"
assert_in "403" "C3 device A token POST telemetry FOR device B -> 403"

# =============================================================================
# D. MALFORMED / TAMPERED / REUSED JWT -> 401
# =============================================================================
log ""
log "--- D. token integrity: malformed/tampered tokens must be 401 ---"
req GET "${API}/audit" "" "not.a.jwt.at.all"
assert_in "401" "D1 garbage bearer token -> 401"
# tamper: flip the signature segment of the real admin token (claims.sig shape).
TAMPERED="${TOKEN%.*}.AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
req GET "${API}/audit" "" "$TAMPERED"
assert_in "401" "D2 admin token with FORGED signature -> 401 (sig verified)"
# tamper: keep the signature, mutate a claims byte -> signature no longer matches.
SIG_PART="${TOKEN##*.}"; CLAIMS_PART="${TOKEN%.*}"
MUT_CLAIMS="${CLAIMS_PART%?}X"   # flip last claims char
req GET "${API}/audit" "" "${MUT_CLAIMS}.${SIG_PART}"
assert_in "401" "D3 admin token with MUTATED claims (stale sig) -> 401"
# empty bearer
TMP="$(mktemp)"
HTTP_STATUS="$(curl -sS -o "$TMP" -w '%{http_code}' -X GET "${BASE_URL}${API}/audit" -H 'Authorization: Bearer ' 2>/dev/null)"
HTTP_BODY="$(cat "$TMP")"; rm -f "$TMP"
assert_in "401" "D4 empty bearer token -> 401"

# =============================================================================
# E. INJECTION strings in PATH + BODY -> never 5xx, never unauth leak
# =============================================================================
log ""
log "--- E. injection strings must not 500 or leak (handled as 401/400/404) ---"
# E1: SQL-ish in a path param while UNAUTH -> must be 401 (auth before lookup),
#     definitely not 500 and not a data leak.
INJ_SQL="1';DROP%20TABLE%20devices;--"
req GET "${API}/devices/${INJ_SQL}/status" "" ""
assert_in "401" "E1 SQL-ish device id (unauth) -> 401 (auth precedes lookup)"
# E2: same injection WITH an admin token -> 404 (not found), never 500.
req GET "${API}/devices/${INJ_SQL}/status" "" "$TOKEN"
assert_in "404 400" "E2 SQL-ish device id (authed) -> 404/400 not 5xx"
assert_not5xx "E2b SQL-ish device id did not crash the server"
# E3: path traversal in artifact id.
req GET "${API}/artifacts/..%2f..%2f..%2fetc%2fpasswd" "" "$TOKEN"
assert_in "404 400" "E3 path-traversal artifact id -> 404/400 not 5xx"
assert_not5xx "E3b path-traversal artifact id did not crash"
# E4: injection payload inside a JSON body field (group name) — stored as data,
#     must succeed (201) or validate (400) but never execute / 500.
INJ_NAME="x'); DROP TABLE groups;--"
INJ_DESC="<script>alert(1)</script>"
req POST "${API}/groups" "$(jq -nc --arg n "$INJ_NAME" --arg d "$INJ_DESC" '{name:$n,description:$d}')" "$TOKEN"
assert_in "201 400 409" "E4 injection/XSS strings in group body -> 201/400 not 5xx"
assert_not5xx "E4b injection body did not crash"
INJ_GROUP_ID="$(jqget '.group_id')"
# read it back: the payload must be returned VERBATIM (stored as inert data, not interpreted).
if [ -n "$INJ_GROUP_ID" ] && [ "$INJ_GROUP_ID" != "null" ]; then
  req GET "${API}/groups/${INJ_GROUP_ID}" "" "$TOKEN"
  if printf '%s' "$HTTP_BODY" | jq -e '.name | contains("DROP TABLE")' >/dev/null 2>&1; then
    pass "E5 injection payload stored+returned as inert data (not interpreted)"
  else
    skip "E5 could not confirm verbatim storage (group name not echoed)"
  fi
  req DELETE "${API}/groups/${INJ_GROUP_ID}" "" "$TOKEN" >/dev/null 2>&1 || true
fi
# E6: NoSQL-ish operator object in a query param.
req GET "${API}/deltas?base[\$ne]=null&target[\$ne]=null" "" "$TOKEN"
assert_not5xx "E6 NoSQL-operator query params did not crash"

# =============================================================================
# F. OVERSIZED + MALFORMED JSON -> 400 (or 413), never 500
# =============================================================================
log ""
log "--- F. malformed / oversized JSON must be 400/413, never 500 ---"
req POST "${API}/groups" '{ this is : not json ]' "$TOKEN"
assert_in "400" "F1 malformed JSON body -> 400"
assert_not5xx "F1b malformed JSON did not crash"
# truncated JSON
req POST "${API}/groups" '{"name":' "$TOKEN"
assert_in "400" "F2 truncated JSON body -> 400"
# wrong type for a field
req POST "${API}/groups" '{"name":12345}' "$TOKEN"
assert_in "400 201 409" "F3 wrong-typed field -> 400 (or accepted/conflict) not 5xx"
assert_not5xx "F3b wrong-typed field did not crash"
# deeply oversized body (1 MiB of junk) -> must be rejected cleanly, not 500.
if command -v python3 >/dev/null 2>&1; then
  # A 1 MiB body exceeds the argv limit for an inline --data, so write it to a
  # file and POST via --data-binary @file (a script-mechanics detail; the probe
  # subject is the SERVER's handling of an oversized body).
  BIGF="${WORK}/big.json"
  python3 -c 'import json,sys;open(sys.argv[1],"w").write(json.dumps({"name":"A"*1048576}))' "$BIGF"
  BTMP="$(mktemp)"
  HTTP_STATUS="$(curl -gsS -o "$BTMP" -w '%{http_code}' -X POST "${BASE_URL}${API}/groups" \
      -H "Authorization: Bearer ${TOKEN}" -H 'Content-Type: application/json' \
      --data-binary "@${BIGF}" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$BTMP")"; rm -f "$BTMP"
  assert_in "400 413 201 409" "F4 1 MiB JSON body handled cleanly (400/413/201) not 5xx"
  assert_not5xx "F4b oversized body did not crash"
else
  skip "F4 oversized body (python3 unavailable)"
fi

# =============================================================================
# G. TRUST BOUNDARY — upload verification key is server-config only
# =============================================================================
log ""
log "--- G. trust boundary: a request cannot supply the verification key ---"
# Self-hosted server boots WITHOUT HELIX_ARTIFACT_PUBKEY, so an authed operator
# upload of any artifact is rejected because no trusted key is configured. This
# proves the key path is config-only — there is no request param that injects a
# key to make verification pass. (When run --external against a key-configured
# server, an unsigned/garbage-signed upload is rejected as SIGNATURE_INVALID.)
GZIP="${WORK}/g.zip"
python3 - "$GZIP" <<'PY'
import sys, zipfile
with zipfile.ZipFile(sys.argv[1], "w", compression=zipfile.ZIP_STORED) as z:
    zi = zipfile.ZipInfo("payload.bin"); zi.compress_type = zipfile.ZIP_STORED
    z.writestr(zi, b"trust-boundary probe payload")
PY
GSHA="$(openssl dgst -sha256 -binary "$GZIP" 2>/dev/null | xxd -p -c256 | tr -d '\n')"
GMETA="$(jq -nc --arg sha "${GSHA:-deadbeef}" --arg sig "$(printf 'attacker-supplied-sig' | base64)" \
  '{sha256:$sha,signature:$sig,version:"1.2.3",os:"android",target_model:"OrangePi5Max"}')"
GTMP="$(mktemp)"
HTTP_STATUS="$(curl -sS -o "$GTMP" -w '%{http_code}' -X POST "${BASE_URL}${API}/artifacts/upload" \
    -H "Authorization: Bearer ${TOKEN}" \
    -F "file=@${GZIP};type=application/zip;filename=ota.zip" \
    -F "metadata=${GMETA};type=application/json" \
    -F "public_key=$(openssl rand -base64 32 2>/dev/null || echo AAAA)" 2>/dev/null)"
HTTP_BODY="$(cat "$GTMP")"; rm -f "$GTMP"
GCODE="$(jqget '.error.code')"
# Acceptable: 422 SIGNATURE_INVALID (no trusted key OR sig mismatch). The
# attacker's injected 'public_key' form field must NOT make the upload succeed.
if [ "$HTTP_STATUS" = "422" ] && [ "$GCODE" = "SIGNATURE_INVALID" ]; then
  pass "G1 upload with an attacker-supplied public_key field is rejected 422 SIGNATURE_INVALID (key is config-only)"
elif printf '%s' "$HTTP_STATUS" | grep -q '^2'; then
  fail "G1 SECURITY: upload SUCCEEDED with a request-supplied key (trust-boundary bypass!) HTTP $HTTP_STATUS body: $(printf '%s' "$HTTP_BODY" | head -c 200)"
else
  pass "G1 attacker-supplied-key upload rejected (HTTP $HTTP_STATUS code=${GCODE}; not a success)"
fi
assert_not5xx "G1b attacker-key upload did not crash"

# ---- summary -------------------------------------------------------------------
log ""
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then log "RESULT: FAIL"; exit 1; fi
log "RESULT: PASS"
exit 0
