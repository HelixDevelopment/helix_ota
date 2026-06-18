#!/usr/bin/env bash
# =============================================================================
# recall_lifecycle.sh — Helix OTA server-driven RECALL (forward-fix) E2E
# -----------------------------------------------------------------------------
# Purpose:
#   Autonomous, anti-bluff end-to-end challenge (Constitution §11.4 / §11.4.27 /
#   §11.4.98 / §11.4.123) that exercises the OTA *recall* (forward-fix rollback)
#   lifecycle against a LIVE `ota-server` it self-hosts. It builds REAL signed
#   ed25519 artifacts (so a real deployment can exist), stages release v1.1.0 +
#   an all-targets deployment D1, then drives the operator recall to a
#   forward-fix release v1.2.0 and asserts — over real HTTP with curl + jq, no
#   mocks of the system under test — that:
#
#     1. POST /api/v1/deployments/{D1}/recall {to_release_id:<v1.2.0>} -> 201
#        returning a rollback_history row with:
#           .kind                == "rollback"
#           .from_release_id     == <v1.1.0 release id> (the deployment's current)
#           .to_release_id       == <v1.2.0 release id> (the forward-fix target)
#           .details.mode        == "forward-fix"
#           .recall_deployment_id non-empty (a NEW active deployment was created)
#           .triggered_by        non-empty (the recalling operator)
#     2. GET /api/v1/deployments/{D1}/rollbacks -> 200 and that SAME row is
#        present in .items[] (the history is real, not a fire-and-forget 201).
#     3. The superseded deployment D1 status transitioned to "superseded"
#        (GET /api/v1/deployments/{D1} -> .status == "superseded").
#     4. The NEW recall deployment is ACTIVE and points at the v1.2.0 release
#        (GET /api/v1/deployments/{recall_deployment_id} ->
#         .status == "active" AND .release_id == <v1.2.0 release id>).
#
#   Anti-bluff negative controls (no false PASS):
#     - recall to a NON-EXISTENT target release -> 404 (target must exist).
#     - recall of a NON-EXISTENT deployment     -> 404 (deployment gate).
#     - recall with an EMPTY to_release_id       -> 400 (validation).
#     - a bogus artifact signature on upload     -> 422 SIGNATURE_INVALID
#       (proves the server really verifies, so the later 201 uploads are real).
#
# The recall endpoint requires a real, deployable release, which in turn
# requires a genuinely SIGNED artifact (the server's trust boundary verifies a
# detached ed25519 signature against its CONFIG-supplied key — never the
# request's). This script therefore self-hosts the server with an EPHEMERAL
# ed25519 keypair and signs artifacts that reproduce the server's exact
# validation contract (the same recipe as pipeline_signed.sh). If openssl
# ed25519 is unavailable the upload stage SKIPs-with-reason (§11.4.3) rather
# than fabricating a 201 — never a false PASS.
#
# Signing contract (verified against ota-artifact-validator stages.go
# ValidateHash + ValidateSignature, and server testutil_test.go validMeta):
#   sha256_hex = lowercase-hex SHA-256 over the WHOLE ZIP file bytes
#   signature  = base64( ed25519.Sign( priv, hex_decode(sha256_hex) ) )
#                i.e. ed25519 signs the raw 32 DIGEST bytes, NOT the file.
#   pubkey     = base64( last 32 raw bytes of the DER SubjectPublicKeyInfo )
#                supplied to the server via HELIX_ARTIFACT_PUBKEY.
#
# Keys are EPHEMERAL: generated into a mktemp dir that is rm -rf'd on exit.
# They are NEVER committed (§11.4.10). The server binds a UNIQUE ephemeral port
# (a free-port probe, so parallel runs never collide) and is killed on every
# exit path (trap cleanup, §11.4.14). Re-runnable end-to-end any number of
# times with self-contained state (§11.4.98 / §11.4.50).
#
# Usage:
#   recall_lifecycle.sh [--port N] [--server-bin PATH]
#   Env: HELIX_PORT (default: a free probed port), HELIX_RECALL_EVIDENCE
#        (default tests/e2e/RECALL_EVIDENCE.txt)
#
# Outputs:
#   Human-readable PASS/FAIL/SKIP lines on stdout + the evidence file; exit 0
#   only if every hard assertion passed. exit 1 on any FAIL, exit 3 on a
#   prerequisite/signing SKIP (never a false PASS).
#
# Side-effects: starts + stops one ota-server on the chosen port; frees the
#   port on exit; writes captured evidence. No host state touched.
#
# Dependencies: bash, go, openssl(>=3 ed25519), xxd, base64, curl, jq, python3.
#
# Cross-references:
#   server/internal/api/handlers_recall.go (handleRecall / handleListRollbacks),
#   server/internal/api/server.go (recall + rollbacks routes),
#   server/internal/api/handlers_artifact.go (resolvePublicKey / trust boundary),
#   tests/e2e/pipeline_signed.sh (the signed-upload recipe this reuses),
#   docs/scripts/recall_lifecycle.md (companion user guide, §11.4.18).
# =============================================================================
set -u
set -o pipefail

# ---- repo geometry -------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"

PORT="${HELIX_PORT:-}"
SERVER_BIN=""
EVIDENCE="${HELIX_RECALL_EVIDENCE:-${SCRIPT_DIR}/RECALL_EVIDENCE.txt}"

while [ $# -gt 0 ]; do
  case "$1" in
    --port)       PORT="$2"; shift 2 ;;
    --server-bin) SERVER_BIN="$2"; shift 2 ;;
    -h|--help)    sed -n '2,72p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

API="/api/v1"
RUN_TAG="recall-$(date +%s)-$$"

# Ephemeral admin/token secrets for THIS run only (never committed).
ADMIN_USER="admin@helix.test"
ADMIN_PW="recall-pw-${RUN_TAG}"
TOKEN_SECRET="recall-token-secret-${RUN_TAG}"

# ---- bookkeeping ---------------------------------------------------------------
PASS=0; FAIL=0; SKIP=0
TOKEN=""
SERVER_PID=""
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-recall.XXXXXX")"   # ephemeral keys + zips

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

# free_port -> echoes an unused TCP port (probe via python3; never collides).
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

# ---- prerequisites -------------------------------------------------------------
for bin in go openssl xxd base64 curl jq python3; do
  command -v "$bin" >/dev/null 2>&1 || { log "ABORT: required tool '$bin' not found"; exit 3; }
done

[ -n "$PORT" ] || PORT="$(free_port)"
[ -n "$PORT" ] || { log "ABORT: could not probe a free port"; exit 3; }
BASE_URL="http://127.0.0.1:${PORT}"

log "== Helix OTA server-driven RECALL (forward-fix) lifecycle E2E =="
log "base_url=${BASE_URL} run=${RUN_TAG} evidence=${EVIDENCE}"
log "started: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "openssl: $(openssl version 2>/dev/null)"
log ""

# ---- 0. ephemeral ed25519 keypair (NEVER committed) ----------------------------
PRIV_PEM="${WORK}/artifact_priv.pem"
openssl genpkey -algorithm ed25519 -out "$PRIV_PEM" 2>/dev/null \
  || { log "ABORT: openssl could not generate an ed25519 key (scheme unsupported on this host)"; exit 3; }
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

# upload_signed <zip> <version>  -> sets HTTP_STATUS/HTTP_BODY ; .artifact_id in body
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

# create_release <artifact_id> <version> <notes> -> echoes release_id
create_release() {
  local art="$1" ver="$2" notes="$3"
  req POST "${API}/releases" "$(jq -nc --arg a "$art" --arg v "$ver" --arg n "$notes" \
    '{artifact_id:$a,version:$v,os:"android",target_model:"OrangePi5Max",notes:$n}')"
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

# ---- 3. anti-bluff control: a bogus signature MUST be rejected -----------------
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

# ---- 4. upload the v1.1.0 (current) + v1.2.0 (forward-fix) signed artifacts -----
CUR_ZIP="${WORK}/v110.zip"; build_zip_stored "$CUR_ZIP" "CURRENT payload v1.1.0 ${RUN_TAG}"
upload_signed "$CUR_ZIP" "1.1.0"
if [ "$HTTP_STATUS" = "SIGN_FAIL" ]; then
  skip "v1.1.0 upload: openssl ed25519 signing failed on this host — pipeline cannot be driven (REASON: $HTTP_BODY)"
  log ""; log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
  log "RESULT: SKIP (signing unavailable; NOT a false PASS)"; exit 3
fi
assert_status 201 "POST /artifacts/upload (signed v1.1.0)" || { log "ABORT: v1.1.0 upload not 201"; exit 1; }
CUR_ART_ID="$(jqget '.artifact_id')"
[ "$(jqget '.verified')" = "true" ] && pass "v1.1.0 artifact .verified == true" || fail "v1.1.0 artifact not verified"
[ -n "$CUR_ART_ID" ] && [ "$CUR_ART_ID" != "null" ] || { fail "no v1.1.0 artifact_id"; exit 1; }

FIX_ZIP="${WORK}/v120.zip"; build_zip_stored "$FIX_ZIP" "FORWARD-FIX payload v1.2.0 ${RUN_TAG}"
upload_signed "$FIX_ZIP" "1.2.0"
assert_status 201 "POST /artifacts/upload (signed v1.2.0 forward-fix)" || { log "ABORT: v1.2.0 upload not 201"; exit 1; }
FIX_ART_ID="$(jqget '.artifact_id')"
[ "$(jqget '.verified')" = "true" ] && pass "v1.2.0 artifact .verified == true" || fail "v1.2.0 artifact not verified"
[ -n "$FIX_ART_ID" ] && [ "$FIX_ART_ID" != "null" ] || { fail "no v1.2.0 artifact_id"; exit 1; }

# ---- 5. create the v1.1.0 (current) + v1.2.0 (forward-fix) releases -------------
create_release "$CUR_ART_ID" "1.1.0" "current release"
assert_status 201 "POST /releases (v1.1.0 current)" || { log "ABORT"; exit 1; }
CUR_REL_ID="$(jqget '.release_id')"
[ -n "$CUR_REL_ID" ] && [ "$CUR_REL_ID" != "null" ] || { fail "no v1.1.0 release_id"; exit 1; }
log "      current_release_id(v1.1.0)=${CUR_REL_ID}"

create_release "$FIX_ART_ID" "1.2.0" "forward-fix release"
assert_status 201 "POST /releases (v1.2.0 forward-fix)" || { log "ABORT"; exit 1; }
FIX_REL_ID="$(jqget '.release_id')"
[ -n "$FIX_REL_ID" ] && [ "$FIX_REL_ID" != "null" ] || { fail "no v1.2.0 release_id"; exit 1; }
log "      forward_fix_release_id(v1.2.0)=${FIX_REL_ID}"

# ---- 6. deploy the v1.1.0 release to all targets -> this is D1 -----------------
req POST "${API}/deployments" "$(jq -nc --arg r "$CUR_REL_ID" '{release_id:$r,strategy:"all-targets"}')"
assert_status 201 "POST /deployments (v1.1.0 -> all-targets) = D1" || { log "ABORT"; exit 1; }
D1="$(jqget '.deployment_id')"
[ -n "$D1" ] && [ "$D1" != "null" ] || { fail "no deployment_id for D1"; exit 1; }
[ "$(jqget '.status')" = "active" ] && pass "D1 starts ACTIVE" || fail "D1 is not active at creation (got '$(jqget '.status')')"
[ "$(jqget '.release_id')" = "$CUR_REL_ID" ] && pass "D1 points at the v1.1.0 release" || fail "D1 release_id mismatch"
log "      D1 deployment_id=${D1}"

# ---- 7. ANTI-BLUFF negative controls on recall (no false PASS) -----------------
# 7a. recall with an EMPTY to_release_id -> 400 (validation).
req POST "${API}/deployments/${D1}/recall" '{"to_release_id":"","reason":"empty"}'
assert_status 400 "POST /deployments/{D1}/recall (empty to_release_id) -> 400"

# 7b. recall to a NON-EXISTENT target release -> 404 (target must exist).
req POST "${API}/deployments/${D1}/recall" "$(jq -nc --arg t "no-such-release-${RUN_TAG}" '{to_release_id:$t,reason:"bad target"}')"
assert_status 404 "POST /deployments/{D1}/recall (absent target release) -> 404"

# 7c. recall of a NON-EXISTENT deployment -> 404 (deployment gate).
req POST "${API}/deployments/no-such-deployment-${RUN_TAG}/recall" "$(jq -nc --arg t "$FIX_REL_ID" '{to_release_id:$t}')"
assert_status 404 "POST /deployments/{absent}/recall -> 404"

# ---- 8. THE PAYOFF: operator recalls D1 to the v1.2.0 forward-fix release -------
req POST "${API}/deployments/${D1}/recall" \
  "$(jq -nc --arg t "$FIX_REL_ID" '{to_release_id:$t,reason:"D1 v1.1.0 regressed in the field; forward-fix to v1.2.0"}')"
assert_status 201 "POST /deployments/{D1}/recall (forward-fix to v1.2.0) -> 201" || { log "ABORT: recall not 201"; exit 1; }
RB_ID="$(jqget '.id')"
[ -n "$RB_ID" ] && [ "$RB_ID" != "null" ] || fail "recall 201 but no rollback row id"
[ "$(jqget '.deployment_id')" = "$D1" ] && pass "rollback row deployment_id == D1" || fail "rollback row deployment_id mismatch (got '$(jqget '.deployment_id')')"
[ "$(jqget '.kind')" = "rollback" ] && pass "rollback row .kind == 'rollback'" || fail "rollback row .kind mismatch (got '$(jqget '.kind')')"
[ "$(jqget '.from_release_id')" = "$CUR_REL_ID" ] && pass "rollback row .from_release_id == the v1.1.0 release (D1's current)" || fail "rollback .from_release_id mismatch (got '$(jqget '.from_release_id')', want '$CUR_REL_ID')"
[ "$(jqget '.to_release_id')" = "$FIX_REL_ID" ] && pass "rollback row .to_release_id == the v1.2.0 forward-fix release" || fail "rollback .to_release_id mismatch (got '$(jqget '.to_release_id')', want '$FIX_REL_ID')"
[ "$(jqget '.details.mode')" = "forward-fix" ] && pass "rollback row .details.mode == 'forward-fix'" || fail "rollback .details.mode mismatch (got '$(jqget '.details.mode')')"
RECALL_DEP_ID="$(jqget '.recall_deployment_id')"
[ -n "$RECALL_DEP_ID" ] && [ "$RECALL_DEP_ID" != "null" ] && pass "rollback row carries a non-empty recall_deployment_id (${RECALL_DEP_ID})" || fail "rollback .recall_deployment_id is empty (no new deployment created)"
[ -n "$(jqget '.triggered_by')" ] && [ "$(jqget '.triggered_by')" != "null" ] && pass "rollback row records triggered_by (the recalling operator)" || fail "rollback .triggered_by is empty"

# ---- 9. GET /deployments/{D1}/rollbacks -> the SAME row is in history -----------
req GET "${API}/deployments/${D1}/rollbacks" ""
assert_status 200 "GET /deployments/{D1}/rollbacks -> 200"
RB_COUNT="$(printf '%s' "$HTTP_BODY" | jq -r '.items | length' 2>/dev/null)"
if [ -z "$RB_COUNT" ] || [ "$RB_COUNT" = "null" ] || [ "$RB_COUNT" -eq 0 ] 2>/dev/null; then
  fail "anti-bluff: rollback history is EMPTY after a recall (the 201 must be recorded)"
else
  pass "rollback history is non-empty (${RB_COUNT} row(s))"
  if printf '%s' "$HTTP_BODY" | jq -e --arg id "$RB_ID" '[.items[].id] | index($id) != null' >/dev/null 2>&1; then
    pass "rollback history contains the recall row id we just created"
  else
    fail "rollback history does NOT contain the recall row id (body: $(printf '%s' "$HTTP_BODY" | head -c 240))"
  fi
  if printf '%s' "$HTTP_BODY" | jq -e --arg f "$CUR_REL_ID" --arg t "$FIX_REL_ID" \
      '[.items[] | select(.kind=="rollback" and .from_release_id==$f and .to_release_id==$t and .details.mode=="forward-fix")] | length > 0' >/dev/null 2>&1; then
    pass "history row carries kind=rollback + mode=forward-fix + from(v1.1.0)/to(v1.2.0) release ids"
  else
    fail "history row missing the kind/mode/from/to contract (body: $(printf '%s' "$HTTP_BODY" | head -c 280))"
  fi
fi

# ---- 10. the superseded deployment D1 transitioned to 'superseded' --------------
req GET "${API}/deployments/${D1}" ""
assert_status 200 "GET /deployments/{D1} -> 200"
if [ "$(jqget '.status')" = "superseded" ]; then
  pass "superseded deployment D1 .status == 'superseded' (recall changed it from 'active')"
else
  fail "superseded deployment D1 .status is NOT 'superseded' (got '$(jqget '.status')')"
fi

# ---- 11. the NEW recall deployment is ACTIVE on the v1.2.0 release --------------
if [ -n "$RECALL_DEP_ID" ] && [ "$RECALL_DEP_ID" != "null" ]; then
  req GET "${API}/deployments/${RECALL_DEP_ID}" ""
  assert_status 200 "GET /deployments/{recall_deployment_id} -> 200"
  [ "$(jqget '.status')" = "active" ] && pass "recall deployment is ACTIVE" || fail "recall deployment is not active (got '$(jqget '.status')')"
  [ "$(jqget '.release_id')" = "$FIX_REL_ID" ] && pass "recall deployment points at the v1.2.0 forward-fix release" || fail "recall deployment release_id mismatch (got '$(jqget '.release_id')', want '$FIX_REL_ID')"
fi

# ---- summary -------------------------------------------------------------------
log ""
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then log "RESULT: FAIL"; exit 1; fi
log "RESULT: PASS"
exit 0
