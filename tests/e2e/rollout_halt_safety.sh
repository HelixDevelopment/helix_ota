#!/usr/bin/env bash
# =============================================================================
# rollout_halt_safety.sh — Helix OTA staged-rollout HALT-on-breach safety E2E
# -----------------------------------------------------------------------------
# Purpose:
#   Autonomous, anti-bluff end-to-end challenge (Constitution §11.4 / §11.4.27 /
#   §11.4.50 / §11.4.85 / §11.4.98 / §11.4.123) that exercises the SAFETY-CRITICAL
#   half of the staged-rollout state machine against a LIVE `ota-server` it
#   self-hosts: the engine MUST *HALT* a rollout the moment a phase cohort's
#   error-rate breaches the phase error_threshold (or its post-boot health
#   window fails) — and the halt MUST be STICKY (a subsequently HEALTHY verdict
#   may never silently resume a halted rollout).
#
#   The existing e2e suite (pipeline_signed.sh) only drives the HAPPY rollout
#   path: create -> get -> evaluate with a HEALTHY verdict (success_rate 0.99,
#   error_rate 0.0) -> ADVANCE. NOTHING in the suite ever pushes a verdict that
#   BREACHES the error_threshold, so the single most important safety property
#   of the whole rollout subsystem — "halt wins over advance; when in doubt,
#   stop" (telemetry_processing §5; ota-rollout-engine/decide.go) — was UNTESTED
#   end-to-end over real HTTP. This script closes that gap.
#
#   Over real HTTP (curl + jq, no mocks of the system under test), it asserts:
#
#     1. POST /api/v1/deployments/{D}/rollout {2 phases @50%/100%} -> 201, the
#        state echoes deployment_id and starts at current_phase 0 / status
#        "active" (the pre-breach baseline — proves the rollout is genuinely
#        running before we breach it, so the later halt is a real transition).
#     2. ANTI-BLUFF baseline-advance: a HEALTHY verdict on a SEPARATE control
#        deployment ADVANCES (action "advance"), proving the same endpoint does
#        NOT halt unconditionally — the halt below is caused by the breach, not
#        by a stuck handler.
#     3. THE PAYOFF — POST .../rollout/evaluate with error_rate >= error_threshold
#        (0.20 >= 0.05) -> 200 with:
#           .action       == "halt"
#           .reason       == "error_threshold_breached"
#           .state.status == "halted"
#           .state.current_phase == 0  (the breach did NOT advance the phase).
#     4. STICKY-HALT (the critical anti-resume invariant): re-evaluating the
#        halted rollout with a PERFECT healthy verdict (success 1.0, error 0.0)
#        STILL returns action "halt" / status "halted" — a halted deployment can
#        never silently un-halt itself.
#     5. GET .../rollout reflects the persisted halt (.status == "halted",
#        .current_phase == 0) — the 200 above was recorded, not fire-and-forget.
#     6. SAFETY-INVARIANT (halt-wins-over-advance): on a FRESH rollout, a verdict
#        that simultaneously MEETS the success bar (1.0 >= 0.95) AND breaches the
#        error threshold (0.20 >= 0.05) -> "halt", never "advance".
#     7. POST-BOOT-FAILURE halt: on a FRESH rollout, post_boot_health_failed:true
#        with otherwise-perfect rates -> action "halt" / reason
#        "post_boot_health_failed".
#
#   Anti-bluff negative controls (no false PASS):
#     - create rollout on a NON-EXISTENT deployment            -> 404.
#     - create rollout with an EMPTY phases array              -> 400.
#     - evaluate a rollout with an OUT-OF-RANGE rate (1.5)     -> 400 (the
#       engine validates verdict rates are fractions in [0,1]).
#     - a bogus artifact signature on upload                   -> 422
#       SIGNATURE_INVALID (proves the server really verifies, so the later 201
#       signed upload that backs the real deployment is genuine, not faked).
#
# A rollout requires a real deployment, which requires a real release, which
# requires a genuinely SIGNED artifact (the server's trust boundary verifies a
# detached ed25519 signature against its CONFIG-supplied key — never the
# request's). This script therefore self-hosts the server with an EPHEMERAL
# ed25519 keypair and signs artifacts reproducing the server's exact validation
# contract (the same recipe as pipeline_signed.sh / recall_lifecycle.sh). If
# openssl ed25519 is unavailable the pipeline SKIPs-with-reason (§11.4.3) rather
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
# Keys are EPHEMERAL: generated into a mktemp dir rm -rf'd on exit; NEVER
# committed (§11.4.10). The admin password is a FRESH per-run test-only value,
# never a real secret, never printed. The server binds a UNIQUE ephemeral port
# (a free-port probe via python3, so parallel runs never collide; §11.4.119)
# and is killed on every exit path (trap cleanup, §11.4.14). Every curl is
# bounded with --connect-timeout/--max-time so the script can never hang on a
# wedged socket. Re-runnable end-to-end any number of times with self-contained
# state (§11.4.98 / §11.4.50).
#
# Usage:
#   rollout_halt_safety.sh [--port N] [--server-bin PATH]
#   Env: HELIX_PORT (default: a free probed port),
#        HELIX_ROLLOUT_HALT_EVIDENCE (default tests/e2e/ROLLOUT_HALT_EVIDENCE.txt)
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
#   server/internal/api/handlers_rollout.go (create/get/evaluate + 400/404 map),
#   submodules/ota-rollout-engine/decide.go (the pure HALT-wins-over-advance
#     safety invariant), submodules/ota-rollout-engine/verdict.go (Action/Reason
#     vocabulary + verdict-range validation), submodules/ota-rollout-engine/
#     engine.go (idempotent terminal/sticky-halt handling),
#   server/internal/api/handlers_artifact.go (resolvePublicKey / trust boundary),
#   tests/e2e/pipeline_signed.sh (the happy-path rollout this complements),
#   tests/e2e/recall_lifecycle.sh (the signed-upload recipe this reuses).
# =============================================================================
set -u
set -o pipefail

# ---- repo geometry -------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"

PORT="${HELIX_PORT:-}"
SERVER_BIN=""
EVIDENCE="${HELIX_ROLLOUT_HALT_EVIDENCE:-${SCRIPT_DIR}/ROLLOUT_HALT_EVIDENCE.txt}"

while [ $# -gt 0 ]; do
  case "$1" in
    --port)       PORT="$2"; shift 2 ;;
    --server-bin) SERVER_BIN="$2"; shift 2 ;;
    -h|--help)    sed -n '2,120p' "$0"; exit 0 ;;
    *) echo "unknown flag: $1" >&2; exit 2 ;;
  esac
done

API="/api/v1"
RUN_TAG="rollouthalt-$(date +%s)-$$"

# Ephemeral admin/token secrets for THIS run only (never committed, never printed).
ADMIN_USER="admin@helix.test"
ADMIN_PW="rollouthalt-pw-${RUN_TAG}"
TOKEN_SECRET="rollouthalt-token-secret-${RUN_TAG}"

# ---- bookkeeping ---------------------------------------------------------------
PASS=0; FAIL=0; SKIP=0
TOKEN=""
SERVER_PID=""
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-rollouthalt.XXXXXX")"   # ephemeral keys + zips

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
# Every curl is bounded so a wedged socket can never hang the script.
req() {
  local method="$1" path="$2" data="${3:-}" tok="${4:-$TOKEN}"
  local tmp; tmp="$(mktemp)"
  local -a args=(-sS --connect-timeout 5 --max-time 20 -o "$tmp" -w '%{http_code}'
                 -X "$method" "${BASE_URL}${path}" -H 'Accept: application/json')
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
# assert_field <jq-path> <expected> <label>
assert_field() {
  local path="$1" want="$2" label="$3" got
  got="$(jqget "$path")"
  if [ "$got" = "$want" ]; then pass "$label ($path == '$want')"; return 0; fi
  fail "$label ($path: want '$want', got '$got'; body: $(printf '%s' "$HTTP_BODY" | head -c 240))"
  return 1
}

# ---- prerequisites -------------------------------------------------------------
for bin in go openssl xxd base64 curl jq python3; do
  command -v "$bin" >/dev/null 2>&1 || { log "ABORT: required tool '$bin' not found"; exit 3; }
done

[ -n "$PORT" ] || PORT="$(free_port)"
[ -n "$PORT" ] || { log "ABORT: could not probe a free port"; exit 3; }
BASE_URL="http://127.0.0.1:${PORT}"

log "== Helix OTA staged-rollout HALT-on-breach SAFETY E2E =="
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
  HTTP_STATUS="$(curl -sS --connect-timeout 5 --max-time 30 -o "$tmp" -w '%{http_code}' \
      -X POST "${BASE_URL}${API}/artifacts/upload" \
      -H "Authorization: Bearer ${TOKEN}" \
      -F "file=@${zip};type=application/zip;filename=ota.zip" \
      -F "metadata=${meta};type=application/json" 2>/dev/null)" || HTTP_STATUS="000"
  HTTP_BODY="$(cat "$tmp")"; rm -f "$tmp"
}

# create_release <artifact_id> <version> <notes> -> req sets HTTP_BODY; .release_id
create_release() {
  local art="$1" ver="$2" notes="$3"
  req POST "${API}/releases" "$(jq -nc --arg a "$art" --arg v "$ver" --arg n "$notes" \
    '{artifact_id:$a,version:$v,os:"android",target_model:"OrangePi5Max",notes:$n}')"
}

# deploy_release <release_id> <group> -> echoes deployment_id.
# Each deployment uses a DISTINCT group so the (os,target_model,group) conflict
# key differs — the server permits only ONE active deployment per target set
# (handlers_deployment.go ActiveDeploymentForTarget -> 409 Conflict otherwise),
# so the four fixtures below must each own their own group to coexist active.
deploy_release() {
  local rel="$1" group="$2"
  req POST "${API}/deployments" "$(jq -nc --arg r "$rel" --arg g "$group" \
    '{release_id:$r,strategy:"all-targets",group:$g}')"
  jqget '.deployment_id'
}

# A canonical 2-phase rollout plan (50% then 100%), success bar 0.95, error bar 0.05.
ROLLOUT_PLAN='{"phases":[{"percentage":50,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true},{"percentage":100,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true}]}'

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
  code="$(curl -sS --connect-timeout 2 --max-time 5 -o /dev/null -w '%{http_code}' "${BASE_URL}/healthz" 2>/dev/null || echo 000)"
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
BAD_STATUS="$(curl -sS --connect-timeout 5 --max-time 30 -o "$BAD_TMP" -w '%{http_code}' \
    -X POST "${BASE_URL}${API}/artifacts/upload" \
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

# ---- 4. one signed artifact -> release -> THREE deployments (breach/control/inv) -
ZIP="${WORK}/v100.zip"; build_zip_stored "$ZIP" "rollout-halt payload v1.0.0 ${RUN_TAG}"
upload_signed "$ZIP" "1.0.0"
if [ "$HTTP_STATUS" = "SIGN_FAIL" ]; then
  skip "artifact upload: openssl ed25519 signing failed on this host — pipeline cannot be driven (REASON: $HTTP_BODY)"
  log ""; log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
  log "RESULT: SKIP (signing unavailable; NOT a false PASS)"; exit 3
fi
assert_status 201 "POST /artifacts/upload (signed v1.0.0)" || { log "ABORT: upload not 201"; exit 1; }
ART_ID="$(jqget '.artifact_id')"
[ "$(jqget '.verified')" = "true" ] && pass "artifact .verified == true" || fail "artifact not verified"
[ -n "$ART_ID" ] && [ "$ART_ID" != "null" ] || { fail "no artifact_id"; exit 1; }

create_release "$ART_ID" "1.0.0" "rollout-halt safety release"
assert_status 201 "POST /releases (v1.0.0)" || { log "ABORT"; exit 1; }
REL_ID="$(jqget '.release_id')"
[ -n "$REL_ID" ] && [ "$REL_ID" != "null" ] || { fail "no release_id"; exit 1; }

# D_BREACH — the deployment we will breach + assert sticky-halt against.
D_BREACH="$(deploy_release "$REL_ID" "breach-${RUN_TAG}")"
[ -n "$D_BREACH" ] && [ "$D_BREACH" != "null" ] && pass "deployment D_BREACH created (${D_BREACH})" || { fail "no D_BREACH deployment"; exit 1; }
# D_CTRL — a control deployment to prove a HEALTHY verdict ADVANCES (the endpoint
#          does not halt unconditionally).
D_CTRL="$(deploy_release "$REL_ID" "ctrl-${RUN_TAG}")"
[ -n "$D_CTRL" ] && [ "$D_CTRL" != "null" ] && pass "deployment D_CTRL created (${D_CTRL})" || { fail "no D_CTRL deployment"; exit 1; }
# D_INV — a fresh deployment for the halt-wins-over-advance safety invariant.
D_INV="$(deploy_release "$REL_ID" "inv-${RUN_TAG}")"
[ -n "$D_INV" ] && [ "$D_INV" != "null" ] && pass "deployment D_INV created (${D_INV})" || { fail "no D_INV deployment"; exit 1; }
# D_PBF — a fresh deployment for the post-boot-failure halt.
D_PBF="$(deploy_release "$REL_ID" "pbf-${RUN_TAG}")"
[ -n "$D_PBF" ] && [ "$D_PBF" != "null" ] && pass "deployment D_PBF created (${D_PBF})" || { fail "no D_PBF deployment"; exit 1; }

# ---- 5. ANTI-BLUFF negative controls on rollout create (no false PASS) ----------
# 5a. create rollout on a NON-EXISTENT deployment -> 404.
req POST "${API}/deployments/no-such-deployment-${RUN_TAG}/rollout" "$ROLLOUT_PLAN"
assert_status 404 "POST /deployments/{absent}/rollout -> 404"

# 5b. create rollout with an EMPTY phases array -> 400.
req POST "${API}/deployments/${D_BREACH}/rollout" '{"phases":[]}'
assert_status 400 "POST /deployments/{D_BREACH}/rollout (empty phases) -> 400"

# ---- 6. create the rollout on D_BREACH (pre-breach baseline) --------------------
req POST "${API}/deployments/${D_BREACH}/rollout" "$ROLLOUT_PLAN"
assert_status 201 "POST /deployments/{D_BREACH}/rollout (2-phase plan) -> 201" || { log "ABORT"; exit 1; }
assert_field '.deployment_id' "$D_BREACH" "rollout state echoes deployment_id"
assert_field '.status' "active" "fresh rollout starts ACTIVE (pre-breach baseline)"
assert_field '.current_phase' "0" "fresh rollout starts at current_phase 0"

# ---- 7. ANTI-BLUFF baseline-advance: a HEALTHY verdict on D_CTRL ADVANCES -------
# Proves the SAME endpoint does NOT halt unconditionally; the halt below is
# caused by the breach, not by a stuck/always-halting handler.
req POST "${API}/deployments/${D_CTRL}/rollout" "$ROLLOUT_PLAN"
assert_status 201 "POST /deployments/{D_CTRL}/rollout -> 201 (control)" || { log "ABORT"; exit 1; }
req POST "${API}/deployments/${D_CTRL}/rollout/evaluate" '{"success_rate":0.99,"error_rate":0.0,"post_boot_health_failed":false}'
assert_status 200 "POST /deployments/{D_CTRL}/rollout/evaluate (healthy) -> 200"
assert_field '.action' "advance" "control: a HEALTHY verdict ADVANCES (endpoint is not stuck-halting)"
assert_field '.reason' "success_threshold_met" "control: advance reason == success_threshold_met"
assert_field '.state.current_phase' "1" "control: phase advanced 0 -> 1"

# ---- 8. THE PAYOFF: an error-threshold breach on D_BREACH HALTS the rollout -----
# error_rate 0.20 >= error_threshold 0.05 -> halt / error_threshold_breached.
req POST "${API}/deployments/${D_BREACH}/rollout/evaluate" '{"success_rate":0.80,"error_rate":0.20,"post_boot_health_failed":false}'
assert_status 200 "POST /deployments/{D_BREACH}/rollout/evaluate (error breach) -> 200" || { log "ABORT"; exit 1; }
assert_field '.action' "halt" "BREACH HALTS: action == 'halt'"
assert_field '.reason' "error_threshold_breached" "BREACH reason == 'error_threshold_breached'"
assert_field '.state.status' "halted" "BREACH leaves rollout state .status == 'halted'"
assert_field '.state.current_phase' "0" "BREACH did NOT advance the phase (still 0 — no silent progress)"

# ---- 9. STICKY-HALT: a PERFECT healthy verdict may NOT resume a halted rollout --
req POST "${API}/deployments/${D_BREACH}/rollout/evaluate" '{"success_rate":1.0,"error_rate":0.0,"post_boot_health_failed":false}'
assert_status 200 "POST /deployments/{D_BREACH}/rollout/evaluate (post-halt healthy) -> 200"
assert_field '.action' "halt" "STICKY-HALT: a halted rollout STAYS halted even on a perfect verdict (action 'halt')"
assert_field '.state.status' "halted" "STICKY-HALT: state remains .status == 'halted' (no silent resume)"
assert_field '.state.current_phase' "0" "STICKY-HALT: phase still 0 (never advanced out of halt)"

# ---- 10. GET reflects the PERSISTED halt (the 200 was recorded, not lost) -------
req GET "${API}/deployments/${D_BREACH}/rollout" ""
assert_status 200 "GET /deployments/{D_BREACH}/rollout -> 200"
assert_field '.status' "halted" "persisted rollout .status == 'halted' (halt is durable, not fire-and-forget)"
assert_field '.current_phase' "0" "persisted rollout .current_phase == 0"

# ---- 11. SAFETY INVARIANT: halt WINS over advance in one window -----------------
# success_rate 1.0 (>= 0.95 success bar) AND error_rate 0.20 (>= 0.05 error bar)
# simultaneously -> the engine MUST halt, never advance ("when in doubt, stop").
req POST "${API}/deployments/${D_INV}/rollout" "$ROLLOUT_PLAN"
assert_status 201 "POST /deployments/{D_INV}/rollout -> 201 (invariant fixture)" || { log "ABORT"; exit 1; }
req POST "${API}/deployments/${D_INV}/rollout/evaluate" '{"success_rate":1.0,"error_rate":0.20,"post_boot_health_failed":false}'
assert_status 200 "POST /deployments/{D_INV}/rollout/evaluate (success-met AND error-breached) -> 200"
assert_field '.action' "halt" "SAFETY INVARIANT: halt WINS over advance when both bars trip (action 'halt')"
assert_field '.reason' "error_threshold_breached" "SAFETY INVARIANT: reason == 'error_threshold_breached' (not success_threshold_met)"
assert_field '.state.status' "halted" "SAFETY INVARIANT: state .status == 'halted'"

# ---- 12. POST-BOOT-FAILURE halt: a failed health window aborts regardless --------
req POST "${API}/deployments/${D_PBF}/rollout" "$ROLLOUT_PLAN"
assert_status 201 "POST /deployments/{D_PBF}/rollout -> 201 (post-boot fixture)" || { log "ABORT"; exit 1; }
req POST "${API}/deployments/${D_PBF}/rollout/evaluate" '{"success_rate":1.0,"error_rate":0.0,"post_boot_health_failed":true}'
assert_status 200 "POST /deployments/{D_PBF}/rollout/evaluate (post_boot_health_failed) -> 200"
assert_field '.action' "halt" "POST-BOOT FAILURE HALTS: action == 'halt' (perfect rates but failed health window)"
assert_field '.reason' "post_boot_health_failed" "POST-BOOT FAILURE reason == 'post_boot_health_failed'"
assert_field '.state.status' "halted" "POST-BOOT FAILURE leaves state .status == 'halted'"

# ---- 13. ANTI-BLUFF: an OUT-OF-RANGE verdict rate is rejected 400 ----------------
# The engine validates verdict rates are fractions in [0,1]; the handler maps the
# validation error to 400 — proving the evaluate path really validates input and
# the 200s above are genuine accepted evaluations, not a rubber-stamp endpoint.
req POST "${API}/deployments/${D_PBF}/rollout/evaluate" '{"success_rate":1.5,"error_rate":0.0,"post_boot_health_failed":false}'
assert_status 400 "POST /deployments/{D_PBF}/rollout/evaluate (success_rate 1.5 out of [0,1]) -> 400"

# ---- summary -------------------------------------------------------------------
log ""
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
log "== summary: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped =="
if [ "$FAIL" -gt 0 ]; then log "RESULT: FAIL"; exit 1; fi
log "RESULT: PASS"
exit 0
