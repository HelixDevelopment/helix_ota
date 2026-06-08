#!/usr/bin/env bash
# =============================================================================
# pipeline_signed.sh — Helix OTA full SIGNED artifact-pipeline E2E (anti-bluff)
# -----------------------------------------------------------------------------
# Purpose:
#   Close the artifact-pipeline SKIP in challenge_operational.sh for REAL
#   (Constitution §11.4 / §11.4.27 / §11.4.123). This script self-hosts a live
#   `ota-server` configured with an EPHEMERAL ed25519 public key, then builds
#   a genuinely signed OTA artifact whose signature reproduces the server's
#   exact validation contract and drives the COMPLETE happy path:
#
#     upload (signed) -> 201
#       base v1.0.0 + target v1.1.0 (two real signed artifacts)
#     release         -> 201   (base + target)
#     deploy target   -> 201
#     rollout create  -> 201 ; get -> 200 ; evaluate -> 200 (decision action)
#     register delta  -> 201   (base_artifact -> target_artifact)
#     client/update   -> 200   (device on 1.0.0 sees the 1.1.0 update
#                               AND the registered delta in .delta)
#
#   Every assertion is made against the live HTTP server with real status codes
#   AND real JSON bodies. A 422 SIGNATURE_INVALID on upload would HARD-FAIL the
#   challenge — there is no false PASS. If openssl ed25519 cannot reproduce the
#   server's scheme the upload stage reports the EXACT reason (§11.4.6) instead
#   of fabricating a 201.
#
# Signing contract (verified against ota-artifact-validator@v0.1.0 stages.go
# ValidateHash + ValidateSignature, and server testutil_test.go validMeta):
#   sha256_hex = lowercase-hex SHA-256 over the WHOLE ZIP file bytes
#   signature  = base64( ed25519.Sign( priv, hex_decode(sha256_hex) ) )
#                i.e. ed25519 signs the raw 32 DIGEST bytes, NOT the file.
#   pubkey     = base64( last 32 raw bytes of the DER SubjectPublicKeyInfo )
#                supplied to the server via HELIX_ARTIFACT_PUBKEY.
#   payload_properties: file_hash / file_size / metadata_hash / metadata_size
#                (AOSP-style) carried in the metadata part.
#
# Keys are EPHEMERAL: generated into a mktemp dir that is .gitignore-excluded
# and rm -rf'd on exit. They are NEVER committed (§11.4.10).
#
# Usage:
#   pipeline_signed.sh [--port N] [--server-bin PATH]
#   Env: HELIX_PORT (default 8080), HELIX_PIPELINE_EVIDENCE (default
#        tests/e2e/PIPELINE_EVIDENCE.txt)
#
# Side-effects: starts + stops one ota-server on the chosen port; frees the port
#   on exit; writes captured evidence. No host state touched.
#
# Dependencies: bash, go, openssl(>=3 ed25519), xxd, base64, curl, jq, zip OR
#   python3 (for the ZIP_STORED archive; we use python3's zipfile ZIP_STORED).
#
# Cross-references:
#   server/internal/api/handlers_artifact.go (resolvePublicKey / S1..S6),
#   server/internal/api/testutil_test.go (validMeta signing recipe),
#   server/internal/config/config.go (HELIX_ARTIFACT_PUBKEY),
#   ota-artifact-validator@v0.1.0/stages.go (ValidateHash/ValidateSignature),
#   tests/e2e/challenge_operational.sh (the SKIP this closes).
# =============================================================================
set -u
set -o pipefail

# ---- repo geometry -------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"

PORT="${HELIX_PORT:-8080}"
SERVER_BIN=""
EVIDENCE="${HELIX_PIPELINE_EVIDENCE:-${SCRIPT_DIR}/PIPELINE_EVIDENCE.txt}"

while [ $# -gt 0 ]; do
  case "$1" in
    --port)       PORT="$2"; shift 2 ;;
    --server-bin) SERVER_BIN="$2"; shift 2 ;;
    -h|--help)    sed -n '2,60p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

BASE_URL="http://127.0.0.1:${PORT}"
API="/api/v1"
RUN_TAG="pipe-$(date +%s)-$$"

# Ephemeral admin/token secrets for THIS run only (never committed).
ADMIN_USER="admin@helix.test"
ADMIN_PW="pipeline-pw-${RUN_TAG}"
TOKEN_SECRET="pipeline-token-secret-${RUN_TAG}"

# ---- bookkeeping ---------------------------------------------------------------
PASS=0; FAIL=0; SKIP=0
TOKEN=""
SERVER_PID=""
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-pipe.XXXXXX")"   # ephemeral keys + zips

# tee everything to the evidence file.
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
  # belt+suspenders: free the port if anything else is squatting our PID's port
  rm -rf "$WORK" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

HTTP_STATUS=""; HTTP_BODY=""
# req METHOD PATH [DATA] [AUTH_TOKEN]  (JSON). AUTH_TOKEN empty => no header.
req() {
  local method="$1" path="$2" data="${3:-}" tok="${4:-$TOKEN}"
  local tmp; tmp="$(mktemp)"
  local -a args=(-sS -o "$tmp" -w '%{http_code}' -X "$method" "${BASE_URL}${path}"
                 -H 'Accept: application/json')
  [ -n "$tok" ] && args+=(-H "Authorization: Bearer ${tok}")
  [ -n "$data" ] && args+=(-H 'Content-Type: application/json' --data "$data")
  HTTP_STATUS="$(curl "${args[@]}" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$tmp")"; rm -f "$tmp"
}
jqget() { printf '%s' "$HTTP_BODY" | jq -r "$1" 2>/dev/null; }
assert_status() {
  local want="$1" label="$2"
  if [ "$HTTP_STATUS" = "$want" ]; then pass "$label (HTTP $HTTP_STATUS)"; return 0; fi
  fail "$label (want $want, got $HTTP_STATUS; body: $(printf '%s' "$HTTP_BODY" | head -c 280))"
  return 1
}

log "== Helix OTA SIGNED artifact-pipeline E2E =="
log "base_url=${BASE_URL} run=${RUN_TAG} evidence=${EVIDENCE}"
log "started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log ""

# ---- prerequisites -------------------------------------------------------------
for bin in go openssl xxd base64 curl jq python3; do
  command -v "$bin" >/dev/null 2>&1 || { log "ABORT: required tool '$bin' not found"; exit 3; }
done
OSSL_VER="$(openssl version 2>/dev/null)"
log "openssl: ${OSSL_VER}"

# ---- 0. ephemeral ed25519 keypair (NEVER committed) ----------------------------
PRIV_PEM="${WORK}/artifact_priv.pem"
openssl genpkey -algorithm ed25519 -out "$PRIV_PEM" 2>/dev/null \
  || { log "ABORT: openssl could not generate an ed25519 key (scheme unsupported on this host)"; exit 3; }
# raw 32-byte public key = last 32 bytes of the DER SubjectPublicKeyInfo.
PUBKEY_B64="$(openssl pkey -in "$PRIV_PEM" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 | tr -d '\n')"
PUBLEN="$(openssl pkey -in "$PRIV_PEM" -pubout -outform DER 2>/dev/null | tail -c 32 | wc -c | tr -d ' ')"
if [ "$PUBLEN" != "32" ]; then
  log "ABORT: extracted ed25519 public key is ${PUBLEN} bytes, expected 32"; exit 3
fi
pass "generated ephemeral ed25519 keypair (raw pubkey 32 bytes, base64 configured)"

# sign_digest_hex <digest-hex> -> base64 detached ed25519 sig over raw digest bytes
sign_digest_hex() {
  local digest_hex="$1"
  local rawf sigf
  rawf="$(mktemp "${WORK}/digest.XXXXXX")"; sigf="$(mktemp "${WORK}/sig.XXXXXX")"
  printf '%s' "$digest_hex" | xxd -r -p > "$rawf"
  if ! openssl pkeyutl -sign -inkey "$PRIV_PEM" -rawin -in "$rawf" -out "$sigf" 2>/dev/null; then
    rm -f "$rawf" "$sigf"; return 1
  fi
  base64 < "$sigf" | tr -d '\n'
  rm -f "$rawf" "$sigf"
}

# build_zip_stored <out.zip> <payload-string>  (ZIP_STORED payload.bin via python3)
build_zip_stored() {
  local out="$1" payload="$2"
  PAYLOAD="$payload" OUT="$out" python3 - <<'PY'
import os, zipfile
out = os.environ["OUT"]; payload = os.environ["PAYLOAD"].encode()
with zipfile.ZipFile(out, "w", compression=zipfile.ZIP_STORED) as z:
    zi = zipfile.ZipInfo("payload.bin")
    zi.compress_type = zipfile.ZIP_STORED
    z.writestr(zi, payload)
PY
}

sha256_hex_of_file() { openssl dgst -sha256 -binary "$1" | xxd -p -c256 | tr -d '\n'; }

# upload_signed <zip> <version>  -> sets HTTP_STATUS/HTTP_BODY ; echoes artifact_id
# Builds metadata exactly like server testutil validMeta (S2 digest over the whole
# ZIP, S3 ed25519 over the raw digest bytes) + AOSP payload_properties.
upload_signed() {
  local zip="$1" version="$2"
  local digest sig file_size file_hash meta_hash
  digest="$(sha256_hex_of_file "$zip")"
  sig="$(sign_digest_hex "$digest")" || { HTTP_STATUS="SIGN_FAIL"; HTTP_BODY="openssl ed25519 sign failed"; return 1; }
  file_size="$(wc -c < "$zip" | tr -d ' ')"
  file_hash="$(printf 'file-hash' | base64 | tr -d '\n')"
  meta_hash="$(printf 'meta-hash' | base64 | tr -d '\n')"
  local meta
  meta="$(jq -nc \
    --arg sha "$digest" --arg sig "$sig" --arg ver "$version" \
    --arg os "android" --arg tm "OrangePi5Max" \
    --arg fh "$file_hash" --argjson fs "$file_size" \
    --arg mh "$meta_hash" --argjson ms 64 \
    '{sha256:$sha,signature:$sig,version:$ver,os:$os,target_model:$tm,
      file_hash:$fh,file_size:$fs,metadata_hash:$mh,metadata_size:$ms}')"
  local tmp; tmp="$(mktemp)"
  HTTP_STATUS="$(curl -sS -o "$tmp" -w '%{http_code}' -X POST "${BASE_URL}${API}/artifacts/upload" \
      -H "Authorization: Bearer ${TOKEN}" \
      -F "file=@${zip};type=application/zip;filename=ota.zip" \
      -F "metadata=${meta};type=application/json" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$tmp")"; rm -f "$tmp"
}

# ---- 1. build + start the server with the ephemeral pubkey ----------------------
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
HELIX_ARTIFACT_PUBKEY="$PUBKEY_B64" \
  "$SERVER_BIN" >"$SERVER_LOG" 2>&1 &
SERVER_PID=$!
log "server pid=${SERVER_PID} (port ${PORT}), waiting for readiness ..."

# wait up to ~10s for /healthz
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
pass "ota-server healthy on ${BASE_URL}/healthz (HELIX_ARTIFACT_PUBKEY configured)"

# ---- 2. login ------------------------------------------------------------------
req POST "${API}/auth/login" "$(jq -nc --arg u "$ADMIN_USER" --arg p "$ADMIN_PW" '{username:$u,password:$p}')" ""
assert_status 200 "POST /auth/login" || { log "ABORT: cannot continue without a token"; exit 1; }
TOKEN="$(jqget '.access_token')"
[ -n "$TOKEN" ] && [ "$TOKEN" != "null" ] || { fail "login 200 but no access_token"; exit 1; }
pass "obtained admin access token"

# ---- 3. anti-bluff control: an UNSIGNED/garbage signature MUST be rejected ------
# Prove the server is really verifying (so a later 201 means a real valid sig).
BAD_ZIP="${WORK}/bad.zip"; build_zip_stored "$BAD_ZIP" "bad payload ${RUN_TAG}"
BAD_DIGEST="$(sha256_hex_of_file "$BAD_ZIP")"
BAD_META="$(jq -nc --arg sha "$BAD_DIGEST" --arg sig "$(printf 'not-a-real-signature' | base64)" \
  --arg ver "9.9.9" --arg os "android" --arg tm "OrangePi5Max" \
  '{sha256:$sha,signature:$sig,version:$ver,os:$os,target_model:$tm}')"
BAD_TMP="$(mktemp)"
BAD_STATUS="$(curl -sS -o "$BAD_TMP" -w '%{http_code}' -X POST "${BASE_URL}${API}/artifacts/upload" \
    -H "Authorization: Bearer ${TOKEN}" \
    -F "file=@${BAD_ZIP};type=application/zip;filename=ota.zip" \
    -F "metadata=${BAD_META};type=application/json" 2>/dev/null)"
BAD_BODY="$(cat "$BAD_TMP")"; rm -f "$BAD_TMP"
BAD_CODE="$(printf '%s' "$BAD_BODY" | jq -r '.error.code' 2>/dev/null)"
if [ "$BAD_STATUS" = "422" ] && [ "$BAD_CODE" = "SIGNATURE_INVALID" ]; then
  pass "anti-bluff: a bogus signature is rejected 422 SIGNATURE_INVALID (server really verifies)"
else
  fail "anti-bluff: bogus signature was NOT rejected as expected (got HTTP ${BAD_STATUS} code=${BAD_CODE}; body: $(printf '%s' "$BAD_BODY" | head -c 200))"
fi

# ---- 4. upload the BASE signed artifact (v1.0.0) -------------------------------
BASE_ZIP="${WORK}/base.zip"; build_zip_stored "$BASE_ZIP" "BASE payload v1.0.0 ${RUN_TAG}"
upload_signed "$BASE_ZIP" "1.0.0"
if [ "$HTTP_STATUS" = "SIGN_FAIL" ]; then
  skip "BASE upload: openssl ed25519 signing failed on this host — pipeline cannot be driven (REASON: $HTTP_BODY)"
  log ""; log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
  log "RESULT: SKIP (signing unavailable; NOT a false PASS)"; exit 3
fi
assert_status 201 "POST /artifacts/upload (signed BASE v1.0.0)" || { log "ABORT: base upload not 201"; exit 1; }
BASE_ART_ID="$(jqget '.artifact_id')"
[ "$(jqget '.verified')" = "true" ] && pass "BASE artifact .verified == true" || fail "BASE artifact not verified"
[ -n "$BASE_ART_ID" ] && [ "$BASE_ART_ID" != "null" ] || { fail "no base artifact_id"; exit 1; }
log "      base_artifact_id=${BASE_ART_ID} sha256=$(jqget '.sha256')"

# ---- 5. upload the TARGET signed artifact (v1.1.0) -----------------------------
TARGET_ZIP="${WORK}/target.zip"; build_zip_stored "$TARGET_ZIP" "TARGET payload v1.1.0 ${RUN_TAG}"
upload_signed "$TARGET_ZIP" "1.1.0"
assert_status 201 "POST /artifacts/upload (signed TARGET v1.1.0)" || { log "ABORT: target upload not 201"; exit 1; }
TARGET_ART_ID="$(jqget '.artifact_id')"
[ "$(jqget '.verified')" = "true" ] && pass "TARGET artifact .verified == true" || fail "TARGET artifact not verified"
[ -n "$TARGET_ART_ID" ] && [ "$TARGET_ART_ID" != "null" ] || { fail "no target artifact_id"; exit 1; }
log "      target_artifact_id=${TARGET_ART_ID} sha256=$(jqget '.sha256')"

# ---- 6. GET artifact metadata back (proves it persisted verified) -------------
req GET "${API}/artifacts/${TARGET_ART_ID}" ""
assert_status 200 "GET /artifacts/{id} (target)"
[ "$(jqget '.artifact_id')" = "$TARGET_ART_ID" ] && pass "GET artifact echoes target artifact_id" || fail "GET artifact id mismatch"

# ---- 7. create the BASE release (v1.0.0) --------------------------------------
req POST "${API}/releases" "$(jq -nc --arg a "$BASE_ART_ID" \
  '{artifact_id:$a,version:"1.0.0",os:"android",target_model:"OrangePi5Max",notes:"base"}')"
assert_status 201 "POST /releases (BASE v1.0.0)" || { log "ABORT"; exit 1; }
BASE_REL_ID="$(jqget '.release_id')"
log "      base_release_id=${BASE_REL_ID}"

# ---- 8. create the TARGET release (v1.1.0) ------------------------------------
req POST "${API}/releases" "$(jq -nc --arg a "$TARGET_ART_ID" \
  '{artifact_id:$a,version:"1.1.0",os:"android",target_model:"OrangePi5Max",notes:"target"}')"
assert_status 201 "POST /releases (TARGET v1.1.0)" || { log "ABORT"; exit 1; }
TARGET_REL_ID="$(jqget '.release_id')"
[ -n "$TARGET_REL_ID" ] && [ "$TARGET_REL_ID" != "null" ] || { fail "no target release_id"; exit 1; }
log "      target_release_id=${TARGET_REL_ID}"

# ---- 9. register a device CURRENTLY on v1.0.0 (so it resolves the BASE) --------
DEV_BODY="$(jq -nc --arg hw "hw-${RUN_TAG}" \
  '{hardware_id:$hw,model:"OrangePi5Max",os:"android",current_version:"1.0.0"}')"
req POST "${API}/devices/register" "$DEV_BODY"
assert_status 201 "POST /devices/register (device on 1.0.0)" || { log "ABORT"; exit 1; }
DEVICE_ID="$(jqget '.device_id')"
DEVICE_TOKEN="$(jqget '.device_token')"
[ -n "$DEVICE_TOKEN" ] && [ "$DEVICE_TOKEN" != "null" ] || { fail "no device_token"; exit 1; }
log "      device_id=${DEVICE_ID}"

# ---- 10. deploy the TARGET release to all matching targets --------------------
req POST "${API}/deployments" "$(jq -nc --arg r "$TARGET_REL_ID" '{release_id:$r,strategy:"all-targets"}')"
assert_status 201 "POST /deployments (target release)" || { log "ABORT"; exit 1; }
DEP_ID="$(jqget '.deployment_id')"
[ -n "$DEP_ID" ] && [ "$DEP_ID" != "null" ] || { fail "no deployment_id"; exit 1; }
log "      deployment_id=${DEP_ID} target_count=$(jqget '.target_count')"

# ---- 11. staged rollout: create -> get -> evaluate ----------------------------
ROLLOUT_PLAN='{"phases":[{"percentage":50,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true},{"percentage":100,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true}]}'
req POST "${API}/deployments/${DEP_ID}/rollout" "$ROLLOUT_PLAN"
assert_status 201 "POST /deployments/{id}/rollout (create)"
[ "$(jqget '.deployment_id')" = "$DEP_ID" ] && pass "rollout state echoes deployment_id" || fail "rollout deployment_id mismatch"

req GET "${API}/deployments/${DEP_ID}/rollout" ""
assert_status 200 "GET /deployments/{id}/rollout"

req POST "${API}/deployments/${DEP_ID}/rollout/evaluate" '{"success_rate":0.99,"error_rate":0.0,"post_boot_health_failed":false}'
assert_status 200 "POST /deployments/{id}/rollout/evaluate"
ROLLOUT_ACTION="$(jqget '.action')"
[ -n "$ROLLOUT_ACTION" ] && [ "$ROLLOUT_ACTION" != "null" ] && pass "rollout evaluate returned decision action: ${ROLLOUT_ACTION}" || fail "rollout evaluate returned no action"

# ---- 12. register a delta (base_artifact -> target_artifact) -------------------
DELTA_SHA="$(printf 'delta-bytes-%s' "$RUN_TAG" | openssl dgst -sha256 -binary | xxd -p -c256 | tr -d '\n')"
req POST "${API}/deltas" "$(jq -nc --arg b "$BASE_ART_ID" --arg t "$TARGET_ART_ID" \
  --arg sha "$DELTA_SHA" --argjson sz 4096 \
  '{base_artifact_id:$b,target_artifact_id:$t,sha256:$sha,size:$sz,storage_ref:"s3://helix-artifacts/delta-e2e"}')"
assert_status 201 "POST /deltas (register base->target delta)" || { log "ABORT"; exit 1; }
DELTA_ID="$(jqget '.id')"
[ "$(jqget '.base_artifact_id')" = "$BASE_ART_ID" ] && pass "delta echoes base_artifact_id" || fail "delta base mismatch"
log "      delta_id=${DELTA_ID}"

# ---- 13. THE PAYOFF: device update-check returns the update WITH the delta -----
# Device is on 1.0.0 (the base release version); the target release is 1.1.0;
# a base->target delta is registered, so the update-check must serve 200 with
# .version=1.1.0 AND a populated .delta block (delta_updates_design.md).
req GET "${API}/client/update" "" "$DEVICE_TOKEN"
assert_status 200 "GET /client/update (device on 1.0.0 -> sees 1.1.0)" || {
  log "      update body: $(printf '%s' "$HTTP_BODY" | head -c 300)"
}
UPD_VERSION="$(jqget '.version')"
UPD_RELID="$(jqget '.release_id')"
UPD_SHA="$(jqget '.sha256')"
UPD_SIG="$(jqget '.signature')"
[ "$UPD_VERSION" = "1.1.0" ] && pass "update-check offers version 1.1.0" || fail "update-check version != 1.1.0 (got '$UPD_VERSION')"
[ "$UPD_RELID" = "$TARGET_REL_ID" ] && pass "update-check release_id matches target release" || fail "update-check release_id mismatch (got '$UPD_RELID')"
[ -n "$UPD_SHA" ] && [ "$UPD_SHA" != "null" ] && pass "update-check carries the artifact sha256" || fail "update-check missing sha256"
[ -n "$UPD_SIG" ] && [ "$UPD_SIG" != "null" ] && pass "update-check carries the artifact signature" || fail "update-check missing signature"

# the delta-bearing assertion (the heart of closing the SKIP):
DELTA_BLOCK="$(printf '%s' "$HTTP_BODY" | jq -c '.delta' 2>/dev/null)"
if [ -n "$DELTA_BLOCK" ] && [ "$DELTA_BLOCK" != "null" ]; then
  DELTA_BASE_VER="$(printf '%s' "$HTTP_BODY" | jq -r '.delta.base_version' 2>/dev/null)"
  DELTA_BSHA="$(printf '%s' "$HTTP_BODY" | jq -r '.delta.sha256' 2>/dev/null)"
  pass "update-check INCLUDES a .delta block (delta-bearing update reached)"
  [ "$DELTA_BASE_VER" = "1.0.0" ] && pass "delta.base_version == 1.0.0 (the device's current)" || fail "delta.base_version mismatch (got '$DELTA_BASE_VER')"
  [ "$DELTA_BSHA" = "$DELTA_SHA" ] && pass "delta.sha256 matches the registered delta" || fail "delta.sha256 mismatch (got '$DELTA_BSHA')"
  log "      DELTA-BEARING UPDATE BODY: $(printf '%s' "$HTTP_BODY" | jq -c '{version,release_id,sha256,delta}')"
else
  fail "update-check did NOT include a .delta block (delta selection did not engage; body: $(printf '%s' "$HTTP_BODY" | head -c 300))"
fi

# ---- 14. control: a device already on 1.1.0 gets 204 (no update) ---------------
DEV2_BODY="$(jq -nc --arg hw "hw2-${RUN_TAG}" \
  '{hardware_id:$hw,model:"OrangePi5Max",os:"android",current_version:"1.1.0"}')"
req POST "${API}/devices/register" "$DEV2_BODY"
DEVICE2_TOKEN="$(jqget '.device_token')"
req GET "${API}/client/update" "" "$DEVICE2_TOKEN"
assert_status 204 "GET /client/update (device already on 1.1.0 -> 204 no-content)"

# ---- summary -------------------------------------------------------------------
log ""
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then log "RESULT: FAIL"; exit 1; fi
log "RESULT: PASS"
exit 0
