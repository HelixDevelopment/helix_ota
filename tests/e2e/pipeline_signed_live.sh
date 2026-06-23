#!/usr/bin/env bash
# =============================================================================
# pipeline_signed_live.sh — full SIGNED OTA pipeline, proven END-TO-END against
#   the REAL live system (ota-server + REAL PostgreSQL), anti-bluff
#   (§11.4 / §11.4.27 / §11.4.69 / §11.4.123).
# -----------------------------------------------------------------------------
# Purpose:
#   tests/e2e/pipeline_signed.sh proves the COMPLETE signed pipeline
#   (upload(signed)->release->deploy->rollout->device-poll-receives-the-update)
#   but ONLY against a SELF-HOSTED in-memory ota-server: it must own the artifact
#   pubkey to sign, so it boots its OWN server with HELIX_ARTIFACT_PUBKEY set to a
#   key it controls. That leaves a gap: the SAME end-to-end signed flow was never
#   driven against the REAL system backed by real Postgres (the F-CLUSTER stack
#   booted by tests/lib/boot_real_system.sh).
#
#   This suite closes that gap using CALLER-PUBKEY MODE
#   (boot_real_system.sh + HELIX_SYSTEM_SIGNING_KEY): the caller owns an ed25519
#   keypair, the live server is configured to trust the caller's PUBLIC key, and
#   the caller signs artifacts with the matching PRIVATE half — so the live server
#   ACCEPTS them (201, verified=true). It then drives the full pipeline against the
#   live, real-DB system:
#
#     login (live admin)                              -> 200
#     upload BASE   v1.0.0 (caller-signed)            -> 201 verified=true
#     upload TARGET v1.1.0 (caller-signed)            -> 201 verified=true
#     GET artifact (target persisted verified)        -> 200
#     anti-bluff control: bogus signature             -> 422 SIGNATURE_INVALID
#     create BASE   release (v1.0.0)                   -> 201
#     create TARGET release (v1.1.0)                   -> 201
#     register device CURRENTLY on 1.0.0              -> 201 (gets device_token)
#     deploy TARGET release (all-targets)             -> 201
#     rollout create / get / evaluate                 -> 201 / 200 / 200(action)
#     register delta (base_artifact -> target)        -> 201
#     DEVICE POLLS /client/update (on 1.0.0)          -> 200, sees 1.1.0 + the
#         correct signed artifact (sha256 + signature) + the registered .delta
#     control: device already on 1.1.0 polls          -> 204 (no update)
#
#   Every step is a REAL HTTP call against the live system with a captured,
#   redacted response written under EVIDENCE_DIR; every PASS goes through
#   ab_pass_with_evidence so a PASS without real captured evidence is mechanically
#   impossible (§11.4.69). A genuinely-unwired endpoint or flow gap is an honest
#   ab_skip_with_reason / ab_fail — NEVER a fabricated 201/200.
#
#   This proves the END-TO-END signed OTA flow works against real Postgres — the
#   thing pipeline_signed.sh only proved in-memory.
#
# Signing contract (identical to pipeline_signed.sh / trust_boundary_live.sh,
# verified against ota-artifact-validator stages.go ValidateHash/ValidateSignature):
#   sha256_hex = lowercase-hex SHA-256 over the WHOLE ZIP bytes
#   signature  = base64( ed25519.Sign(priv, hex_decode(sha256_hex)) )
#                i.e. ed25519 signs the raw 32 DIGEST bytes, NOT the file.
#
# Keys are EPHEMERAL: generated into a mktemp dir, rm -rf'd on exit, NEVER
#   committed (§11.4.10). Captured evidence is redacted of token/key material
#   before it is written under docs/qa/ (§11.4.10).
#
# Usage:
#   # caller-pubkey mode against the live F-CLUSTER (boots, runs, tears down):
#   bash tests/e2e/pipeline_signed_live.sh
#   # against an already-booted base URL in caller-pubkey mode (skip boot/teardown):
#   BASE_URL=http://127.0.0.1:18080 bash tests/e2e/pipeline_signed_live.sh --no-boot
#
# Env:
#   TARGET / REMOTE_USER   passed through to boot_real_system.sh (live host).
#   BASE_URL               override the live base URL (else taken from boot).
#   ADMIN_USER/ADMIN_PW    live admin login (defaults match system.compose.yml).
#   EVIDENCE_DIR           where redacted evidence is written
#                          (default docs/qa/20260623-signed-pipeline-live).
#
# Dependencies: bash, openssl(>=3 ed25519), xxd, base64, curl, jq, python3, ssh,
#   scp (when tunneling to a remote loopback).
# Cross-references: tests/lib/boot_real_system.sh (caller-pubkey mode),
#   tests/security/trust_boundary_live.sh (caller-pubkey upload + ssh tunnel),
#   tests/e2e/pipeline_signed.sh (the in-memory flow this lifts to the real DB),
#   tests/lib/anti_bluff.sh, server/internal/api/handlers_artifact.go.
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BOOT="${REPO_ROOT}/tests/lib/boot_real_system.sh"

# shellcheck source=/dev/null
. "${REPO_ROOT}/tests/lib/anti_bluff.sh"

EVIDENCE_DIR="${EVIDENCE_DIR:-${REPO_ROOT}/docs/qa/20260623-signed-pipeline-live}"
mkdir -p "${EVIDENCE_DIR}"
RUN_LOG="${EVIDENCE_DIR}/run.log"
: > "${RUN_LOG}"

ADMIN_USER="${ADMIN_USER:-admin@helix.system}"
ADMIN_PW="${ADMIN_PW:-ephemeral-test-stack-NOT-A-SECRET}"  # §11.4.10 non-secret test constant (matches system.compose.yml)

DO_BOOT=1
[ "${1:-}" = "--no-boot" ] && DO_BOOT=0

RUN_TAG="pipe-live-$(date +%s)-$$"
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-pipe-live.XXXXXX")"
BOOTED=0

log() { printf '%s %s\n' "[$(date -u +%H:%M:%SZ)]" "$*" | tee -a "${RUN_LOG}" >&2; }

# Redact bearer tokens + base64 token/key blobs so no secret material lands in
# committed evidence (§11.4.10). device_token / access_token values redacted.
redact() {
  sed -E \
    -e 's/(Authorization: Bearer )[A-Za-z0-9._-]+/\1<REDACTED>/g' \
    -e 's/("access_token"[[:space:]]*:[[:space:]]*")[^"]+/\1<REDACTED>/g' \
    -e 's/("refresh_token"[[:space:]]*:[[:space:]]*")[^"]+/\1<REDACTED>/g' \
    -e 's/("device_token"[[:space:]]*:[[:space:]]*")[^"]+/\1<REDACTED>/g'
}

cleanup() {
  if [ "${BOOTED}" = "1" ]; then
    log "TEARDOWN: tearing down the live system stack (project-scoped; integration suite pg untouched) — §11.4.14"
    bash "${BOOT}" --down >>"${RUN_LOG}" 2>&1 || true
  fi
  rm -rf "${WORK}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

ab_init "pipeline_signed_live"

# ---- 0. ephemeral caller keypair (the live server will trust THIS pubkey) -----
CALLER_PRIV="${WORK}/caller_priv.pem"
openssl genpkey -algorithm ed25519 -out "${CALLER_PRIV}" 2>/dev/null \
  || { log "ABORT: openssl could not generate an ed25519 key"; ab_skip_with_reason "signed pipeline live (openssl ed25519 unavailable)" hardware_not_present || true; ab_summary; exit 3; }
CALLER_PUB_B64="$(openssl pkey -in "${CALLER_PRIV}" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 | tr -d '\n')"
[ "$(printf '%s' "${CALLER_PUB_B64}" | base64 -d 2>/dev/null | wc -c | tr -d ' ')" = "32" ] \
  || { log "ABORT: caller pubkey not 32 bytes"; ab_fail "caller pubkey length" "not 32 bytes"; ab_summary; exit 1; }
log "caller ed25519 keypair ready (ephemeral, never committed; server will trust the public half)"

# ---- 1. obtain a LIVE base URL in CALLER-PUBKEY MODE --------------------------
if [ "${DO_BOOT}" = "1" ]; then
  log "BOOT: booting the REAL system in CALLER-PUBKEY MODE (server trusts the caller's pubkey, real Postgres)"
  BOOT_OUT="$(HELIX_SYSTEM_SIGNING_KEY="${CALLER_PRIV}" bash "${BOOT}" --up 2>>"${RUN_LOG}")" || {
    log "ABORT: boot_real_system.sh --up failed (see ${RUN_LOG})"
    ab_skip_with_reason "signed pipeline live (real system would not boot)" topology_unsupported || true
    ab_summary; exit 3
  }
  BOOTED=1
  BASE_URL="${BASE_URL:-$(printf '%s\n' "${BOOT_OUT}" | sed -n 's/^BASE_URL=//p' | tail -1)}"
fi
: "${BASE_URL:?BASE_URL not set (boot failed or --no-boot without BASE_URL)}"
API="/api/v1"
log "live base_url=${BASE_URL}"

# When the live system is reachable only via remote loopback, tunnel curl/scp over
# ssh (the boot harness probes 127.0.0.1:<port> ON the remote).
REMOTE_USER="${REMOTE_USER:-milosvasic}"
TARGET="${TARGET:-${REMOTE_USER}@thinker.local}"
case "${TARGET}" in *@*) ;; *) TARGET="${REMOTE_USER}@${TARGET}" ;; esac
SSH="ssh -o BatchMode=yes -o ConnectTimeout=15"
USE_SSH=0
case "${BASE_URL}" in *127.0.0.1*|*localhost*) [ "${DO_BOOT}" = "1" ] && USE_SSH=1 ;; esac

# curl_live <curl-args...> : run a fully-formed curl against the live server,
# locally or via the remote loopback. Prints curl stdout; sets no globals.
curl_live() {
  if [ "${USE_SSH}" = "1" ]; then
    $SSH "${TARGET}" "curl $*"
  else
    eval "curl $*"
  fi
}

# ---- helpers ----------------------------------------------------------------
build_zip_stored() {
  PAYLOAD="$2" OUT="$1" python3 - <<'PY'
import os, zipfile
out = os.environ["OUT"]; payload = os.environ["PAYLOAD"].encode()
with zipfile.ZipFile(out, "w", compression=zipfile.ZIP_STORED) as z:
    zi = zipfile.ZipInfo("payload.bin"); zi.compress_type = zipfile.ZIP_STORED
    z.writestr(zi, payload)
PY
}
sha256_hex_of_file() { openssl dgst -sha256 -binary "$1" | xxd -p -c256 | tr -d '\n'; }
# sign_digest_hex <priv.pem> <digest-hex> -> base64 ed25519 sig over raw digest bytes
sign_digest_hex() {
  _sd_priv="$1"; _sd_hex="$2"
  _sd_raw="$(mktemp "${WORK}/dg.XXXXXX")"; _sd_sig="$(mktemp "${WORK}/sg.XXXXXX")"
  printf '%s' "${_sd_hex}" | xxd -r -p > "${_sd_raw}"
  openssl pkeyutl -sign -inkey "${_sd_priv}" -rawin -in "${_sd_raw}" -out "${_sd_sig}" 2>/dev/null \
    || { rm -f "${_sd_raw}" "${_sd_sig}"; return 1; }
  base64 < "${_sd_sig}" | tr -d '\n'
  rm -f "${_sd_raw}" "${_sd_sig}"
}

# req METHOD PATH [JSON_DATA] [TOKEN] [EVIDENCE_FILE]
#   Runs a JSON request against the live server. Captures "<body>\n<httpcode>"
#   into EVIDENCE_FILE (redacted) and sets HTTP_STATUS/HTTP_BODY globals.
HTTP_STATUS=""; HTTP_BODY=""
req() {
  _r_method="$1"; _r_path="$2"; _r_data="${3:-}"; _r_tok="${4:-${TOKEN:-}}"; _r_ev="${5:-}"
  _r_hdr=""
  [ -n "${_r_tok}" ] && _r_hdr="-H 'Authorization: Bearer ${_r_tok}'"
  _r_cmd="curl -sS -o - -w '\n%{http_code}' -X ${_r_method} '${BASE_URL}${_r_path}' -H 'Accept: application/json' ${_r_hdr}"
  if [ -n "${_r_data}" ]; then
    _r_cmd="${_r_cmd} -H 'Content-Type: application/json' --data '${_r_data}'"
  fi
  # Retry on a transient connection failure (HTTP 000 / empty) — see live_upload.
  _r_out=""; _r_try=0
  while [ "${_r_try}" -lt 4 ]; do
    _r_out="$(curl_live "${_r_cmd#curl }" 2>/dev/null || true)"
    _r_code="$(printf '%s\n' "${_r_out}" | tail -1)"
    case "${_r_code}" in 000|"") _r_try=$((_r_try+1)); sleep 2 ;; *) break ;; esac
  done
  HTTP_STATUS="$(printf '%s\n' "${_r_out}" | tail -1)"
  HTTP_BODY="$(printf '%s\n' "${_r_out}" | sed '$d')"
  [ -n "${_r_ev}" ] && printf '%s %s %s\nHTTP %s\n%s\n' "${_r_method}" "${_r_path}" "(live)" "${HTTP_STATUS}" "${HTTP_BODY}" | redact > "${_r_ev}"
  printf '%s' "${HTTP_STATUS}"
}
jqget() { printf '%s' "${HTTP_BODY}" | jq -r "$1" 2>/dev/null; }

# live_upload <zip> <version> <sig_b64> <evidence-file> -> echoes httpcode.
#   Multipart upload to the live server; ZIP staged on the remote when tunneling.
#   Matches the server's ArtifactUploadMetadata schema. NOTE: live_upload runs in
#   command substitution (a subshell), so it CANNOT export HTTP_BODY to the
#   caller — the caller MUST read the JSON body back from <evidence-file> via
#   upload_body() (the 4th line onward of the evidence file).
live_upload() {
  _u_zip="$1"; _u_ver="$2"; _u_sig="$3"; _u_ev="$4"
  # Minimal metadata matching the proven-live recipe in trust_boundary_live.sh
  # (the server's ArtifactUploadMetadata uses DisallowUnknownFields — only the
  # required fields are sent; the AOSP payload_properties are NOT required and are
  # omitted to keep the request identical to the proven-live upload).
  _u_sha="$(sha256_hex_of_file "${_u_zip}")"
  _u_meta="$(jq -nc --arg sha "${_u_sha}" --arg sig "${_u_sig}" --arg ver "${_u_ver}" \
      --arg os "android" --arg tm "OrangePi5Max" \
      '{sha256:$sha,signature:$sig,version:$ver,os:$os,target_model:$tm}')"
  if [ "${USE_SSH}" = "1" ]; then
    _u_remote="/tmp/$(basename "${_u_zip}").$$"
    scp -q -o BatchMode=yes "${_u_zip}" "${TARGET}:${_u_remote}" >/dev/null 2>&1 || { HTTP_BODY="SCP_FAIL"; echo "SCP_FAIL"; return 1; }
    _u_filearg="${_u_remote}"; _u_runner() { $SSH "${TARGET}" "$1"; }
  else
    _u_filearg="${_u_zip}"; _u_runner() { eval "$1"; }
  fi
  _u_cmd="curl -sS -o - -w '\n%{http_code}' -X POST '${BASE_URL}${API}/artifacts/upload' \
      -H 'Authorization: Bearer ${TOKEN}' \
      -F 'file=@${_u_filearg};type=application/zip;filename=ota.zip' \
      -F 'metadata=${_u_meta};type=application/json'"
  # Retry on a transient connection failure (HTTP 000 / empty body) — a brief
  # ssh/curl hiccup against a system the boot proved healthy must not be reported
  # as a product FAIL (§11.4.6). A genuine reject (4xx/5xx) returns immediately.
  _u_out=""; _u_try=0
  while [ "${_u_try}" -lt 4 ]; do
    _u_out="$(_u_runner "${_u_cmd}" 2>/dev/null || true)"
    _u_code="$(printf '%s\n' "${_u_out}" | tail -1)"
    case "${_u_code}" in 000|"") _u_try=$((_u_try+1)); sleep 2 ;; *) break ;; esac
  done
  [ "${USE_SSH}" = "1" ] && $SSH "${TARGET}" "rm -f '${_u_remote}'" >/dev/null 2>&1 || true
  _u_body="$(printf '%s\n' "${_u_out}" | sed '$d')"
  printf '%s %s\nHTTP %s\n%s\n' "POST" "${API}/artifacts/upload (signed ${_u_ver})" "$(printf '%s\n' "${_u_out}" | tail -1)" "${_u_body}" | redact > "${_u_ev}"
  printf '%s\n' "${_u_out}" | tail -1
}

# upload_body <evidence-file> : re-read the captured JSON body (4th line onward)
# from a live_upload evidence file (live_upload ran in a subshell so it could not
# export HTTP_BODY). Prints the JSON body on stdout.
upload_body() { sed -n '3,$p' "$1" 2>/dev/null; }

# ---- 2. live readiness + login ----------------------------------------------
# Boot already waited for /readyz->200, but a transient miss (e.g. a contending
# §11.4.119 single-owner restart of the shared stack) should not immediately SKIP
# a system the boot just proved healthy. Re-probe a few times before deciding.
READY_CODE=000
for _ in 1 2 3 4 5 6; do
  READY_CODE="$(curl_live "-sS -o /dev/null -w '%{http_code}' --max-time 6 '${BASE_URL}/readyz'" 2>/dev/null | tail -c3 || echo 000)"
  [ "${READY_CODE}" = "200" ] && break
  sleep 2
done
if [ "${READY_CODE}" != "200" ]; then
  log "live /readyz not 200 (last=${READY_CODE}) — system unreachable"
  ab_skip_with_reason "signed pipeline live (system not reachable)" network_unreachable_external || true
  ab_summary; exit 3
fi
log "live /readyz -> 200"

LOGIN_BODY="$(jq -nc --arg u "${ADMIN_USER}" --arg p "${ADMIN_PW}" '{username:$u,password:$p}')"
EV_LOGIN="${EVIDENCE_DIR}/step01_login.txt"
req POST "${API}/auth/login" "${LOGIN_BODY}" "" "${EV_LOGIN}" >/dev/null
TOKEN="$(jqget '.access_token')"
if [ "${HTTP_STATUS}" = "200" ] && [ -n "${TOKEN}" ] && [ "${TOKEN}" != "null" ]; then
  ab_pass_with_evidence "step 01 login: live admin /auth/login -> 200, access_token issued (redacted)" "${EV_LOGIN}"
else
  ab_fail "step 01 live admin login" "expected 200 + access_token, got HTTP ${HTTP_STATUS} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
  ab_summary; exit 1
fi

# ---- 3. anti-bluff control: a bogus signature MUST be rejected ---------------
# Proves the live server REALLY verifies, so a later 201 means a genuine valid sig.
BAD_ZIP="${WORK}/bad.zip"; build_zip_stored "${BAD_ZIP}" "bad payload ${RUN_TAG}"
BAD_SHA="$(sha256_hex_of_file "${BAD_ZIP}")"
EV_BAD="${EVIDENCE_DIR}/step02_antibluff_bogus_sig_reject.txt"
CODE_BAD="$(live_upload "${BAD_ZIP}" "9.9.9" "$(printf 'not-a-real-signature' | base64 | tr -d '\n')" "${EV_BAD}")"
BAD_BODY="$(upload_body "${EV_BAD}")"
BAD_ERR="$(printf '%s' "${BAD_BODY}" | jq -r '.error.code' 2>/dev/null)"
log "step 02 anti-bluff bogus-sig: HTTP ${CODE_BAD} error.code=${BAD_ERR}"
if [ "${CODE_BAD}" = "422" ] && [ "${BAD_ERR}" = "SIGNATURE_INVALID" ]; then
  ab_pass_with_evidence "step 02 anti-bluff: live server REJECTS a bogus signature (422 SIGNATURE_INVALID) — it really verifies, so a 201 below is a genuine valid signature" "${EV_BAD}"
else
  ab_fail "step 02 anti-bluff bogus-sig reject" "expected 422 SIGNATURE_INVALID, got HTTP ${CODE_BAD} code=${BAD_ERR} (body: $(printf '%s' "${BAD_BODY}" | head -c 200))"
fi

# ---- 4. upload BASE signed artifact (v1.0.0) --------------------------------
BASE_ZIP="${WORK}/base.zip"; build_zip_stored "${BASE_ZIP}" "BASE payload v1.0.0 ${RUN_TAG}"
BASE_SHA="$(sha256_hex_of_file "${BASE_ZIP}")"
BASE_SIG="$(sign_digest_hex "${CALLER_PRIV}" "${BASE_SHA}")" || {
  log "openssl ed25519 signing failed — pipeline cannot be driven"
  ab_skip_with_reason "BASE upload (signing unavailable on this host)" hardware_not_present || true
  ab_summary; exit 3
}
EV_BASE="${EVIDENCE_DIR}/step03_upload_base_signed.txt"
CODE_BASE="$(live_upload "${BASE_ZIP}" "1.0.0" "${BASE_SIG}" "${EV_BASE}")"
BASE_BODY="$(upload_body "${EV_BASE}")"
BASE_ART_ID="$(printf '%s' "${BASE_BODY}" | jq -r '.artifact_id' 2>/dev/null)"
BASE_VERIFIED="$(printf '%s' "${BASE_BODY}" | jq -r '.verified' 2>/dev/null)"
log "step 03 BASE upload: HTTP ${CODE_BASE} verified=${BASE_VERIFIED} artifact_id=${BASE_ART_ID}"
if [ "${CODE_BASE}" = "201" ] && [ "${BASE_VERIFIED}" = "true" ] && [ -n "${BASE_ART_ID}" ] && [ "${BASE_ART_ID}" != "null" ]; then
  ab_pass_with_evidence "step 03 upload BASE v1.0.0: caller-signed artifact ACCEPTED by LIVE server (201, verified=true) on real Postgres" "${EV_BASE}"
else
  ab_fail "step 03 upload BASE v1.0.0" "expected 201 verified=true + artifact_id, got HTTP ${CODE_BASE} verified=${BASE_VERIFIED} (body: $(printf '%s' "${BASE_BODY}" | head -c 200))"
  ab_summary; exit 1
fi

# ---- 5. upload TARGET signed artifact (v1.1.0) ------------------------------
TARGET_ZIP="${WORK}/target.zip"; build_zip_stored "${TARGET_ZIP}" "TARGET payload v1.1.0 ${RUN_TAG}"
TARGET_SHA="$(sha256_hex_of_file "${TARGET_ZIP}")"
TARGET_SIG="$(sign_digest_hex "${CALLER_PRIV}" "${TARGET_SHA}")"
EV_TGT="${EVIDENCE_DIR}/step04_upload_target_signed.txt"
CODE_TGT="$(live_upload "${TARGET_ZIP}" "1.1.0" "${TARGET_SIG}" "${EV_TGT}")"
TGT_BODY="$(upload_body "${EV_TGT}")"
TARGET_ART_ID="$(printf '%s' "${TGT_BODY}" | jq -r '.artifact_id' 2>/dev/null)"
TARGET_VERIFIED="$(printf '%s' "${TGT_BODY}" | jq -r '.verified' 2>/dev/null)"
TARGET_ART_SHA="$(printf '%s' "${TGT_BODY}" | jq -r '.sha256' 2>/dev/null)"
log "step 04 TARGET upload: HTTP ${CODE_TGT} verified=${TARGET_VERIFIED} artifact_id=${TARGET_ART_ID}"
if [ "${CODE_TGT}" = "201" ] && [ "${TARGET_VERIFIED}" = "true" ] && [ -n "${TARGET_ART_ID}" ] && [ "${TARGET_ART_ID}" != "null" ]; then
  ab_pass_with_evidence "step 04 upload TARGET v1.1.0: caller-signed artifact ACCEPTED by LIVE server (201, verified=true) on real Postgres" "${EV_TGT}"
else
  ab_fail "step 04 upload TARGET v1.1.0" "expected 201 verified=true + artifact_id, got HTTP ${CODE_TGT} verified=${TARGET_VERIFIED} (body: $(printf '%s' "${TGT_BODY}" | head -c 200))"
  ab_summary; exit 1
fi

# ---- 6. GET the target artifact back (proves persisted verified) ------------
EV_GET="${EVIDENCE_DIR}/step05_get_target_artifact.txt"
req GET "${API}/artifacts/${TARGET_ART_ID}" "" "" "${EV_GET}" >/dev/null
GOT_ID="$(jqget '.artifact_id')"
log "step 05 GET artifact: HTTP ${HTTP_STATUS} artifact_id=${GOT_ID}"
if [ "${HTTP_STATUS}" = "200" ] && [ "${GOT_ID}" = "${TARGET_ART_ID}" ]; then
  ab_pass_with_evidence "step 05 GET /artifacts/{id}: target artifact persisted + retrievable from real Postgres (200, id echoes)" "${EV_GET}"
else
  ab_fail "step 05 GET target artifact" "expected 200 + matching artifact_id, got HTTP ${HTTP_STATUS} id=${GOT_ID}"
fi

# ---- 7. create BASE release (v1.0.0) ----------------------------------------
EV_RELB="${EVIDENCE_DIR}/step06_release_base.txt"
req POST "${API}/releases" "$(jq -nc --arg a "${BASE_ART_ID}" \
  '{artifact_id:$a,version:"1.0.0",os:"android",target_model:"OrangePi5Max",notes:"base"}')" "" "${EV_RELB}" >/dev/null
BASE_REL_ID="$(jqget '.release_id')"
log "step 06 BASE release: HTTP ${HTTP_STATUS} release_id=${BASE_REL_ID}"
if [ "${HTTP_STATUS}" = "201" ] && [ -n "${BASE_REL_ID}" ] && [ "${BASE_REL_ID}" != "null" ]; then
  ab_pass_with_evidence "step 06 create BASE release v1.0.0: live server persists release referencing the signed BASE artifact (201)" "${EV_RELB}"
else
  ab_fail "step 06 create BASE release" "expected 201 + release_id, got HTTP ${HTTP_STATUS} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
  ab_summary; exit 1
fi

# ---- 8. create TARGET release (v1.1.0) --------------------------------------
EV_RELT="${EVIDENCE_DIR}/step07_release_target.txt"
req POST "${API}/releases" "$(jq -nc --arg a "${TARGET_ART_ID}" \
  '{artifact_id:$a,version:"1.1.0",os:"android",target_model:"OrangePi5Max",notes:"target"}')" "" "${EV_RELT}" >/dev/null
TARGET_REL_ID="$(jqget '.release_id')"
log "step 07 TARGET release: HTTP ${HTTP_STATUS} release_id=${TARGET_REL_ID}"
if [ "${HTTP_STATUS}" = "201" ] && [ -n "${TARGET_REL_ID}" ] && [ "${TARGET_REL_ID}" != "null" ]; then
  ab_pass_with_evidence "step 07 create TARGET release v1.1.0: live server persists release referencing the signed TARGET artifact (201)" "${EV_RELT}"
else
  ab_fail "step 07 create TARGET release" "expected 201 + release_id, got HTTP ${HTTP_STATUS} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
  ab_summary; exit 1
fi

# ---- 9. register a device CURRENTLY on v1.0.0 -------------------------------
EV_DEV="${EVIDENCE_DIR}/step08_register_device.txt"
req POST "${API}/devices/register" "$(jq -nc --arg hw "hw-${RUN_TAG}" \
  '{hardware_id:$hw,model:"OrangePi5Max",os:"android",current_version:"1.0.0"}')" "" "${EV_DEV}" >/dev/null
DEVICE_ID="$(jqget '.device_id')"
DEVICE_TOKEN="$(jqget '.device_token')"
log "step 08 register device (on 1.0.0): HTTP ${HTTP_STATUS} device_id=${DEVICE_ID}"
if [ "${HTTP_STATUS}" = "201" ] && [ -n "${DEVICE_TOKEN}" ] && [ "${DEVICE_TOKEN}" != "null" ]; then
  ab_pass_with_evidence "step 08 register device on 1.0.0: live server registers the device + issues a device_token (201, token redacted)" "${EV_DEV}"
else
  ab_fail "step 08 register device" "expected 201 + device_token, got HTTP ${HTTP_STATUS} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
  ab_summary; exit 1
fi

# ---- 10. deploy the TARGET release ------------------------------------------
EV_DEP="${EVIDENCE_DIR}/step09_deploy_target.txt"
req POST "${API}/deployments" "$(jq -nc --arg r "${TARGET_REL_ID}" '{release_id:$r,strategy:"all-targets"}')" "" "${EV_DEP}" >/dev/null
DEP_ID="$(jqget '.deployment_id')"
DEP_TC="$(jqget '.target_count')"
log "step 09 deploy TARGET release: HTTP ${HTTP_STATUS} deployment_id=${DEP_ID} target_count=${DEP_TC}"
if [ "${HTTP_STATUS}" = "201" ] && [ -n "${DEP_ID}" ] && [ "${DEP_ID}" != "null" ]; then
  ab_pass_with_evidence "step 09 deploy TARGET release: live server creates a deployment targeting matching devices (201)" "${EV_DEP}"
else
  ab_fail "step 09 deploy TARGET release" "expected 201 + deployment_id, got HTTP ${HTTP_STATUS} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
  ab_summary; exit 1
fi

# ---- 11. staged rollout: create -> get -> evaluate --------------------------
ROLLOUT_PLAN='{"phases":[{"percentage":50,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true},{"percentage":100,"success_threshold":0.95,"error_threshold":0.05,"duration_seconds":60,"auto_progress":true}]}'
EV_RC="${EVIDENCE_DIR}/step10_rollout_create.txt"
req POST "${API}/deployments/${DEP_ID}/rollout" "${ROLLOUT_PLAN}" "" "${EV_RC}" >/dev/null
RC_DEPID="$(jqget '.deployment_id')"
log "step 10 rollout create: HTTP ${HTTP_STATUS} deployment_id=${RC_DEPID}"
if [ "${HTTP_STATUS}" = "201" ] && [ "${RC_DEPID}" = "${DEP_ID}" ]; then
  ab_pass_with_evidence "step 10 rollout create: live server creates a staged rollout for the deployment (201, echoes deployment_id)" "${EV_RC}"
else
  ab_fail "step 10 rollout create" "expected 201 + matching deployment_id, got HTTP ${HTTP_STATUS} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
fi

EV_RG="${EVIDENCE_DIR}/step11_rollout_get.txt"
req GET "${API}/deployments/${DEP_ID}/rollout" "" "" "${EV_RG}" >/dev/null
log "step 11 rollout get: HTTP ${HTTP_STATUS}"
if [ "${HTTP_STATUS}" = "200" ]; then
  ab_pass_with_evidence "step 11 rollout get: live server returns the rollout state (200)" "${EV_RG}"
else
  ab_fail "step 11 rollout get" "expected 200, got HTTP ${HTTP_STATUS} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
fi

EV_RE="${EVIDENCE_DIR}/step12_rollout_evaluate.txt"
req POST "${API}/deployments/${DEP_ID}/rollout/evaluate" '{"success_rate":0.99,"error_rate":0.0,"post_boot_health_failed":false}' "" "${EV_RE}" >/dev/null
ROLLOUT_ACTION="$(jqget '.action')"
log "step 12 rollout evaluate: HTTP ${HTTP_STATUS} action=${ROLLOUT_ACTION}"
if [ "${HTTP_STATUS}" = "200" ] && [ -n "${ROLLOUT_ACTION}" ] && [ "${ROLLOUT_ACTION}" != "null" ]; then
  ab_pass_with_evidence "step 12 rollout evaluate: live server returns a rollout decision action='${ROLLOUT_ACTION}' from a healthy-metrics evaluation (200) — rollout PROGRESSES" "${EV_RE}"
else
  ab_fail "step 12 rollout evaluate" "expected 200 + decision action, got HTTP ${HTTP_STATUS} action=${ROLLOUT_ACTION} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
fi

# ---- 12. register a delta (base_artifact -> target_artifact) ----------------
DELTA_SHA="$(printf 'delta-bytes-%s' "${RUN_TAG}" | openssl dgst -sha256 -binary | xxd -p -c256 | tr -d '\n')"
EV_DELTA="${EVIDENCE_DIR}/step13_register_delta.txt"
req POST "${API}/deltas" "$(jq -nc --arg b "${BASE_ART_ID}" --arg t "${TARGET_ART_ID}" \
  --arg sha "${DELTA_SHA}" --argjson sz 4096 \
  '{base_artifact_id:$b,target_artifact_id:$t,sha256:$sha,size:$sz,storage_ref:"s3://helix-artifacts/delta-e2e-live"}')" "" "${EV_DELTA}" >/dev/null
DELTA_ID="$(jqget '.id')"
DELTA_BASE_ECHO="$(jqget '.base_artifact_id')"
log "step 13 register delta: HTTP ${HTTP_STATUS} delta_id=${DELTA_ID}"
if [ "${HTTP_STATUS}" = "201" ] && [ "${DELTA_BASE_ECHO}" = "${BASE_ART_ID}" ]; then
  ab_pass_with_evidence "step 13 register delta: live server persists a base->target delta (201, echoes base_artifact_id) on real Postgres" "${EV_DELTA}"
else
  ab_fail "step 13 register delta" "expected 201 + base_artifact_id echo, got HTTP ${HTTP_STATUS} base=${DELTA_BASE_ECHO} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
fi

# ---- 13. THE PAYOFF: device polls /client/update and RECEIVES the signed update
# Device is on 1.0.0; target release is 1.1.0; a base->target delta is registered.
# The update-check MUST serve 200 with version 1.1.0 + the artifact sha256 +
# signature + a populated .delta block — the device receives the correct SIGNED
# artifact through the full real-DB pipeline.
EV_POLL="${EVIDENCE_DIR}/step14_device_poll_receives_signed_update.txt"
req GET "${API}/client/update" "" "${DEVICE_TOKEN}" "${EV_POLL}" >/dev/null
UPD_CODE="${HTTP_STATUS}"
UPD_VER="$(jqget '.version')"
UPD_RELID="$(jqget '.release_id')"
UPD_SHA="$(jqget '.sha256')"
UPD_SIG="$(jqget '.signature')"
DELTA_BLOCK="$(printf '%s' "${HTTP_BODY}" | jq -c '.delta' 2>/dev/null)"
DELTA_BVER="$(printf '%s' "${HTTP_BODY}" | jq -r '.delta.base_version' 2>/dev/null)"
DELTA_BSHA="$(printf '%s' "${HTTP_BODY}" | jq -r '.delta.sha256' 2>/dev/null)"
log "step 14 device poll: HTTP ${UPD_CODE} version=${UPD_VER} release_id=${UPD_RELID} sha=${UPD_SHA:0:12}… sig?=$( [ -n "${UPD_SIG}" ] && [ "${UPD_SIG}" != "null" ] && echo yes || echo no ) delta?=$( [ -n "${DELTA_BLOCK}" ] && [ "${DELTA_BLOCK}" != "null" ] && echo yes || echo no )"

POLL_OK=1
[ "${UPD_CODE}" = "200" ] || POLL_OK=0
[ "${UPD_VER}" = "1.1.0" ] || POLL_OK=0
[ "${UPD_RELID}" = "${TARGET_REL_ID}" ] || POLL_OK=0
{ [ -n "${UPD_SHA}" ] && [ "${UPD_SHA}" != "null" ]; } || POLL_OK=0
{ [ -n "${UPD_SIG}" ] && [ "${UPD_SIG}" != "null" ]; } || POLL_OK=0
# the device must receive the SAME signed artifact it would download: sha echoes
# the uploaded target artifact's sha256.
[ "${UPD_SHA}" = "${TARGET_ART_SHA}" ] || POLL_OK=0
{ [ -n "${DELTA_BLOCK}" ] && [ "${DELTA_BLOCK}" != "null" ]; } || POLL_OK=0
[ "${DELTA_BVER}" = "1.0.0" ] || POLL_OK=0
[ "${DELTA_BSHA}" = "${DELTA_SHA}" ] || POLL_OK=0

if [ "${POLL_OK}" = "1" ]; then
  ab_pass_with_evidence "step 14 DEVICE POLL (the payoff): device on 1.0.0 polls /client/update and RECEIVES the correct SIGNED 1.1.0 update through the full real-DB pipeline — version=1.1.0, release matches, sha256 echoes the uploaded signed TARGET artifact, signature present, AND the registered base->target .delta (base_version=1.0.0, sha matches) is served. End-to-end signed OTA flow works against real Postgres." "${EV_POLL}"
else
  ab_fail "step 14 device poll receives signed update" "expected 200 version=1.1.0 release=${TARGET_REL_ID} sha=${TARGET_ART_SHA} +signature +delta(base=1.0.0 sha=${DELTA_SHA}); got HTTP ${UPD_CODE} version=${UPD_VER} release=${UPD_RELID} sha=${UPD_SHA} delta=${DELTA_BLOCK} (body: $(printf '%s' "${HTTP_BODY}" | head -c 280))"
fi

# ---- 14. control: a device already on 1.1.0 gets 204 (no update) ------------
EV_DEV2="${EVIDENCE_DIR}/step15_device_on_target_no_update.txt"
req POST "${API}/devices/register" "$(jq -nc --arg hw "hw2-${RUN_TAG}" \
  '{hardware_id:$hw,model:"OrangePi5Max",os:"android",current_version:"1.1.0"}')" "" "${EV_DEV2}" >/dev/null
DEVICE2_TOKEN="$(jqget '.device_token')"
EV_POLL2="${EVIDENCE_DIR}/step15_device_on_target_poll.txt"
req GET "${API}/client/update" "" "${DEVICE2_TOKEN}" "${EV_POLL2}" >/dev/null
log "step 15 device-on-1.1.0 poll: HTTP ${HTTP_STATUS}"
if [ "${HTTP_STATUS}" = "204" ]; then
  ab_pass_with_evidence "step 15 control: a device already on 1.1.0 polling /client/update gets 204 (no update) — the live pipeline does not over-serve" "${EV_POLL2}"
else
  ab_fail "step 15 device-on-1.1.0 gets 204" "expected 204 no-content, got HTTP ${HTTP_STATUS} (body: $(printf '%s' "${HTTP_BODY}" | head -c 200))"
fi

# ---- summary -----------------------------------------------------------------
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
ab_summary
RC=$?
{
  echo "=== pipeline_signed_live SUMMARY ==="
  echo "base_url=${BASE_URL}"
  echo "PASS=${AB_PASS} FAIL=${AB_FAIL} SKIP=${AB_SKIP}"
  echo "exit=${RC}"
} | tee -a "${EVIDENCE_DIR}/SUMMARY.txt" >/dev/null
exit "${RC}"
