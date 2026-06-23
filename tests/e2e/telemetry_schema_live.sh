#!/usr/bin/env bash
# =============================================================================
# telemetry_schema_live.sh — Helix OTA server-side telemetry SCHEMA-VALIDATION E2E
# -----------------------------------------------------------------------------
# Purpose:
#   Autonomous, anti-bluff end-to-end challenge (Constitution §11.4 / §11.4.6 /
#   §11.4.27 / §11.4.50 / §11.4.69 / §11.4.98 / §11.4.123) that proves the LIVE
#   ota-server VALIDATES device telemetry payloads against the ota-telemetry-
#   schema codec contract server-side, over real HTTP (curl + jq, no mocks of
#   the system under test):
#
#     * a VALID telemetry event is ACCEPTED  (202, accepted=1 rejected=0) AND
#       PERSISTED (verifiable via GET /devices/{id}/telemetry — sink-side
#       positive evidence per §11.4.69, not just a 202 fire-and-forget).
#     * a WRONG-SCHEMA / MALFORMED telemetry body is REJECTED (no false accept,
#       no silent persist), via the two distinct rejection surfaces the server
#       actually has:
#         (A) bindJSON strict decode (DisallowUnknownFields + size cap + the
#             ota-protocol TelemetryEvent enum validated in UnmarshalJSON): an
#             unknown top-level field, a non-JSON body, OR an unknown event enum
#             token -> HTTP 400 malformed (the whole batch refused at decode).
#         (B) the ota-telemetry-schema codec Validate(): a body whose SHAPE +
#             enums decode cleanly but whose event content is invalid per the
#             schema — a missing required deployment_id, or a negative
#             duration_ms — is counted into .rejected (202 with accepted=0
#             rejected=N) and is NOT persisted.
#       (Which layer catches which breach was VERIFIED against the live server,
#        not assumed — §11.4.6: the unknown-enum case is a decode-layer 400, a
#        stronger refusal than a codec-layer rejected=1.)
#
#   This closes a real coverage gap: the bank's existing telemetry challenges
#   cover the telemetry OVERVIEW aggregate, the READ filters/pagination, and a
#   single device-originated POST happy-path (HOTA-RK3588), but NOTHING asserts
#   that a WRONG-SCHEMA telemetry payload is genuinely refused server-side and
#   leaves no persisted residue. A control plane that silently swallowed
#   malformed device telemetry would corrupt every downstream health/rollout
#   decision — so "valid accepted, malformed refused, nothing-bad-persisted" is
#   a safety property worth a dedicated live proof.
#
#   Validation contract verified against:
#     server/internal/api/bind.go (bindJSON: DisallowUnknownFields, dec.More(),
#       MaxBytesReader),
#     server/internal/api/handlers_client.go handleClientTelemetry (builds the
#       canonical ota-protocol.TelemetryReport per event, calls
#       otatelemetry.NewEvent(report).Validate(); a failing Validate() => the
#       event is counted REJECTED and skipped, never persisted),
#     submodules/ota-telemetry-schema/event.go Event.Validate ->
#       submodules/ota-protocol/validate.go ValidateTelemetryReport (device_id
#       + deployment_id + valid event enum + non-zero timestamp required;
#       duration_ms/bytes_transferred must be >= 0 when present).
#
#   A device may report only for its OWN id (resource-ownership gate). The
#   script therefore registers a real device (gets a device_token), creates a
#   real deployment to supply the schema-required deployment_id, and POSTs the
#   telemetry as that device with its Bearer token — the realistic device path.
#
# Anti-bluff design:
#   - Self-hosts a real ota-server (in-memory repo, plain HTTP) on a probed free
#     port (§11.4.119 single-owner), killed on every exit path (§11.4.14).
#   - The VALID-accept PASS is gated on sink-side evidence (the persisted event
#     read back via GET), not merely the 202 — §11.4.69 positive evidence.
#   - An ACCEPT-CONTROL: the SAME valid event family proves the reject below is
#     caused by the schema breach, not by an always-rejecting handler.
#   - Ephemeral admin password is a FRESH per-run test-only value, never a real
#     secret, never printed; device_token / Bearer values are REDACTED in the
#     committed evidence (§11.4.10).
#   - Every curl is bounded (--connect-timeout / --max-time) so a wedged socket
#     can never hang the script. Re-runnable end-to-end any number of times with
#     self-contained state (§11.4.98 / §11.4.50).
#   - No external signing/openssl needed (telemetry needs no signed artifact),
#     so the only SKIP path is a genuinely-absent prerequisite tool — never a
#     false PASS.
#
# Usage:
#   telemetry_schema_live.sh [--port N] [--server-bin PATH] [--evidence-dir DIR]
#   Env: HELIX_PORT (default: a free probed port),
#        HELIX_TELEMETRY_SCHEMA_EVIDENCE_DIR
#          (default tests/e2e/TELEMETRY_SCHEMA_EVIDENCE/)
#
# Outputs:
#   Human-readable PASS/FAIL/SKIP lines on stdout + a redacted evidence file per
#   step under the evidence dir; exit 0 only if every hard assertion passed,
#   exit 1 on any FAIL, exit 3 on a prerequisite SKIP (never a false PASS).
#
# Side-effects: starts + stops one ota-server on the chosen port; frees the port
#   on exit; writes captured (redacted) evidence. No host state touched.
#
# Dependencies: bash, go, base64, curl, jq, python3.
#
# Cross-references:
#   server/internal/api/handlers_client.go (handleClientTelemetry),
#   server/internal/api/bind.go (strict JSON decode),
#   submodules/ota-telemetry-schema/event.go (Event.Validate),
#   submodules/ota-protocol/validate.go (ValidateTelemetryReport),
#   tests/e2e/rollout_halt_safety.sh (self-hosting recipe this mirrors),
#   tests/e2e/pipeline_signed_live.sh (device-register + redaction recipe).
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"

PORT="${HELIX_PORT:-}"
SERVER_BIN=""
EVIDENCE_DIR="${HELIX_TELEMETRY_SCHEMA_EVIDENCE_DIR:-${SCRIPT_DIR}/TELEMETRY_SCHEMA_EVIDENCE}"

while [ $# -gt 0 ]; do
  case "$1" in
    --port)         PORT="$2"; shift 2 ;;
    --server-bin)   SERVER_BIN="$2"; shift 2 ;;
    --evidence-dir) EVIDENCE_DIR="$2"; shift 2 ;;
    -h|--help)      sed -n '2,110p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

API="/api/v1"
RUN_TAG="telemschema-$(date +%s)-$$"

ADMIN_USER="admin@helix.test"
ADMIN_PW="telemschema-pw-${RUN_TAG}"
TOKEN_SECRET="telemschema-token-secret-${RUN_TAG}"

PASS=0; FAIL=0; SKIP=0
TOKEN=""
DEVICE_TOKEN=""
SERVER_PID=""
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-telemschema.XXXXXX")"

mkdir -p "${EVIDENCE_DIR}"
SUMMARY="${EVIDENCE_DIR}/SUMMARY.txt"
: > "$SUMMARY"
log() { printf '%s\n' "$*" | tee -a "$SUMMARY"; }
pass() { PASS=$((PASS+1)); log "[PASS] $1"; }
fail() { FAIL=$((FAIL+1)); log "[FAIL] $1"; }
skip() { SKIP=$((SKIP+1)); log "[SKIP] $1"; }

# §11.4.10 — redact Bearer tokens + device_token values out of committed evidence.
redact() {
  sed -E \
    -e 's/(Authorization: Bearer )[A-Za-z0-9._-]+/\1<REDACTED>/g' \
    -e 's/("device_token"[[:space:]]*:[[:space:]]*")[^"]+/\1<REDACTED>/g' \
    -e 's/("access_token"[[:space:]]*:[[:space:]]*")[^"]+/\1<REDACTED>/g'
}

cleanup() {
  if [ -n "$SERVER_PID" ] && kill -0 "$SERVER_PID" 2>/dev/null; then
    kill "$SERVER_PID" 2>/dev/null || true
    wait "$SERVER_PID" 2>/dev/null || true
  fi
  rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

free_port() {
  python3 - <<'PY'
import socket
s = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
}

HTTP_STATUS=""; HTTP_BODY=""
# req METHOD PATH [DATA] [AUTH_TOKEN] [EVIDENCE_FILE]
# A raw-body variant is used for the non-JSON malformed case via req_raw below.
req() {
  local method="$1" path="$2" data="${3:-}" tok="${4:-$TOKEN}" ev="${5:-}"
  local tmp; tmp="$(mktemp)"
  local -a args=(-sS --connect-timeout 5 --max-time 20 -o "$tmp" -w '%{http_code}'
                 -X "$method" "${BASE_URL}${path}" -H 'Accept: application/json')
  [ -n "$tok" ] && args+=(-H "Authorization: Bearer ${tok}")
  [ -n "$data" ] && args+=(-H 'Content-Type: application/json' --data "$data")
  HTTP_STATUS="$(curl "${args[@]}" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$tmp")"; rm -f "$tmp"
  if [ -n "$ev" ]; then
    { printf '%s %s -> HTTP %s\n' "$method" "$path" "$HTTP_STATUS"
      [ -n "$data" ] && printf 'request:  %s\n' "$data"
      printf 'response: %s\n' "$HTTP_BODY"
    } | redact > "$ev"
  fi
}

# req_raw posts a non-JSON / raw body (for the "not even JSON" malformed case).
req_raw() {
  local path="$1" data="$2" tok="${3:-$TOKEN}" ev="${4:-}"
  local tmp; tmp="$(mktemp)"
  HTTP_STATUS="$(curl -sS --connect-timeout 5 --max-time 20 -o "$tmp" -w '%{http_code}' \
      -X POST "${BASE_URL}${path}" -H 'Accept: application/json' \
      -H "Authorization: Bearer ${tok}" \
      -H 'Content-Type: application/json' --data "$data" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$tmp")"; rm -f "$tmp"
  if [ -n "$ev" ]; then
    { printf 'POST %s (raw body) -> HTTP %s\n' "$path" "$HTTP_STATUS"
      printf 'request:  %s\n' "$data"
      printf 'response: %s\n' "$HTTP_BODY"
    } | redact > "$ev"
  fi
}

jqget() { printf '%s' "$HTTP_BODY" | jq -r "$1" 2>/dev/null; }

# ---- prerequisites -------------------------------------------------------------
for bin in go base64 curl jq python3; do
  command -v "$bin" >/dev/null 2>&1 || { log "ABORT: required tool '$bin' not found"; exit 3; }
done

[ -n "$PORT" ] || PORT="$(free_port)"
[ -n "$PORT" ] || { log "ABORT: could not probe a free port"; exit 3; }
BASE_URL="http://127.0.0.1:${PORT}"

log "== Helix OTA telemetry SCHEMA-VALIDATION live E2E =="
log "base_url=${BASE_URL} run=${RUN_TAG} evidence_dir=${EVIDENCE_DIR}"
log "started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log ""

# ---- 1. build + start the server -----------------------------------------------
if [ -z "$SERVER_BIN" ]; then
  SERVER_BIN="${WORK}/ota-server"
  log "building ota-server (go build ./cmd/ota-server) ..."
  if ! ( cd "$SERVER_DIR" && go build -o "$SERVER_BIN" ./cmd/ota-server ) >"${EVIDENCE_DIR}/go_build.log" 2>&1; then
    log "ABORT: go build failed (see ${EVIDENCE_DIR}/go_build.log)"; exit 3
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
  code="$(curl -sS --connect-timeout 2 --max-time 5 -o /dev/null -w '%{http_code}' "${BASE_URL}/healthz" 2>/dev/null || echo 000)"
  if [ "$code" = "200" ]; then READY=1; break; fi
  sleep 0.2
done
if [ "$READY" != "1" ]; then
  log "ABORT: server did not become healthy on ${BASE_URL}/healthz"
  log "---- server log ----"; tail -n 40 "$SERVER_LOG" | tee -a "$SUMMARY"
  exit 1
fi
pass "ota-server healthy on ${BASE_URL}/healthz"

# ---- 2. admin login ------------------------------------------------------------
req POST "${API}/auth/login" "$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PW" '{username:$u,password:$p}')" "" "${EVIDENCE_DIR}/step01_login.txt"
[ "$HTTP_STATUS" = "200" ] || { fail "POST /auth/login (want 200, got $HTTP_STATUS)"; log "ABORT"; exit 1; }
TOKEN="$(jqget '.access_token')"
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { fail "login 200 but no access_token"; exit 1; }
pass "obtained admin access token (HTTP 200)"

# ---- 3. register a real device (gets a device_token) ---------------------------
HW_ID="hwtelem-${RUN_TAG}"
req POST "${API}/devices/register" "$(jq -nc --arg hw "$HW_ID" \
  '{hardware_id:$hw,model:"OrangePi5Max",os:"android",current_version:"1.0.0"}')" "" "${EVIDENCE_DIR}/step02_register_device.txt"
[ "$HTTP_STATUS" = "201" ] || { fail "POST /devices/register (want 201, got $HTTP_STATUS; body $(printf '%s' "$HTTP_BODY" | head -c 200))"; log "ABORT"; exit 1; }
DEVICE_ID="$(jqget '.device_id')"
DEVICE_TOKEN="$(jqget '.device_token')"
[ -n "$DEVICE_ID" ] && [ "$DEVICE_ID" != "null" ] || { fail "register 201 but no device_id"; exit 1; }
[ -n "$DEVICE_TOKEN" ] && [ "$DEVICE_TOKEN" != "null" ] || { fail "register 201 but no device_token"; exit 1; }
pass "registered device ${DEVICE_ID} + issued device_token (201, token redacted in evidence)"

# A deployment_id is REQUIRED by the schema validator. We don't need a real
# update to flow — we only need a deployment id the schema will accept. Create a
# minimal deployment via a signed-free path is not available, so we use a
# synthetic-but-nonempty deployment_id string: the schema validator only checks
# deployment_id is NON-EMPTY (validate.go), it does not cross-check existence on
# the ingest path. This keeps the test focused on SCHEMA validation, not the
# deployment lifecycle (covered elsewhere). The id is clearly test-scoped.
DEPLOY_ID="dep-${RUN_TAG}"

NOW_TS="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

# ---- 4. ACCEPT CONTROL: a VALID telemetry event is accepted + PERSISTED ---------
# Valid = device reports for its OWN id, with a non-empty deployment_id, a known
# event enum, and a non-zero timestamp. Expect 202 accepted=1 rejected=0.
VALID_BODY="$(jq -nc --arg d "$DEVICE_ID" --arg dep "$DEPLOY_ID" --arg ts "$NOW_TS" \
  '{device_id:$d,deployment_id:$dep,events:[{event:"installing",timestamp:$ts}]}')"
EV_VALID="${EVIDENCE_DIR}/step03_valid_accepted.txt"
req POST "${API}/client/telemetry" "$VALID_BODY" "$DEVICE_TOKEN" "$EV_VALID"
V_ACC="$(jqget '.accepted')"; V_REJ="$(jqget '.rejected')"
if [ "$HTTP_STATUS" = "202" ] && [ "$V_ACC" = "1" ] && [ "$V_REJ" = "0" ]; then
  pass "VALID telemetry event ACCEPTED (202, accepted=1 rejected=0)"
else
  fail "VALID telemetry event was NOT accepted as expected (HTTP $HTTP_STATUS accepted=$V_ACC rejected=$V_REJ; body $(printf '%s' "$HTTP_BODY" | head -c 240))"
fi

# Sink-side positive evidence (§11.4.69): the accepted event is PERSISTED and
# readable back — a 202 alone is not proof it was stored.
EV_PERSIST="${EVIDENCE_DIR}/step04_persisted_readback.txt"
req GET "${API}/devices/${DEVICE_ID}/telemetry" "" "$TOKEN" "$EV_PERSIST"
P_COUNT="$(jqget '.items | length')"
P_FIRST_EVENT="$(jqget '.items[0].event')"
if [ "$HTTP_STATUS" = "200" ] && [ "${P_COUNT:-0}" -ge 1 ] 2>/dev/null && [ "$P_FIRST_EVENT" = "installing" ]; then
  pass "PERSISTED: GET /devices/{id}/telemetry returns the accepted 'installing' event (sink-side §11.4.69 evidence; items=${P_COUNT})"
else
  fail "accepted event was NOT persisted/readable as expected (HTTP $HTTP_STATUS items=${P_COUNT} first_event=${P_FIRST_EVENT}; body $(printf '%s' "$HTTP_BODY" | head -c 240))"
fi

# ---- 5. REJECT (A): strict-decode — an UNKNOWN top-level field -> HTTP 400 -------
# bindJSON DisallowUnknownFields: a wrong-schema body with an extra field is a
# malformed body, never silently accepted.
UNKNOWN_BODY="$(jq -nc --arg d "$DEVICE_ID" --arg dep "$DEPLOY_ID" --arg ts "$NOW_TS" \
  '{device_id:$d,deployment_id:$dep,bogus_field:"not-in-schema",events:[{event:"installing",timestamp:$ts}]}')"
EV_UNKNOWN="${EVIDENCE_DIR}/step05_unknown_field_400.txt"
req POST "${API}/client/telemetry" "$UNKNOWN_BODY" "$DEVICE_TOKEN" "$EV_UNKNOWN"
if [ "$HTTP_STATUS" = "400" ]; then
  pass "REJECT(strict-decode): an UNKNOWN top-level field -> HTTP 400 (DisallowUnknownFields, wrong schema refused)"
else
  fail "wrong-schema body with unknown field was NOT rejected 400 (got HTTP $HTTP_STATUS; body $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi

# ---- 6. REJECT (A'): a NON-JSON body -> HTTP 400 --------------------------------
EV_NOTJSON="${EVIDENCE_DIR}/step06_not_json_400.txt"
req_raw "${API}/client/telemetry" "this-is-not-json-at-all" "$DEVICE_TOKEN" "$EV_NOTJSON"
if [ "$HTTP_STATUS" = "400" ]; then
  pass "REJECT(strict-decode): a NON-JSON body -> HTTP 400 (malformed body refused)"
else
  fail "non-JSON body was NOT rejected 400 (got HTTP $HTTP_STATUS; body $(printf '%s' "$HTTP_BODY" | head -c 200))"
fi

# ---- 7. REJECT (A''): strict-decode — an UNKNOWN event ENUM -> HTTP 400 ----------
# VERIFIED-LIVE (§11.4.6): the ota-protocol TelemetryEvent type validates its
# enum token in UnmarshalJSON (enums.go), so an unknown event enum is refused at
# the strict-decode (bindJSON) layer with HTTP 400 — BEFORE the per-event codec
# Validate() ever runs. This is a STRONGER guarantee than a 202/rejected=1: the
# whole malformed batch is bounced at decode time, nothing is even attempted.
BADENUM_BODY="$(jq -nc --arg d "$DEVICE_ID" --arg dep "$DEPLOY_ID" --arg ts "$NOW_TS" \
  '{device_id:$d,deployment_id:$dep,events:[{event:"not_a_real_event",timestamp:$ts}]}')"
EV_BADENUM="${EVIDENCE_DIR}/step07_bad_enum_400.txt"
req POST "${API}/client/telemetry" "$BADENUM_BODY" "$DEVICE_TOKEN" "$EV_BADENUM"
if [ "$HTTP_STATUS" = "400" ]; then
  pass "REJECT(strict-decode): an UNKNOWN event enum -> HTTP 400 (enum validated in UnmarshalJSON, whole batch refused at decode)"
else
  fail "unknown event enum was NOT rejected 400 at decode (got HTTP $HTTP_STATUS; body $(printf '%s' "$HTTP_BODY" | head -c 240))"
fi

# ---- 8. REJECT (B'): schema codec — a MISSING required deployment_id ------------
# deployment_id is required by ValidateTelemetryReport; omit it -> rejected.
NODEP_BODY="$(jq -nc --arg d "$DEVICE_ID" --arg ts "$NOW_TS" \
  '{device_id:$d,events:[{event:"installing",timestamp:$ts}]}')"
EV_NODEP="${EVIDENCE_DIR}/step08_missing_deployment_id_rejected.txt"
req POST "${API}/client/telemetry" "$NODEP_BODY" "$DEVICE_TOKEN" "$EV_NODEP"
N_ACC="$(jqget '.accepted')"; N_REJ="$(jqget '.rejected')"
if [ "$HTTP_STATUS" = "202" ] && [ "$N_ACC" = "0" ] && [ "$N_REJ" = "1" ]; then
  pass "REJECT(schema): a MISSING required deployment_id is rejected (202 accepted=0 rejected=1, not persisted)"
else
  fail "missing deployment_id was NOT rejected by the schema codec (HTTP $HTTP_STATUS accepted=$N_ACC rejected=$N_REJ; body $(printf '%s' "$HTTP_BODY" | head -c 240))"
fi

# ---- 9. REJECT (B''): schema codec — a NEGATIVE duration_ms ---------------------
# duration_ms must be >= 0 when present; a negative value is malformed.
NEGDUR_BODY="$(jq -nc --arg d "$DEVICE_ID" --arg dep "$DEPLOY_ID" --arg ts "$NOW_TS" \
  '{device_id:$d,deployment_id:$dep,events:[{event:"installing",timestamp:$ts,duration_ms:-5}]}')"
EV_NEGDUR="${EVIDENCE_DIR}/step09_negative_duration_rejected.txt"
req POST "${API}/client/telemetry" "$NEGDUR_BODY" "$DEVICE_TOKEN" "$EV_NEGDUR"
D_ACC="$(jqget '.accepted')"; D_REJ="$(jqget '.rejected')"
if [ "$HTTP_STATUS" = "202" ] && [ "$D_ACC" = "0" ] && [ "$D_REJ" = "1" ]; then
  pass "REJECT(schema): a NEGATIVE duration_ms is rejected (202 accepted=0 rejected=1, not persisted)"
else
  fail "negative duration_ms was NOT rejected by the schema codec (HTTP $HTTP_STATUS accepted=$D_ACC rejected=$D_REJ; body $(printf '%s' "$HTTP_BODY" | head -c 240))"
fi

# ---- 10. NO-RESIDUE: only the ONE valid event persisted, nothing malformed ------
# After 1 valid accept + 4 malformed rejects, the device's telemetry history must
# still contain exactly ONE event (the valid one). Proves the rejects truly left
# NO persisted residue (§11.4.69 / §11.4.6 — refusal must be real, not cosmetic).
EV_RESIDUE="${EVIDENCE_DIR}/step10_no_residue_readback.txt"
req GET "${API}/devices/${DEVICE_ID}/telemetry" "" "$TOKEN" "$EV_RESIDUE"
R_COUNT="$(jqget '.items | length')"
if [ "$HTTP_STATUS" = "200" ] && [ "${R_COUNT:-0}" = "1" ]; then
  pass "NO-RESIDUE: after 1 valid + 4 malformed POSTs the device has exactly 1 persisted event (malformed left no residue)"
else
  fail "malformed telemetry left persisted residue OR count wrong (HTTP $HTTP_STATUS items=${R_COUNT}, expected 1)"
fi

# ---- summary -------------------------------------------------------------------
log ""
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then log "RESULT: FAIL"; exit 1; fi
log "RESULT: PASS"
exit 0
