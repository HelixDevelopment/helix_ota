#!/usr/bin/env bash
# =============================================================================
# trust_boundary_live.sh — OTA artifact-signature TRUST BOUNDARY, proven at
#   RUNTIME against the REAL live system (anti-bluff, §11.4.69 / §11.4.123).
# -----------------------------------------------------------------------------
# Purpose:
#   Prove the artifact-signature trust boundary
#   (server/internal/api/handlers_artifact.go:resolvePublicKey) holds on the
#   LIVE control plane — the ed25519 verification key comes ONLY from server
#   configuration (HELIX_ARTIFACT_PUBKEY), NEVER from the request. The
#   ota-artifact-validator submodule already mutation-proves the ed25519 stage at
#   the UNIT level; this complements it by proving the SAME property end-to-end on
#   the running HTTP server backed by real PostgreSQL.
#
#   This test is driven against the system booted in CALLER-PUBKEY MODE
#   (tests/lib/boot_real_system.sh + HELIX_SYSTEM_SIGNING_KEY): the caller owns
#   an ed25519 keypair, the live server is configured to trust the caller's
#   PUBLIC key, and the caller signs artifacts with the matching PRIVATE key. That
#   makes the positive (accept-valid) path drivable on the live system instead of
#   SKIPping it, and lets the negative cases prove the rejection paths for real.
#
#   Three runtime assertions, each via a real HTTP call with captured response:
#     (a) POSITIVE          — artifact signed by the caller's matching private
#                             key is ACCEPTED (201, .verified==true).
#     (b) NEGATIVE bad-sig  — same artifact, corrupted signature => REJECTED
#                             (422 SIGNATURE_INVALID).
#     (c) NEGATIVE key-pin  — THE trust boundary: a DIFFERENT (attacker) keypair
#                             signs the artifact AND the attacker pubkey is
#                             supplied via the request (header + metadata field).
#                             The server MUST IGNORE the request-supplied key and
#                             REJECT (422 SIGNATURE_INVALID) — proving the request
#                             cannot override the config-trusted key.
#
#   Signing contract (identical to tests/e2e/pipeline_signed.sh, verified against
#   ota-artifact-validator stages.go ValidateHash/ValidateSignature):
#     sha256_hex = lowercase-hex SHA-256 over the WHOLE ZIP bytes
#     signature  = base64( ed25519.Sign(priv, hex_decode(sha256_hex)) )
#                  i.e. ed25519 signs the raw 32 DIGEST bytes, NOT the file.
#
# Keys are EPHEMERAL: generated into a mktemp dir, .gitignore-excluded, rm -rf'd
#   on exit, NEVER committed (§11.4.10). Captured evidence is redacted of any
#   token/key material before it is written under docs/qa/.
#
# Usage:
#   # caller-pubkey mode against the live F-CLUSTER (boots, runs, tears down):
#   bash tests/security/trust_boundary_live.sh
#   # against an already-booted base URL (skip boot/teardown):
#   BASE_URL=http://127.0.0.1:18080 bash tests/security/trust_boundary_live.sh --no-boot
#
# Env:
#   TARGET / REMOTE_USER   passed through to boot_real_system.sh (live host).
#   BASE_URL               override the live base URL (else taken from boot).
#   ADMIN_USER/ADMIN_PW    live admin login (defaults match system.compose.yml).
#   EVIDENCE_DIR           where redacted evidence is written
#                          (default docs/qa/20260623-trust-boundary-live).
#
# Dependencies: bash, openssl(>=3 ed25519), xxd, base64, curl, jq, python3, ssh.
# Cross-references: server/internal/api/handlers_artifact.go (resolvePublicKey),
#   tests/lib/boot_real_system.sh (caller-pubkey mode), tests/lib/anti_bluff.sh,
#   tests/e2e/pipeline_signed.sh (signing recipe), server/deploy/system.compose.yml.
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
BOOT="${REPO_ROOT}/tests/lib/boot_real_system.sh"

# shellcheck source=/dev/null
. "${REPO_ROOT}/tests/lib/anti_bluff.sh"

EVIDENCE_DIR="${EVIDENCE_DIR:-${REPO_ROOT}/docs/qa/20260623-trust-boundary-live}"
mkdir -p "${EVIDENCE_DIR}"
RUN_LOG="${EVIDENCE_DIR}/run.log"
: > "${RUN_LOG}"

ADMIN_USER="${ADMIN_USER:-admin@helix.system}"
ADMIN_PW="${ADMIN_PW:-ephemeral-test-stack-NOT-A-SECRET}"  # §11.4.10 non-secret test constant (matches system.compose.yml)

DO_BOOT=1
[ "${1:-}" = "--no-boot" ] && DO_BOOT=0

WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-trustb.XXXXXX")"
BOOTED=0

log() { printf '%s %s\n' "[$(date -u +%H:%M:%SZ)]" "$*" | tee -a "${RUN_LOG}" >&2; }

# Redact bearer tokens, Authorization headers, and base64 key/sig blobs so no
# secret material lands in committed evidence (§11.4.10).
redact() {
  sed -E \
    -e 's/(Authorization: Bearer )[A-Za-z0-9._-]+/\1<REDACTED>/g' \
    -e 's/("access_token"[[:space:]]*:[[:space:]]*")[^"]+/\1<REDACTED>/g' \
    -e 's/("device_token"[[:space:]]*:[[:space:]]*")[^"]+/\1<REDACTED>/g'
}

cleanup() {
  if [ "${BOOTED}" = "1" ]; then
    log "TEARDOWN: tearing down the live system stack (project-scoped; pg of the integration suite untouched)"
    bash "${BOOT}" --down >>"${RUN_LOG}" 2>&1 || true
  fi
  rm -rf "${WORK}" 2>/dev/null || true
}
trap cleanup EXIT INT TERM

ab_init "trust_boundary_live"

# ---- 0. ephemeral keypairs ---------------------------------------------------
# CALLER (trusted): the live server will be configured to trust THIS public key.
CALLER_PRIV="${WORK}/caller_priv.pem"
# ATTACKER (untrusted): signs the request-supplied-key attack; the server must
# never trust it regardless of what the request advertises.
ATTACK_PRIV="${WORK}/attacker_priv.pem"
for kf in "${CALLER_PRIV}" "${ATTACK_PRIV}"; do
  openssl genpkey -algorithm ed25519 -out "${kf}" 2>/dev/null \
    || { log "ABORT: openssl could not generate an ed25519 key"; ab_fail "ed25519 keygen" "openssl ed25519 unsupported"; ab_summary; exit 3; }
done
pubkey_b64() { openssl pkey -in "$1" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 | tr -d '\n'; }
CALLER_PUB_B64="$(pubkey_b64 "${CALLER_PRIV}")"
ATTACK_PUB_B64="$(pubkey_b64 "${ATTACK_PRIV}")"
[ "$(printf '%s' "${CALLER_PUB_B64}" | base64 -d 2>/dev/null | wc -c | tr -d ' ')" = "32" ] \
  || { log "ABORT: caller pubkey not 32 bytes"; ab_fail "caller pubkey" "wrong length"; ab_summary; exit 3; }
log "keypairs ready (caller=trusted, attacker=untrusted; both ephemeral, never committed)"

# ---- 1. obtain a LIVE base URL (caller-pubkey mode) --------------------------
if [ "${DO_BOOT}" = "1" ]; then
  log "BOOT: booting the REAL system in CALLER-PUBKEY MODE (server trusts the caller's pubkey)"
  BOOT_OUT="$(HELIX_SYSTEM_SIGNING_KEY="${CALLER_PRIV}" bash "${BOOT}" --up 2>>"${RUN_LOG}")" || {
    log "ABORT: boot_real_system.sh --up failed (see ${RUN_LOG})"
    ab_skip_with_reason "live trust-boundary suite" topology_unsupported || true
    ab_summary; exit 3
  }
  BOOTED=1
  BASE_URL="${BASE_URL:-$(printf '%s\n' "${BOOT_OUT}" | sed -n 's/^BASE_URL=//p' | tail -1)}"
fi
: "${BASE_URL:?BASE_URL not set (boot failed or --no-boot without BASE_URL)}"
API="/api/v1"
log "live base_url=${BASE_URL}"

# When the live system is on a remote host reachable only via ssh loopback, route
# HTTP through the remote (the boot harness probes 127.0.0.1:<port> on the remote).
# If BASE_URL is a 127.0.0.1 URL and a TARGET is set, tunnel curl over ssh.
REMOTE_USER="${REMOTE_USER:-milosvasic}"
TARGET="${TARGET:-${REMOTE_USER}@thinker.local}"
case "${TARGET}" in *@*) ;; *) TARGET="${REMOTE_USER}@${TARGET}" ;; esac
SSH="ssh -o BatchMode=yes -o ConnectTimeout=15"
USE_SSH=0
case "${BASE_URL}" in *127.0.0.1*|*localhost*) [ "${DO_BOOT}" = "1" ] && USE_SSH=1 ;; esac

# curl_live <curl-args...> : run curl against the live server, locally or via the
# remote loopback. Prints the curl stdout; sets no globals.
curl_live() {
  if [ "${USE_SSH}" = "1" ]; then
    # Build a single remote curl command. Args are already fully formed.
    $SSH "${TARGET}" "curl $*"
  else
    eval "curl $*"
  fi
}

# ---- 2. live readiness + login ----------------------------------------------
READY_CODE="$(curl_live "-sS -o /dev/null -w '%{http_code}' --max-time 6 '${BASE_URL}/readyz'" 2>/dev/null || echo 000)"
if [ "${READY_CODE}" != "200" ]; then
  log "live /readyz not 200 (got ${READY_CODE}) — system unreachable"
  ab_skip_with_reason "live trust-boundary suite (system not reachable)" network_unreachable_external || true
  ab_summary; exit 3
fi
log "live /readyz -> 200"

LOGIN_BODY="$(jq -nc --arg u "${ADMIN_USER}" --arg p "${ADMIN_PW}" '{username:$u,password:$p}')"
LOGIN_OUT="${WORK}/login.json"
LOGIN_CODE="$(curl_live "-sS -o - -w '\n%{http_code}' -X POST '${BASE_URL}${API}/auth/login' -H 'Content-Type: application/json' --data '${LOGIN_BODY}'" 2>/dev/null > "${LOGIN_OUT}"; tail -1 "${LOGIN_OUT}")"
TOKEN="$(sed '$d' "${LOGIN_OUT}" | jq -r '.access_token' 2>/dev/null)"
if [ -z "${TOKEN}" ] || [ "${TOKEN}" = "null" ]; then
  log "ABORT: live login failed (HTTP ${LOGIN_CODE})"
  ab_fail "live admin login" "no access_token from /auth/login (HTTP ${LOGIN_CODE})"
  ab_summary; exit 1
fi
log "obtained live admin token (redacted in evidence)"

# ---- helpers: build a ZIP_STORED artifact + sign a digest --------------------
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

# upload an artifact to the LIVE server. Args:
#   $1 zip  $2 version  $3 sig_b64  $4 evidence-file
#   $5 (optional) attacker-pubkey-b64 to inject via the request HEADER
#                 (X-Helix-Artifact-Pubkey) — the key-pin attack channel.
#   $6 (optional) "metakey" => ALSO inject the key into the metadata JSON as a
#                 public_key field (the strict-schema probe; expected 400).
# Captures "<body>\n<httpcode>" into the evidence file (redacted). Echoes the code.
live_upload() {
  _u_zip="$1"; _u_ver="$2"; _u_sig="$3"; _u_ev="$4"; _u_reqkey="${5:-}"; _u_metakey="${6:-}"
  _u_sha="$(sha256_hex_of_file "${_u_zip}")"
  if [ "${_u_metakey}" = "metakey" ] && [ -n "${_u_reqkey}" ]; then
    # Strict-schema probe: try to smuggle the key as a metadata field. The
    # ArtifactUploadMetadata schema has NO key field and DisallowUnknownFields,
    # so this is expected to be rejected at the metadata-parse layer (400).
    _u_meta="$(jq -nc --arg sha "${_u_sha}" --arg sig "${_u_sig}" --arg ver "${_u_ver}" \
        --arg os "android" --arg tm "OrangePi5Max" --arg rk "${_u_reqkey}" \
        '{sha256:$sha,signature:$sig,version:$ver,os:$os,target_model:$tm,public_key:$rk}')"
  else
    _u_meta="$(jq -nc --arg sha "${_u_sha}" --arg sig "${_u_sig}" --arg ver "${_u_ver}" \
        --arg os "android" --arg tm "OrangePi5Max" \
        '{sha256:$sha,signature:$sig,version:$ver,os:$os,target_model:$tm}')"
  fi
  # When tunneling via ssh, the ZIP must be readable on the remote: stage it there.
  if [ "${USE_SSH}" = "1" ]; then
    _u_remote="/tmp/$(basename "${_u_zip}").$$"
    scp -q -o BatchMode=yes "${_u_zip}" "${TARGET}:${_u_remote}" >/dev/null 2>&1 || { echo "SCP_FAIL"; return 1; }
    _u_filearg="${_u_remote}"
    _u_runner() { $SSH "${TARGET}" "$1"; }
  else
    _u_filearg="${_u_zip}"
    _u_runner() { eval "$1"; }
  fi
  # The request-supplied attacker key is ALSO offered as an HTTP header
  # (X-Helix-Artifact-Pubkey) in addition to the metadata fields — covering both
  # plausible request channels for the key-pin attack.
  _u_hdr=""
  [ -n "${_u_reqkey}" ] && _u_hdr="-H 'X-Helix-Artifact-Pubkey: ${_u_reqkey}'"
  _u_cmd="curl -sS -o - -w '\n%{http_code}' -X POST '${BASE_URL}${API}/artifacts/upload' \
      -H 'Authorization: Bearer ${TOKEN}' ${_u_hdr} \
      -F 'file=@${_u_filearg};type=application/zip;filename=ota.zip' \
      -F 'metadata=${_u_meta};type=application/json'"
  _u_out="$(_u_runner "${_u_cmd}" 2>/dev/null || true)"
  [ "${USE_SSH}" = "1" ] && $SSH "${TARGET}" "rm -f '${_u_remote}'" >/dev/null 2>&1 || true
  printf '%s\n' "${_u_out}" | redact > "${_u_ev}"
  printf '%s\n' "${_u_out}" | tail -1
}

# ---- 3. CASE (a) POSITIVE: caller-signed artifact is ACCEPTED -----------------
GOOD_ZIP="${WORK}/good.zip"; build_zip_stored "${GOOD_ZIP}" "trust-boundary-positive-$(date +%s)"
GOOD_SHA="$(sha256_hex_of_file "${GOOD_ZIP}")"
GOOD_SIG="$(sign_digest_hex "${CALLER_PRIV}" "${GOOD_SHA}")" || {
  log "openssl ed25519 signing failed — positive path cannot be driven"
  ab_skip_with_reason "case (a) positive accept-valid (signing unavailable)" hardware_not_present || true
  ab_summary; exit 3
}
EV_A="${EVIDENCE_DIR}/case_a_positive_accept.txt"
CODE_A="$(live_upload "${GOOD_ZIP}" "1.0.0" "${GOOD_SIG}" "${EV_A}")"
BODY_A="$(sed '$d' "${EV_A}" 2>/dev/null)"
VERIFIED_A="$(printf '%s' "${BODY_A}" | jq -r '.verified' 2>/dev/null)"
log "CASE (a) positive: HTTP ${CODE_A} verified=${VERIFIED_A}"
if [ "${CODE_A}" = "201" ] && [ "${VERIFIED_A}" = "true" ]; then
  ab_pass_with_evidence "case (a) POSITIVE: caller-signed artifact ACCEPTED by live server (201, verified=true) — config-trusted key verifies the matching signature" "${EV_A}"
else
  ab_fail "case (a) POSITIVE accept-valid" "expected 201 verified=true, got HTTP ${CODE_A} verified=${VERIFIED_A} (body: $(printf '%s' "${BODY_A}" | head -c 200))"
fi

# ---- 4. CASE (b) NEGATIVE bad-sig: wrong signature is REJECTED ----------------
# A structurally-VALID base64 ed25519 signature that does NOT match the artifact:
# the caller signs a DIFFERENT (fixed) digest. This is a well-formed signature
# that fails verification of THIS artifact's digest under the trusted key — so the
# rejection is the S3 SIGNATURE_INVALID we want (not a 400 base64/parse error).
WRONG_DIGEST_HEX="deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
BAD_SIG="$(sign_digest_hex "${CALLER_PRIV}" "${WRONG_DIGEST_HEX}")"
[ -n "${BAD_SIG}" ] || BAD_SIG="$(printf 'definitely-not-a-valid-signature' | base64 | tr -d '\n')"
EV_B="${EVIDENCE_DIR}/case_b_bad_signature_reject.txt"
CODE_B="$(live_upload "${GOOD_ZIP}" "1.0.1" "${BAD_SIG}" "${EV_B}")"
BODY_B="$(sed '$d' "${EV_B}" 2>/dev/null)"
ERRCODE_B="$(printf '%s' "${BODY_B}" | jq -r '.error.code' 2>/dev/null)"
log "CASE (b) bad-sig: HTTP ${CODE_B} error.code=${ERRCODE_B}"
if [ "${CODE_B}" = "422" ] && [ "${ERRCODE_B}" = "SIGNATURE_INVALID" ]; then
  ab_pass_with_evidence "case (b) NEGATIVE bad-signature: live server REJECTS corrupted signature (422 SIGNATURE_INVALID) — proves it really verifies" "${EV_B}"
else
  ab_fail "case (b) NEGATIVE bad-signature reject" "expected 422 SIGNATURE_INVALID, got HTTP ${CODE_B} code=${ERRCODE_B} (body: $(printf '%s' "${BODY_B}" | head -c 200))"
fi

# ---- 5. CASE (c) NEGATIVE key-pin: request-supplied key is IGNORED -----------
# THE trust boundary. An ATTACKER keypair signs the artifact, and the attacker's
# PUBLIC key is supplied via the request HEADER (X-Helix-Artifact-Pubkey). If the
# server honored the request key, this would verify and be ACCEPTED (201) — a
# signature-bypass. The server MUST ignore the request key, verify against its
# CONFIG key (the caller's pubkey, which did NOT sign this), and REJECT (422
# SIGNATURE_INVALID).
ATTACK_ZIP="${WORK}/attack.zip"; build_zip_stored "${ATTACK_ZIP}" "trust-boundary-keypin-attack-$(date +%s)"
ATTACK_SHA="$(sha256_hex_of_file "${ATTACK_ZIP}")"
ATTACK_SIG="$(sign_digest_hex "${ATTACK_PRIV}" "${ATTACK_SHA}")" || {
  ab_skip_with_reason "case (c) key-pin (attacker signing unavailable)" hardware_not_present || true
  ab_summary; exit 3
}
EV_C="${EVIDENCE_DIR}/case_c_request_key_ignored_reject.txt"
CODE_C="$(live_upload "${ATTACK_ZIP}" "1.0.2" "${ATTACK_SIG}" "${EV_C}" "${ATTACK_PUB_B64}")"
BODY_C="$(sed '$d' "${EV_C}" 2>/dev/null)"
ERRCODE_C="$(printf '%s' "${BODY_C}" | jq -r '.error.code' 2>/dev/null)"
log "CASE (c) key-pin: HTTP ${CODE_C} error.code=${ERRCODE_C} (attacker pubkey supplied via X-Helix-Artifact-Pubkey header)"
if [ "${CODE_C}" = "201" ]; then
  # The attack SUCCEEDED — the request key overrode the config key. Hard FAIL.
  ab_fail "case (c) NEGATIVE request-supplied-key-ignored (TRUST BOUNDARY)" "CRITICAL: server ACCEPTED (201) an artifact signed by an attacker key supplied via the request header — the request OVERRODE the config-trusted key (signature-verification bypass). body: $(printf '%s' "${BODY_C}" | head -c 200)"
elif [ "${CODE_C}" = "422" ] && [ "${ERRCODE_C}" = "SIGNATURE_INVALID" ]; then
  ab_pass_with_evidence "case (c) NEGATIVE request-supplied-key-IGNORED (TRUST BOUNDARY): attacker-signed artifact + attacker pubkey in the request header => live server IGNORES the request key, verifies against the CONFIG key, REJECTS (422 SIGNATURE_INVALID). The request cannot override the trusted key." "${EV_C}"
else
  ab_fail "case (c) NEGATIVE request-supplied-key-ignored (TRUST BOUNDARY)" "expected 422 SIGNATURE_INVALID, got HTTP ${CODE_C} code=${ERRCODE_C} (body: $(printf '%s' "${BODY_C}" | head -c 200))"
fi

# ---- 5b. CASE (c2) secondary probe: metadata has NO key field at all ----------
# Complement to (c): the request body schema (ArtifactUploadMetadata) carries NO
# verification-key field AND uses DisallowUnknownFields, so an attacker cannot
# even smuggle the key as a metadata field — the metadata-parse layer rejects it
# (400 VALIDATION_FAILED unknown-field). This proves the metadata channel is
# closed to a key by construction (defence-in-depth for the trust boundary).
EV_C2="${EVIDENCE_DIR}/case_c2_metadata_key_field_rejected.txt"
CODE_C2="$(live_upload "${ATTACK_ZIP}" "1.0.3" "${ATTACK_SIG}" "${EV_C2}" "${ATTACK_PUB_B64}" "metakey")"
BODY_C2="$(sed '$d' "${EV_C2}" 2>/dev/null)"
ISSUE_C2="$(printf '%s' "${BODY_C2}" | jq -r '.error.details[0].issue' 2>/dev/null)"
log "CASE (c2) metadata-key-field: HTTP ${CODE_C2} issue=${ISSUE_C2}"
if [ "${CODE_C2}" = "201" ]; then
  ab_fail "case (c2) metadata-key-field rejected" "CRITICAL: server ACCEPTED (201) an attacker artifact with a public_key smuggled into metadata — the metadata channel carries a trusted key (bypass)."
elif [ "${CODE_C2}" = "400" ] && printf '%s' "${ISSUE_C2}" | grep -q 'public_key'; then
  ab_pass_with_evidence "case (c2) metadata-key-field REJECTED: the metadata schema has NO key field + DisallowUnknownFields => a public_key smuggled into metadata is rejected at parse (400 unknown-field). The metadata channel cannot carry a verification key by construction." "${EV_C2}"
else
  # Any other non-201 rejection (e.g. 422 because the field is silently dropped
  # then sig fails) is still a SAFE outcome — the attack did not succeed — but we
  # assert the precise expected 400 to keep the property exact.
  ab_fail "case (c2) metadata-key-field rejected" "expected 400 unknown-field public_key, got HTTP ${CODE_C2} issue=${ISSUE_C2} (body: $(printf '%s' "${BODY_C2}" | head -c 200))"
fi

# ---- summary -----------------------------------------------------------------
log "finished: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
ab_summary
RC=$?
{
  echo "=== trust_boundary_live SUMMARY ==="
  echo "base_url=${BASE_URL}"
  echo "PASS=${AB_PASS} FAIL=${AB_FAIL} SKIP=${AB_SKIP}"
  echo "exit=${RC}"
} | tee -a "${EVIDENCE_DIR}/SUMMARY.txt" >/dev/null
exit "${RC}"
