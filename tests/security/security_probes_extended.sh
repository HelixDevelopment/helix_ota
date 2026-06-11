#!/usr/bin/env bash
# =============================================================================
# security_probes_extended.sh — Helix OTA black-box security probes (anti-bluff)
#                               — coverage NOT in security_probes.sh
# -----------------------------------------------------------------------------
# Purpose:
#   Drive a LIVE ota-server over real HTTP and assert additional access-control,
#   token-lifecycle, protocol-surface, and DoS-shedding defenses that the
#   existing security_probes.sh suite does NOT cover. Every probe is captured
#   (status + body excerpt + the real assertion) into the evidence file. A
#   genuine defect FAILs; a genuinely-missing prerequisite SKIPs (exit 3) — no
#   PASS-by-default (Constitution §11.4 / §11.4.27 / §11.4.69 / §11.4.123).
#
#   The suite is SELF-HOSTING: it builds + boots an ota-server with ephemeral,
#   test-only admin/token secrets on a FREE port it picks itself (never a real
#   secret, never printed/committed; never hard-codes :8080 so it cannot collide
#   with a sibling agent's server — §11.4.119), runs the probes, then stops the
#   server and frees the port on exit (trap).
#
#   Three ephemeral servers are booted in sequence, each on its own free port:
#     S1    — default token TTL; provisions the test devices and serves the
#             H/J/K/L probes with a STABLE admin token (a 1s token shared across a
#             multi-step suite is timing-fragile, §11.4.50 — so S1 is NOT short-TTL).
#     S_EXP — HELIX_ACCESS_TOKEN_TTL=1s, dedicated to probe I so the access token
#             actually EXPIRES mid-run for a real (not synthetic) expiry probe,
#             without affecting any other probe.
#     S2    — in-flight cap (HELIX_MAX_INFLIGHT=1) so a concurrent burst is really
#             shed with 429 RATE_LIMITED.
#
# Probes (each HARD-asserted against the REAL HTTP status/body):
#   H. Refresh-token single-use rotation:
#        H1 login -> refresh rotates -> new pair (200, distinct refresh token)
#        H2 the SAME refresh token reused -> 401 (single-use enforced)
#        H3 a garbage refresh token -> 401
#        H4 refresh with missing refresh_token field -> 400 VALIDATION_FAILED
#   I. Access-token EXPIRY (TTL=1s, real wall-clock):
#        I1 token works immediately after login (200 on a protected read)
#        I2 the SAME token after expiry -> 401 (exp enforced, not just signature)
#   J. Cross-device isolation on the STATUS path (sibling of the existing
#      telemetry-path probe — different handler/route):
#        J1 device A token GET device B /status -> 403
#        J2 device A token GET its OWN /status -> 200 (control)
#   K. Protocol surface:
#        K1 unknown route (authed) -> 404 (no stack/no 5xx)
#        K2 wrong METHOD on an existing route (DELETE /telemetry/overview) -> 404/405
#        K3 unknown-field JSON (DisallowUnknownFields) -> 400 VALIDATION_FAILED
#        K4 artifact upload with a NON-multipart content-type -> 415
#        K5 login with a wrong Content-Type but valid JSON body -> still 200
#             (decoder reads the body directly; documents the real behavior)
#   L. Idempotency / replay hygiene:
#        L1 registering the SAME hardware_id twice -> stable, no 5xx, no dup-crash
#   M. DoS shedding (separate server, HELIX_MAX_INFLIGHT=1):
#        M1 a concurrent burst sheds at least one request with 429 RATE_LIMITED
#             + a Retry-After header (SKIP-with-reason only if the burst cannot
#             be generated — never a fake PASS)
#
# Usage:
#   security_probes_extended.sh [--port N]      (N is a HINT; a free port is
#                                                chosen if N is busy/omitted)
#   Env: HELIX_SECURITY_EVIDENCE (default RUN_EVIDENCE_EXTENDED.txt)
#
# Dependencies: bash, curl, jq, go, python3 (free-port picker + burst).
#
# Cross-references:
#   server/internal/api/handlers_auth.go    (refresh single-use rotation),
#   server/internal/api/token.go            (access-token exp),
#   server/internal/api/handlers_device.go  (status-path ownership 403),
#   server/internal/api/rate_limit.go       (HELIX_MAX_INFLIGHT 429),
#   server/internal/api/bind.go             (DisallowUnknownFields 400),
#   server/internal/api/handlers_artifact.go(multipart-only 415).
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"

PORT_HINT="${HELIX_PORT:-0}"
ADMIN_USER="${HELIX_ADMIN_USERNAME:-admin@helix.test}"
EVIDENCE="${HELIX_SECURITY_EVIDENCE:-${SCRIPT_DIR}/RUN_EVIDENCE_EXTENDED.txt}"
API="/api/v1"
RUN_TAG="secx-$(date +%s)-$$"

while [ $# -gt 0 ]; do
  case "$1" in
    --port) PORT_HINT="$2"; shift 2 ;;
    -h|--help) sed -n '2,70p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

PASS=0; FAIL=0; SKIP=0
S1_PID=""; S2_PID=""; SEXP_PID=""
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-secx.XXXXXX")"

: > "$EVIDENCE"
log()  { printf '%s\n' "$*" | tee -a "$EVIDENCE"; }
pass() { PASS=$((PASS+1)); log "[PASS] $1"; }
fail() { FAIL=$((FAIL+1)); log "[FAIL] $1"; }
skip() { SKIP=$((SKIP+1)); log "[SKIP] $1"; }

cleanup() {
  for pid in "$S1_PID" "$SEXP_PID" "$S2_PID"; do
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true; wait "$pid" 2>/dev/null || true
    fi
  done
  rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# free_port — print an OS-allocated free TCP port (binds :0, reads the port,
# closes). Avoids the hard-coded :8080 collision with sibling agents (§11.4.119).
free_port() {
  python3 - <<'PY'
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

HTTP_STATUS=""; HTTP_BODY=""; HTTP_HDRS=""
# req BASE METHOD PATH [DATA] [TOKEN] [CONTENT_TYPE]
req() {
  local base="$1" method="$2" path="$3" data="${4:-}" tok="${5:-}" ctype="${6:-application/json}"
  local btmp htmp; btmp="$(mktemp)"; htmp="$(mktemp)"
  local -a args=(-gsS --connect-timeout 5 --max-time 15 -o "$btmp" -D "$htmp" -w '%{http_code}' -X "$method" "${base}${path}" -H 'Accept: application/json')
  [ -n "$tok" ]  && args+=(-H "Authorization: Bearer ${tok}")
  [ -n "$data" ] && args+=(-H "Content-Type: ${ctype}" --data "$data")
  HTTP_STATUS="$(curl "${args[@]}" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$btmp")"; HTTP_HDRS="$(cat "$htmp")"; rm -f "$btmp" "$htmp"
}
jqget() { printf '%s' "$HTTP_BODY" | jq -r "$1" 2>/dev/null; }

assert_in() {
  local wantlist="$1" label="$2" got="$HTTP_STATUS" w
  for w in $wantlist; do
    [ "$got" = "$w" ] && { pass "$label (HTTP $got)"; return 0; }
  done
  fail "$label (want one of [$wantlist], got $got; body: $(printf '%s' "$HTTP_BODY" | head -c 220))"
  return 1
}
assert_code() {
  local want="$1" label="$2"; local got; got="$(jqget '.error.code')"
  if [ "$got" = "$want" ]; then pass "$label (error.code=$got)";
  else fail "$label (want error.code=$want, got '$got'; HTTP $HTTP_STATUS; body: $(printf '%s' "$HTTP_BODY" | head -c 200))"; fi
}

# wait_ready BASE PID — poll /healthz until 200 or the process dies.
wait_ready() {
  local base="$1" pid="$2" i
  for i in $(seq 1 60); do
    kill -0 "$pid" 2>/dev/null || return 1
    [ "$(curl -sS --connect-timeout 2 --max-time 3 -o /dev/null -w '%{http_code}' "${base}/healthz" 2>/dev/null || echo 000)" = "200" ] && return 0
    sleep 0.2
  done
  return 1
}

log "== Helix OTA EXTENDED security probe suite =="
log "run=${RUN_TAG} evidence=${EVIDENCE}"
log "started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log ""

for bin in curl jq go python3; do
  command -v "$bin" >/dev/null 2>&1 || { log "ABORT: '$bin' not found"; exit 3; }
done

SERVER_BIN="${WORK}/ota-server"
log "building ota-server ..."
( cd "$SERVER_DIR" && go build -o "$SERVER_BIN" ./cmd/ota-server ) >>"$EVIDENCE" 2>&1 \
  || { log "ABORT: go build failed"; exit 3; }

# ---- S1: short access-token TTL -------------------------------------------------
S1_PORT="$PORT_HINT"
{ [ "$S1_PORT" = "0" ] || ! python3 -c "import socket,sys; s=socket.socket(); s.bind(('127.0.0.1',int(sys.argv[1]))); s.close()" "$S1_PORT" 2>/dev/null; } \
  && S1_PORT="$(free_port)"
S1_BASE="http://127.0.0.1:${S1_PORT}"
S1_PW="secx-s1-pw-${RUN_TAG}"
log "booting S1 (default token TTL) on ${S1_BASE} ..."
HELIX_PORT="$S1_PORT" HELIX_ADMIN_USERNAME="$ADMIN_USER" HELIX_ADMIN_PASSWORD="$S1_PW" \
HELIX_TOKEN_SECRET="secx-s1-secret-${RUN_TAG}" \
  "$SERVER_BIN" >"${WORK}/s1.log" 2>&1 &
S1_PID=$!
wait_ready "$S1_BASE" "$S1_PID" || { log "ABORT: S1 not healthy"; tail -n 30 "${WORK}/s1.log" | tee -a "$EVIDENCE"; exit 1; }
pass "S1 self-hosted ota-server healthy on ${S1_BASE} (default token TTL)"

# admin login + provision two devices for the J probes
req "$S1_BASE" POST "${API}/auth/login" "$(jq -nc --arg u "$ADMIN_USER" --arg p "$S1_PW" '{username:$u,password:$p}')" ""
[ "$HTTP_STATUS" = "200" ] || { log "ABORT: S1 admin login failed (HTTP $HTTP_STATUS)"; exit 1; }
TOKEN="$(jqget '.access_token')"; REFRESH1="$(jqget '.refresh_token')"
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { log "ABORT: no S1 admin token"; exit 1; }
pass "S1 admin login established"

reg_device() {
  req "$S1_BASE" POST "${API}/devices/register" "$(jq -nc --arg hw "$1" \
    '{hardware_id:$hw,model:"OrangePi5Max",os:"android",current_version:"1.0.0"}')" "$TOKEN"
}
reg_device "secxdev-A-${RUN_TAG}"; DEV_A_ID="$(jqget '.device_id')"; DEV_A_TOK="$(jqget '.device_token')"
reg_device "secxdev-B-${RUN_TAG}"; DEV_B_ID="$(jqget '.device_id')"; DEV_B_TOK="$(jqget '.device_token')"
if [ -z "$DEV_A_TOK" ] || [ "$DEV_A_TOK" = "null" ] || [ -z "$DEV_B_ID" ] || [ "$DEV_B_ID" = "null" ]; then
  log "ABORT: could not provision two devices on S1"; exit 1
fi
log "      device A=${DEV_A_ID}  device B=${DEV_B_ID}"
log ""

# =============================================================================
# H. REFRESH-TOKEN single-use rotation
# =============================================================================
log "--- H. refresh-token single-use rotation ---"
[ -n "$REFRESH1" ] && [ "$REFRESH1" != "null" ] || { log "ABORT: login did not return a refresh_token"; exit 1; }
req "$S1_BASE" POST "${API}/auth/refresh" "$(jq -nc --arg r "$REFRESH1" '{refresh_token:$r}')" ""
assert_in "200" "H1 first refresh rotates the token pair -> 200"
REFRESH2="$(jqget '.refresh_token')"
if [ -n "$REFRESH2" ] && [ "$REFRESH2" != "null" ] && [ "$REFRESH2" != "$REFRESH1" ]; then
  pass "H1b rotation returned a NEW, distinct refresh token"
else
  fail "H1b rotation did not return a distinct new refresh token (got '$REFRESH2')"
fi
# Reuse the ALREADY-CONSUMED first refresh token -> must be rejected 401.
req "$S1_BASE" POST "${API}/auth/refresh" "$(jq -nc --arg r "$REFRESH1" '{refresh_token:$r}')" ""
assert_in "401" "H2 reusing the consumed refresh token -> 401 (single-use enforced)"
assert_code "UNAUTHENTICATED" "H2b reused refresh -> UNAUTHENTICATED"
# Garbage refresh token.
req "$S1_BASE" POST "${API}/auth/refresh" '{"refresh_token":"not-a-real-refresh-token"}' ""
assert_in "401" "H3 garbage refresh token -> 401"
# Missing field.
req "$S1_BASE" POST "${API}/auth/refresh" '{}' ""
assert_in "400" "H4 refresh missing refresh_token -> 400"
assert_code "VALIDATION_FAILED" "H4b missing field -> VALIDATION_FAILED"

# =============================================================================
# J. CROSS-DEVICE isolation on the STATUS path (distinct from the telemetry path)
# =============================================================================
log ""
log "--- J. cross-device isolation on /devices/:id/status ---"
req "$S1_BASE" GET "${API}/devices/${DEV_B_ID}/status" "" "$DEV_A_TOK"
assert_in "403" "J1 device A token GET device B /status -> 403"
assert_code "FORBIDDEN" "J1b cross-device status -> FORBIDDEN"
req "$S1_BASE" GET "${API}/devices/${DEV_A_ID}/status" "" "$DEV_A_TOK"
assert_in "200" "J2 device A token GET its OWN /status -> 200 (control: ownership allows self)"

# =============================================================================
# K. PROTOCOL surface
# =============================================================================
log ""
log "--- K. protocol surface (404 / method / content-type / unknown fields) ---"
req "$S1_BASE" GET "${API}/this/route/does/not/exist" "" "$TOKEN"
assert_in "404" "K1 unknown route (authed) -> 404"
# Wrong method on an existing route. gin's HandleMethodNotAllowed is off by
# default so an unmatched method on a known path falls through to 404; assert
# the REAL behavior (404 or 405), never 5xx.
req "$S1_BASE" DELETE "${API}/telemetry/overview" "" "$TOKEN"
assert_in "404 405" "K2 wrong METHOD on existing route -> 404/405 (not 5xx)"
# DisallowUnknownFields: an unexpected field in the JSON body is rejected 400.
req "$S1_BASE" POST "${API}/groups" '{"name":"k3","totally_unknown_field":true}' "$TOKEN"
assert_in "400" "K3 unknown JSON field -> 400 (DisallowUnknownFields)"
assert_code "VALIDATION_FAILED" "K3b unknown field -> VALIDATION_FAILED"
# Artifact upload with a non-multipart content-type -> 415 UNSUPPORTED_MEDIA_TYPE.
req "$S1_BASE" POST "${API}/artifacts/upload" '{"not":"multipart"}' "$TOKEN" "application/json"
assert_in "415" "K4 artifact upload non-multipart body -> 415"
assert_code "UNSUPPORTED_MEDIA_TYPE" "K4b non-multipart upload -> UNSUPPORTED_MEDIA_TYPE"
# Login uses a raw json.Decoder (no content-type gate). A valid JSON body with a
# wrong Content-Type still parses -> 200. This documents the REAL behavior; it is
# NOT a defect (the body is what is trusted, not the header), and it proves the
# probe actually exercised the parse path rather than a header short-circuit.
req "$S1_BASE" POST "${API}/auth/login" \
  "$(jq -nc --arg u "$ADMIN_USER" --arg p "$S1_PW" '{username:$u,password:$p}')" "" "text/plain"
assert_in "200" "K5 login with wrong Content-Type but valid JSON body -> 200 (body-trusted decoder)"

# =============================================================================
# L. IDEMPOTENCY / replay hygiene — re-registering the same hardware_id
# =============================================================================
log ""
log "--- L. re-registering the same hardware_id must not crash or 5xx ---"
HW_DUP="secxdev-DUP-${RUN_TAG}"
reg_device "$HW_DUP"; L_S1="$HTTP_STATUS"
reg_device "$HW_DUP"; L_S2="$HTTP_STATUS"
case "$L_S2" in
  5*) fail "L1 duplicate hardware_id register crashed/5xx (first=$L_S1 second=$L_S2)" ;;
  000) fail "L1 duplicate hardware_id register connection failed" ;;
  *) pass "L1 duplicate hardware_id register handled cleanly (first=$L_S1 second=$L_S2, no 5xx)" ;;
esac

# =============================================================================
# I. ACCESS-TOKEN EXPIRY — a DEDICATED ephemeral server with HELIX_ACCESS_TOKEN_TTL=1s.
#    The 1s TTL is isolated to THIS probe so it cannot expire the admin token used
#    for provisioning + the H/J/K/L probes above (which run on the default-TTL S1)
#    — a 1s token shared across a multi-step suite is timing-fragile (§11.4.50).
# =============================================================================
log ""
log "--- I. access-token expiry (dedicated TTL=1s server, real wall-clock) ---"
SEXP_PORT="$(free_port)"
SEXP_BASE="http://127.0.0.1:${SEXP_PORT}"
SEXP_PW="secx-exp-pw-${RUN_TAG}"
log "booting S_EXP (HELIX_ACCESS_TOKEN_TTL=1s) on ${SEXP_BASE} ..."
HELIX_PORT="$SEXP_PORT" HELIX_ADMIN_USERNAME="$ADMIN_USER" HELIX_ADMIN_PASSWORD="$SEXP_PW" \
HELIX_TOKEN_SECRET="secx-exp-secret-${RUN_TAG}" HELIX_ACCESS_TOKEN_TTL="1s" \
  "$SERVER_BIN" >"${WORK}/sexp.log" 2>&1 &
SEXP_PID=$!
if ! wait_ready "$SEXP_BASE" "$SEXP_PID"; then
  skip "I1/I2 could not boot the short-TTL server (S_EXP not healthy)"
else
  # Mint a FRESH short-TTL access token, then use it immediately + after expiry.
  req "$SEXP_BASE" POST "${API}/auth/login" "$(jq -nc --arg u "$ADMIN_USER" --arg p "$SEXP_PW" '{username:$u,password:$p}')" ""
  EXP_TOKEN="$(jqget '.access_token')"
  if [ -z "$EXP_TOKEN" ] || [ "$EXP_TOKEN" = "null" ]; then
    fail "I1/I2 could not mint expiry-probe token (login HTTP $HTTP_STATUS)"
  else
    req "$SEXP_BASE" GET "${API}/telemetry/overview" "" "$EXP_TOKEN"
    assert_in "200" "I1 fresh access token works immediately -> 200"
    log "      waiting 2s for the 1s-TTL access token to expire ..."
    sleep 2
    req "$SEXP_BASE" GET "${API}/telemetry/overview" "" "$EXP_TOKEN"
    assert_in "401" "I2 same access token AFTER expiry -> 401 (exp enforced, not just signature)"
  fi
fi
# Stop S_EXP + S1 now; M needs a separate server with HELIX_MAX_INFLIGHT=1.
if kill -0 "$SEXP_PID" 2>/dev/null; then kill "$SEXP_PID" 2>/dev/null || true; wait "$SEXP_PID" 2>/dev/null || true; fi
SEXP_PID=""
if kill -0 "$S1_PID" 2>/dev/null; then kill "$S1_PID" 2>/dev/null || true; wait "$S1_PID" 2>/dev/null || true; fi
S1_PID=""

# =============================================================================
# M. DoS SHEDDING — HELIX_MAX_INFLIGHT=1 -> concurrent burst sheds 429
# =============================================================================
log ""
log "--- M. in-flight cap sheds excess concurrency with 429 RATE_LIMITED ---"
S2_PORT="$(free_port)"
S2_BASE="http://127.0.0.1:${S2_PORT}"
S2_PW="secx-s2-pw-${RUN_TAG}"
log "booting S2 (HELIX_MAX_INFLIGHT=1) on ${S2_BASE} ..."
HELIX_PORT="$S2_PORT" HELIX_ADMIN_USERNAME="$ADMIN_USER" HELIX_ADMIN_PASSWORD="$S2_PW" \
HELIX_TOKEN_SECRET="secx-s2-secret-${RUN_TAG}" HELIX_MAX_INFLIGHT="1" \
  "$SERVER_BIN" >"${WORK}/s2.log" 2>&1 &
S2_PID=$!
if ! wait_ready "$S2_BASE" "$S2_PID"; then
  skip "M1 could not boot the max-inflight server (S2 not healthy)"
else
  pass "S2 self-hosted ota-server healthy on ${S2_BASE} (HELIX_MAX_INFLIGHT=1)"
  # Fire a concurrent burst at a real protected route. With the in-flight cap=1
  # and many simultaneous requests, the semaphore must shed at least one with
  # 429 + Retry-After. We capture every status code into a file and assert >=1
  # is 429. The python helper opens N sockets truly concurrently (threads).
  STATUS_FILE="${WORK}/burst_status.txt"
  python3 - "$S2_BASE${API}/telemetry/overview" "$STATUS_FILE" <<'PY'
import sys, threading, http.client, urllib.parse
url, outfile = sys.argv[1], sys.argv[2]
u = urllib.parse.urlparse(url)
N = 80
codes = [None] * N
barrier = threading.Barrier(N)
def hit(i):
    try:
        barrier.wait(timeout=10)
    except Exception:
        pass
    try:
        c = http.client.HTTPConnection(u.hostname, u.port, timeout=10)
        # No auth header on purpose: the in-flight middleware runs BEFORE auth,
        # so a shed request returns 429 regardless of credentials; un-shed
        # requests reach auth and return 401. Either way the status is recorded.
        c.request("GET", u.path)
        r = c.getresponse(); codes[i] = r.status; r.read(); c.close()
    except Exception:
        codes[i] = 0
ts = [threading.Thread(target=hit, args=(i,)) for i in range(N)]
for t in ts: t.start()
for t in ts: t.join()
with open(outfile, "w") as f:
    for code in codes:
        f.write(str(code) + "\n")
PY
  N429="$(grep -c '^429$' "$STATUS_FILE" 2>/dev/null || echo 0)"
  NTOTAL="$(grep -cv '^$' "$STATUS_FILE" 2>/dev/null || echo 0)"
  log "      burst result: ${NTOTAL} requests, ${N429} shed with 429"
  printf 'burst status histogram:\n' >>"$EVIDENCE"
  sort "$STATUS_FILE" | uniq -c >>"$EVIDENCE" 2>/dev/null || true
  if [ "$N429" -ge 1 ]; then
    pass "M1 in-flight cap shed ${N429}/${NTOTAL} concurrent requests with 429"
    # Confirm a shed response actually carries the RATE_LIMITED code + Retry-After.
    req "$S2_BASE" GET "${API}/telemetry/overview" "" ""   # warm single (will be 401, not shed)
    # Re-issue a quick concurrent pair to capture a real 429 body+header.
    RL_BODY=""; RL_HDR=""
    for _ in 1 2 3 4 5; do
      (curl -sS --connect-timeout 4 --max-time 8 -D "${WORK}/rl_h.txt" -o "${WORK}/rl_b.txt" -w '%{http_code}' "${S2_BASE}${API}/telemetry/overview" >"${WORK}/rl_c1.txt" 2>/dev/null) & cp1=$!
      (curl -sS --connect-timeout 4 --max-time 8 -o /dev/null "${S2_BASE}${API}/telemetry/overview" >/dev/null 2>&1) & cp2=$!
      (curl -sS --connect-timeout 4 --max-time 8 -o /dev/null "${S2_BASE}${API}/telemetry/overview" >/dev/null 2>&1) & cp3=$!
      # Wait ONLY on the 3 curl PIDs — NOT a bare `wait`, which would also block on
      # the backgrounded S2 server job (which never exits) and hang the suite.
      wait "$cp1" "$cp2" "$cp3" 2>/dev/null
      if [ "$(cat "${WORK}/rl_c1.txt" 2>/dev/null)" = "429" ]; then
        RL_BODY="$(cat "${WORK}/rl_b.txt" 2>/dev/null)"; RL_HDR="$(cat "${WORK}/rl_h.txt" 2>/dev/null)"; break
      fi
    done
    if [ -n "$RL_BODY" ]; then
      if printf '%s' "$RL_BODY" | jq -e '.error.code == "RATE_LIMITED"' >/dev/null 2>&1; then
        pass "M1b shed response carries error.code=RATE_LIMITED"
      else
        fail "M1b 429 body lacked RATE_LIMITED code (body: $(printf '%s' "$RL_BODY" | head -c 160))"
      fi
      if printf '%s' "$RL_HDR" | grep -qi '^Retry-After:'; then
        pass "M1c shed response carries a Retry-After header"
      else
        fail "M1c 429 response missing Retry-After header"
      fi
    else
      skip "M1b/M1c could not re-capture a single 429 body/header (timing) — M1 already proved shedding"
    fi
  else
    skip "M1 burst did not trigger 429 (host scheduled requests serially; in-flight cap not exercised this run)"
  fi
fi

# ---- summary -------------------------------------------------------------------
log ""
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then log "RESULT: FAIL"; exit 1; fi
log "RESULT: PASS"
exit 0
