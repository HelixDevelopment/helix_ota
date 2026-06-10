#!/usr/bin/env bash
# =============================================================================
# recall_telemetry_probes.sh — Helix OTA RECALL + TELEMETRY security probes
# -----------------------------------------------------------------------------
# Purpose:
#   Black-box (anti-bluff) security probes for two surfaces NOT covered by the
#   sibling suites security_probes.sh / security_probes_filters.sh:
#     - POST /api/v1/deployments/{id}/recall        (rollback control)
#     - GET  /api/v1/devices/{id}/telemetry         (ownership + filter inputs)
#   Every probe boots a REAL ota-server over real HTTP and HARD-asserts the
#   ACTUAL status code returned by a real response (Constitution §11.4 /
#   §11.4.27 / §11.4.123). A genuine defect FAILs — nothing is papered over; a
#   script-internal crash that FAILs would itself be a §11.4.1 FAIL-bluff, so
#   curl globbing is disabled (-g) and the harness mirrors the proven sibling
#   suites exactly. This is a SEPARATE script (not appended to a known-green
#   file) per §11.4.84 working-tree quiescence spirit.
#
# IMPORTANT — expected status codes are the REAL handler behaviour, read from
#   source (§11.4.6 no-guessing), NOT assumed. In particular the recall +
#   telemetry handlers use respondValidation(), which returns HTTP 400
#   (CodeValidationFailed) — NOT 422. See the cross-references below.
#
# Probes (each HARD-asserted against the live server):
#   R. RECALL AUTHZ
#      R1 unauthenticated POST recall                       -> 401
#      R2 device-role token POST recall (operator/admin)    -> 403
#      R3 admin (operator+admin) token POST recall on a real
#         deployment-with-release fixture                   -> 201 (allowed)
#   V. RECALL VALIDATION (needs a real deployment WITH a current release; only
#      reachable when openssl ed25519 signing is available to mint a signed
#      artifact -> release -> deployment fixture; otherwise SKIP-with-reason,
#      never a false PASS, §11.4.3)
#      V1 recall body missing to_release_id                 -> 400 (VALIDATION)
#      V2 recall unknown to_release_id (deployment exists)   -> 404 (NOT_FOUND)
#      V3 recall unknown deployment id (admin token)         -> 404 (NOT_FOUND)
#   O. TELEMETRY OWNERSHIP
#      O1 device A token GET device B telemetry              -> 403
#      O2 device A token GET its OWN telemetry               -> 200
#      O3 privileged (admin) token GET ANY device telemetry  -> 200
#   F. TELEMETRY FILTER INPUT VALIDATION (handler respondValidation -> 400)
#      F1 ?event=<not-a-known-event>                         -> 400
#      F2 ?since=<not-RFC3339>                               -> 400
#      F3 ?limit=0 / 999 / -1   (out of [1,200])             -> 400 (each)
#      F4 ?cursor=<malformed>                                -> 400
#      (control) F5 ?event=success&limit=10&cursor=0 valid   -> 200
#   T. TRUST BOUNDARY
#      T1 recall accepts ONLY {to_release_id,reason} via the project's STRICT
#         bindJSON (DisallowUnknownFields, bind.go); an injected public_key /
#         signing-material field is REJECTED 400 ("malformed recall body") on a
#         known deployment — a stronger boundary than "ignored". The endpoint has
#         no key path (handlers_recall.go has no resolvePublicKey), so a request
#         can neither inject a trusted key nor smuggle unknown fields. (On an
#         unknown deployment the 404 lookup precedes bind, so that path is 404.)
#      T1b the injected fields never crash the server (no 5xx).
#
# Usage:
#   recall_telemetry_probes.sh [--port N] [--external --base-url URL --password PW]
#   Env: HELIX_PORT (default: a per-PID ephemeral port), HELIX_SECURITY_EVIDENCE
#        (default RUN_EVIDENCE_RECALL_TELEMETRY.txt next to this script).
#
# Dependencies: bash, curl, jq, go (self-host). Optional: openssl(>=3 ed25519),
#   xxd, base64, python3 (only the V.* recall-validation fixture needs these;
#   absent => V.* SKIP-with-reason, the rest still run).
#
# Cross-references:
#   server/internal/api/handlers_recall.go     (handleRecall: 404/400/201, NO key path)
#   server/internal/api/handlers_telemetry.go  (handleDeviceTelemetry: ownership 403 +
#                                                event/since/limit/cursor -> 400)
#   server/internal/api/server.go              (route role wiring: recall=operator|admin,
#                                                telemetry=viewer|operator|admin|device)
#   server/internal/api/errors.go              (respondValidation == HTTP 400)
#   server/internal/api/middleware.go          (requireRole: unauth 401, wrong role 403)
#   tests/security/security_probes.sh          (sibling base suite — same harness)
#   tests/e2e/pipeline_signed.sh               (signed-artifact fixture recipe reused)
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"

# Default to a per-PID ephemeral port in the dynamic/private range so parallel
# suites (8080 base, 8097 filters) never collide. Overridable via --port / env.
DEFAULT_PORT=$(( 20000 + ($$ % 20000) ))
PORT="${HELIX_PORT:-$DEFAULT_PORT}"
EXTERNAL=0
BASE_URL=""
ADMIN_USER="${HELIX_ADMIN_USERNAME:-admin@helix.test}"
ADMIN_PW="${HELIX_ADMIN_PASSWORD:-}"
EVIDENCE="${HELIX_SECURITY_EVIDENCE:-${SCRIPT_DIR}/RUN_EVIDENCE_RECALL_TELEMETRY.txt}"
API="/api/v1"
RUN_TAG="secrt-$(date +%s)-$$"

while [ $# -gt 0 ]; do
  case "$1" in
    --port)      PORT="$2"; shift 2 ;;
    --external)  EXTERNAL=1; shift ;;
    --base-url)  BASE_URL="$2"; shift 2 ;;
    --username)  ADMIN_USER="$2"; shift 2 ;;
    --password)  ADMIN_PW="$2"; shift 2 ;;
    -h|--help)   sed -n '2,60p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

[ -z "$BASE_URL" ] && BASE_URL="http://127.0.0.1:${PORT}"
BASE_URL="${BASE_URL%/}"

PASS=0; FAIL=0; SKIP=0
TOKEN=""; SERVER_PID=""
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-secrt.XXXXXX")"

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

log "== Helix OTA recall + telemetry security probe suite =="
log "base_url=${BASE_URL} run=${RUN_TAG} evidence=${EVIDENCE}"
log "started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log ""

for bin in curl jq; do command -v "$bin" >/dev/null 2>&1 || { log "ABORT: '$bin' not found"; exit 3; }; done

# ---- ephemeral ed25519 signing (optional — only the V.* fixture needs it) ------
# When openssl(>=3 ed25519) + xxd + base64 + python3 are present we boot the
# server WITH a configured pubkey and mint a real signed artifact->release->
# deployment fixture so the recall VALIDATION probes (V1/V2) drive a deployment
# that actually has a current release. If signing is unavailable we boot WITHOUT
# a pubkey and SKIP V1/V2 with the exact reason (§11.4.3) — never a false PASS.
SIGN_OK=0
PRIV_PEM="${WORK}/artifact_priv.pem"
PUBKEY_B64=""
if [ "$EXTERNAL" = "0" ] \
   && command -v openssl >/dev/null 2>&1 \
   && command -v xxd >/dev/null 2>&1 \
   && command -v base64 >/dev/null 2>&1 \
   && command -v python3 >/dev/null 2>&1; then
  if openssl genpkey -algorithm ed25519 -out "$PRIV_PEM" 2>/dev/null; then
    PUBKEY_B64="$(openssl pkey -in "$PRIV_PEM" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 | tr -d '\n')"
    PUBLEN="$(openssl pkey -in "$PRIV_PEM" -pubout -outform DER 2>/dev/null | tail -c 32 | wc -c | tr -d ' ')"
    [ "$PUBLEN" = "32" ] && SIGN_OK=1
  fi
fi

# sign_digest_hex <digest-hex> -> base64 detached ed25519 sig over raw digest bytes
sign_digest_hex() {
  local digest_hex="$1" rawf sigf
  rawf="$(mktemp "${WORK}/digest.XXXXXX")"; sigf="$(mktemp "${WORK}/sig.XXXXXX")"
  printf '%s' "$digest_hex" | xxd -r -p > "$rawf"
  if ! openssl pkeyutl -sign -inkey "$PRIV_PEM" -rawin -in "$rawf" -out "$sigf" 2>/dev/null; then
    rm -f "$rawf" "$sigf"; return 1
  fi
  base64 < "$sigf" | tr -d '\n'
  rm -f "$rawf" "$sigf"
}
build_zip_stored() {
  local out="$1" payload="$2"
  PAYLOAD="$payload" OUT="$out" python3 - <<'PY'
import os, zipfile
out = os.environ["OUT"]; payload = os.environ["PAYLOAD"].encode()
with zipfile.ZipFile(out, "w", compression=zipfile.ZIP_STORED) as z:
    zi = zipfile.ZipInfo("payload.bin"); zi.compress_type = zipfile.ZIP_STORED
    z.writestr(zi, payload)
PY
}
sha256_hex_of_file() { openssl dgst -sha256 -binary "$1" | xxd -p -c256 | tr -d '\n'; }
# upload_signed <zip> <version>  -> sets HTTP_STATUS/HTTP_BODY (uses $TOKEN)
upload_signed() {
  local zip="$1" version="$2" digest sig file_size meta tmp
  digest="$(sha256_hex_of_file "$zip")"
  sig="$(sign_digest_hex "$digest")" || { HTTP_STATUS="SIGN_FAIL"; HTTP_BODY="openssl ed25519 sign failed"; return 1; }
  file_size="$(wc -c < "$zip" | tr -d ' ')"
  meta="$(jq -nc --arg sha "$digest" --arg sig "$sig" --arg ver "$version" \
    --arg os "android" --arg tm "OrangePi5Max" \
    --arg fh "$(printf 'file-hash' | base64 | tr -d '\n')" --argjson fs "$file_size" \
    --arg mh "$(printf 'meta-hash' | base64 | tr -d '\n')" --argjson ms 64 \
    '{sha256:$sha,signature:$sig,version:$ver,os:$os,target_model:$tm,
      file_hash:$fh,file_size:$fs,metadata_hash:$mh,metadata_size:$ms}')"
  tmp="$(mktemp)"
  HTTP_STATUS="$(curl -sS -o "$tmp" -w '%{http_code}' -X POST "${BASE_URL}${API}/artifacts/upload" \
      -H "Authorization: Bearer ${TOKEN}" \
      -F "file=@${zip};type=application/zip;filename=ota.zip" \
      -F "metadata=${meta};type=application/json" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$tmp")"; rm -f "$tmp"
}

# ---- boot a self-hosted server (unless --external) ----------------------------
if [ "$EXTERNAL" = "0" ]; then
  command -v go >/dev/null 2>&1 || { log "ABORT: go not found (need it to self-host; or use --external)"; exit 3; }
  ADMIN_PW="selfhost-pw-${RUN_TAG}"
  SERVER_BIN="${WORK}/ota-server"
  log "building ota-server ..."
  ( cd "$SERVER_DIR" && go build -o "$SERVER_BIN" ./cmd/ota-server ) >>"$EVIDENCE" 2>&1 \
    || { log "ABORT: go build failed"; exit 3; }
  if [ "$SIGN_OK" = "1" ]; then
    log "ed25519 signing available — booting WITH HELIX_ARTIFACT_PUBKEY (recall-validation fixture enabled)"
    HELIX_PORT="$PORT" HELIX_ADMIN_USERNAME="$ADMIN_USER" HELIX_ADMIN_PASSWORD="$ADMIN_PW" \
    HELIX_TOKEN_SECRET="secrt-token-secret-${RUN_TAG}" HELIX_ARTIFACT_PUBKEY="$PUBKEY_B64" \
      "$SERVER_BIN" >"${WORK}/server.log" 2>&1 &
  else
    log "ed25519 signing unavailable — booting WITHOUT pubkey (recall-validation V1/V2 will SKIP-with-reason)"
    HELIX_PORT="$PORT" HELIX_ADMIN_USERNAME="$ADMIN_USER" HELIX_ADMIN_PASSWORD="$ADMIN_PW" \
    HELIX_TOKEN_SECRET="secrt-token-secret-${RUN_TAG}" \
      "$SERVER_BIN" >"${WORK}/server.log" 2>&1 &
  fi
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
  SIGN_OK=0
  log "external mode — recall-validation V1/V2 require a key-configured server + signed fixture; will SKIP unless a pre-seeded deployment exists"
  req GET "/healthz"; [ "$HTTP_STATUS" = "200" ] || { log "ABORT: external server not reachable"; exit 1; }
  pass "external ota-server reachable on ${BASE_URL}"
fi

# ---- obtain an admin token (admin+operator+viewer; used for privileged probes
#      and to provision fixtures) ------------------------------------------------
req POST "${API}/auth/login" "$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PW" '{username:$u,password:$p}')" ""
[ "$HTTP_STATUS" = "200" ] || { log "ABORT: admin login failed (HTTP $HTTP_STATUS)"; exit 1; }
TOKEN="$(jqget '.access_token')"
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { log "ABORT: no admin token"; exit 1; }
pass "admin login established (admin+operator+viewer)"

# Provision TWO devices for the telemetry ownership probes.
reg_device() {
  local hw="$1"
  req POST "${API}/devices/register" "$(jq -nc --arg hw "$hw" \
    '{hardware_id:$hw,model:"OrangePi5Max",os:"android",current_version:"1.0.0"}')" "$TOKEN"
}
reg_device "secrt-A-${RUN_TAG}"
DEV_A_ID="$(jqget '.device_id')"; DEV_A_TOK="$(jqget '.device_token')"
reg_device "secrt-B-${RUN_TAG}"
DEV_B_ID="$(jqget '.device_id')"; DEV_B_TOK="$(jqget '.device_token')"
if [ -z "$DEV_A_TOK" ] || [ "$DEV_A_TOK" = "null" ] || [ -z "$DEV_B_TOK" ] || [ "$DEV_B_TOK" = "null" ]; then
  log "ABORT: could not provision two device tokens"; exit 1
fi
log "      device A=${DEV_A_ID}  device B=${DEV_B_ID}"
log ""

# ---- (optional) build a real deployment-with-release fixture for V1/V2 ---------
# Needs the signed-artifact recipe: upload base(1.0.0)+target(1.1.0) -> two
# releases -> deploy the TARGET release. The created deployment has a current
# release, so a recall with a missing/unknown to_release_id reaches the handler
# VALIDATION/NOT_FOUND branches (rather than the no-current-release branch).
DEP_WITH_RELEASE=""; BASE_REL_ID=""
if [ "$SIGN_OK" = "1" ]; then
  log "--- provisioning signed deployment-with-release fixture (for V1/V2) ---"
  BASE_ZIP="${WORK}/base.zip";   build_zip_stored "$BASE_ZIP"   "BASE v1.0.0 ${RUN_TAG}"
  TARGET_ZIP="${WORK}/target.zip"; build_zip_stored "$TARGET_ZIP" "TARGET v1.1.0 ${RUN_TAG}"
  upload_signed "$BASE_ZIP" "1.0.0"
  if [ "$HTTP_STATUS" = "201" ]; then
    BASE_ART_ID="$(jqget '.artifact_id')"
    upload_signed "$TARGET_ZIP" "1.1.0"
    TARGET_ART_ID="$(jqget '.artifact_id')"
    if [ "$HTTP_STATUS" = "201" ] && [ -n "$BASE_ART_ID" ] && [ "$BASE_ART_ID" != "null" ] \
       && [ -n "$TARGET_ART_ID" ] && [ "$TARGET_ART_ID" != "null" ]; then
      req POST "${API}/releases" "$(jq -nc --arg a "$BASE_ART_ID" \
        '{artifact_id:$a,version:"1.0.0",os:"android",target_model:"OrangePi5Max",notes:"base"}')" "$TOKEN"
      BASE_REL_ID="$(jqget '.release_id')"
      req POST "${API}/releases" "$(jq -nc --arg a "$TARGET_ART_ID" \
        '{artifact_id:$a,version:"1.1.0",os:"android",target_model:"OrangePi5Max",notes:"target"}')" "$TOKEN"
      TARGET_REL_ID="$(jqget '.release_id')"
      if [ -n "$TARGET_REL_ID" ] && [ "$TARGET_REL_ID" != "null" ]; then
        req POST "${API}/deployments" "$(jq -nc --arg r "$TARGET_REL_ID" '{release_id:$r,strategy:"all-targets"}')" "$TOKEN"
        if [ "$HTTP_STATUS" = "201" ]; then
          DEP_WITH_RELEASE="$(jqget '.deployment_id')"
          pass "signed fixture ready: deployment ${DEP_WITH_RELEASE} on release ${TARGET_REL_ID} (prior=${BASE_REL_ID})"
        else
          log "      fixture deployment create did not 201 (HTTP $HTTP_STATUS) — V1/V2 will SKIP"
        fi
      fi
    else
      log "      fixture artifact upload did not 201 — V1/V2 will SKIP"
    fi
  elif [ "$HTTP_STATUS" = "SIGN_FAIL" ]; then
    log "      openssl ed25519 sign failed at runtime — V1/V2 will SKIP"
  else
    log "      base artifact upload did not 201 (HTTP $HTTP_STATUS) — V1/V2 will SKIP"
  fi
  log ""
fi

# =============================================================================
# R. RECALL AUTHZ
# =============================================================================
log "--- R. recall authz: only operator/admin may POST recall ---"
# A nonexistent deployment id is fine for the AUTHZ probes: requireRole runs
# BEFORE the handler, so unauth->401 and wrong-role->403 regardless of lookup.
RECALL_PATH="${API}/deployments/nope-${RUN_TAG}/recall"
req POST "$RECALL_PATH" "$(jq -nc '{to_release_id:"x"}')" ""
assert_in "401" "R1 unauthenticated POST recall -> 401"
req POST "$RECALL_PATH" "$(jq -nc '{to_release_id:"x"}')" "$DEV_A_TOK"
assert_in "403" "R2 device-role token POST recall (operator/admin only) -> 403"
# Control: the device token IS valid on its own allowed route, proving R2 is RBAC
# enforcement, not a dead/invalid token.
req GET "${API}/client/update" "" "$DEV_A_TOK"
assert_in "200 204" "R2b device token GET /client/update is ACCEPTED (token valid; R2 is RBAC)"
# R3: an operator/admin (the admin token carries operator) IS allowed — proven
# only on a real deployment-with-release fixture (else SKIP, never fake 201).
if [ -n "$DEP_WITH_RELEASE" ] && [ -n "$BASE_REL_ID" ] && [ "$BASE_REL_ID" != "null" ]; then
  req POST "${API}/deployments/${DEP_WITH_RELEASE}/recall" \
    "$(jq -nc --arg r "$BASE_REL_ID" '{to_release_id:$r,reason:"authz-allowed probe"}')" "$TOKEN"
  assert_in "201" "R3 operator/admin token POST recall (valid) -> 201 (role allowed)"
else
  skip "R3 operator/admin recall-allowed (no signed deployment fixture; needs openssl ed25519 + pubkey-configured server)"
fi

# =============================================================================
# V. RECALL VALIDATION  (respondValidation -> 400 ; not-found -> 404)
# =============================================================================
log ""
log "--- V. recall validation: missing/unknown ids (real handler codes) ---"
if [ -n "$DEP_WITH_RELEASE" ]; then
  # V1: missing to_release_id on a deployment WITH a current release -> 400.
  req POST "${API}/deployments/${DEP_WITH_RELEASE}/recall" "$(jq -nc '{reason:"missing target"}')" "$TOKEN"
  assert_in "400" "V1 recall missing to_release_id -> 400 (VALIDATION)"
  assert_not5xx "V1b missing to_release_id did not crash"
  # V2: unknown to_release_id (deployment exists, target release does not) -> 404.
  req POST "${API}/deployments/${DEP_WITH_RELEASE}/recall" \
    "$(jq -nc --arg r "rel-does-not-exist-${RUN_TAG}" '{to_release_id:$r}')" "$TOKEN"
  assert_in "404" "V2 recall unknown to_release_id -> 404 (NOT_FOUND target release)"
  assert_not5xx "V2b unknown to_release_id did not crash"
else
  skip "V1 recall missing to_release_id (no signed deployment fixture available)"
  skip "V2 recall unknown to_release_id (no signed deployment fixture available)"
fi
# V3: unknown DEPLOYMENT id with an operator/admin token -> 404 (handler lookup
# fails before body validation). Needs no fixture — only an authed operator.
req POST "${API}/deployments/unknown-dep-${RUN_TAG}/recall" \
  "$(jq -nc '{to_release_id:"whatever"}')" "$TOKEN"
assert_in "404" "V3 recall unknown deployment id (authed operator) -> 404 (NOT_FOUND deployment)"
assert_not5xx "V3b unknown deployment id did not crash"

# =============================================================================
# O. TELEMETRY OWNERSHIP
# =============================================================================
log ""
log "--- O. telemetry ownership: a device reads only its OWN telemetry ---"
req GET "${API}/devices/${DEV_B_ID}/telemetry" "" "$DEV_A_TOK"
assert_in "403" "O1 device A token GET device B telemetry -> 403"
req GET "${API}/devices/${DEV_A_ID}/telemetry" "" "$DEV_A_TOK"
assert_in "200" "O2 device A token GET its OWN telemetry -> 200"
# O3: a privileged (viewer/operator/admin) token may read ANY device — the admin
# token carries viewer, so reading device B (not its own) is allowed.
req GET "${API}/devices/${DEV_B_ID}/telemetry" "" "$TOKEN"
assert_in "200" "O3 privileged (admin/viewer) token GET ANY device telemetry -> 200"

# =============================================================================
# F. TELEMETRY FILTER INPUT VALIDATION  (respondValidation -> 400)
# =============================================================================
log ""
log "--- F. telemetry filter validation: bad inputs -> 400 (real handler codes) ---"
# Drive all filter probes against device A's OWN telemetry (ownership satisfied,
# so the 400s are filter validation, not the 403 ownership branch).
TBASE="${API}/devices/${DEV_A_ID}/telemetry"
req GET "${TBASE}?event=not-a-real-event" "" "$DEV_A_TOK"
assert_in "400" "F1 unknown ?event -> 400 (closed-set validation)"
assert_not5xx "F1b unknown ?event did not crash"
req GET "${TBASE}?since=not-a-timestamp" "" "$DEV_A_TOK"
assert_in "400" "F2 non-RFC3339 ?since -> 400"
assert_not5xx "F2b non-RFC3339 ?since did not crash"
req GET "${TBASE}?limit=0" "" "$DEV_A_TOK"
assert_in "400" "F3a ?limit=0 (below [1,200]) -> 400"
req GET "${TBASE}?limit=999" "" "$DEV_A_TOK"
assert_in "400" "F3b ?limit=999 (above [1,200]) -> 400"
req GET "${TBASE}?limit=-1" "" "$DEV_A_TOK"
assert_in "400" "F3c ?limit=-1 (negative) -> 400"
req GET "${TBASE}?cursor=not-a-number" "" "$DEV_A_TOK"
assert_in "400" "F4 malformed ?cursor -> 400"
assert_not5xx "F4b malformed ?cursor did not crash"
# Control: a fully-valid filter set is accepted (200) — proves F1-F4 are
# validation rejections, not a route that 400s everything.
req GET "${TBASE}?event=success&limit=10&cursor=0" "" "$DEV_A_TOK"
assert_in "200" "F5 valid ?event=success&limit=10&cursor=0 -> 200 (control)"

# =============================================================================
# T. TRUST BOUNDARY — recall never sources signing material from the request
# =============================================================================
log ""
log "--- T. trust boundary: recall has NO request-supplied key path ---"
# handlers_recall.go binds ONLY {to_release_id, reason} via the project's strict
# bindJSON (json.Decoder + DisallowUnknownFields, see server/internal/api/bind.go);
# there is no resolvePublicKey / signature path. Extra attacker fields
# (public_key, signature, verification_key) are therefore REJECTED outright with
# 400 "malformed recall body" — a STRONGER trust boundary than silently ignoring
# them: a request can neither inject a trusted key NOR smuggle unexpected fields.
# Handler order is GetDeployment(404 if absent) BEFORE bindJSON(400 on unknown
# fields), so: fixture present (known deployment) => 400 (strict bind rejects the
# injected fields); fixture absent (unknown deployment) => 404 (lookup precedes
# bind). Either way the request-supplied key is never sourced, and no 5xx. The
# clean-body discrimination (a well-formed recall => 201) is proven by R3 above.
INJ_RECALL='{"to_release_id":"%TARGET%","reason":"trust-boundary probe","public_key":"QUFBQQ==","signature":"YXR0YWNrZXItc2ln","verification_key":"injected"}'
if [ -n "$DEP_WITH_RELEASE" ] && [ -n "$BASE_REL_ID" ] && [ "$BASE_REL_ID" != "null" ]; then
  BODY="${INJ_RECALL/\%TARGET\%/$BASE_REL_ID}"
  req POST "${API}/deployments/${DEP_WITH_RELEASE}/recall" "$BODY" "$TOKEN"
  # Known deployment + injected attacker fields => strict bindJSON rejects the
  # unknown fields with 400 (malformed body). The request-supplied public_key/
  # signature is never sourced; recall has no signed-upload path. Stronger than
  # "ignored": the whole request is refused. (Clean-body => 201 proven by R3.)
  assert_in "400" "T1 recall with injected public_key/signature fields is REJECTED 400 (strict bind; key never sourced)"
  assert_not5xx "T1b injected signing fields did not crash"
else
  # Fixture absent: prove the injected fields do not alter the NOT_FOUND outcome
  # on an unknown deployment (handler lookup precedes any field use) and no 5xx.
  BODY="${INJ_RECALL/\%TARGET\%/some-release}"
  req POST "${API}/deployments/unknown-dep-tb-${RUN_TAG}/recall" "$BODY" "$TOKEN"
  assert_in "404" "T1 recall+injected signing fields on unknown deployment -> 404 (fields ignored; no key path)"
  assert_not5xx "T1b injected signing fields did not crash"
fi

# ---- summary -------------------------------------------------------------------
log ""
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then log "RESULT: FAIL"; exit 1; fi
log "RESULT: PASS"
exit 0
