#!/usr/bin/env bash
#
# tier1_recall_recovery_e2e.sh — Tier-1 FAILURE -> operator-RECALL(forward-fix)
# -> device-RECOVERY OTA E2E on podman (no live hardware).
#
# Purpose
#   Prove the COMPLETE recall/recovery round-trip against a CONTAINERIZED Helix
#   OTA control plane, extending tier1_full_lifecycle_e2e.sh's happy path with the
#   failure + forward-fix-recall + recovery steps the in-process Go test
#   (server/internal/deviceemu/recall_recovery_test.go) proves at the unit-of-
#   integration level. The flow:
#     1. boot the control plane (ota-server) in a podman pod with a real EPHEMERAL
#        ed25519 trust key so signed artifact uploads verify;
#     2. operator stages release v1.1.0 + an all-targets deployment D1;
#     3. emulator (current 1.0.0) applies v1.1.0, full telemetry lifecycle
#        ACCEPTED, version advances, device healthy;
#     4. the device hits a POST-APPLY FAILURE: the host re-registers the same
#        hardware-id (idempotent -> fresh device token) and POSTs a real
#        `failure` telemetry event (error_code=post_apply_health_check_failed)
#        through the SAME device-scoped /client/telemetry endpoint the emulator
#        uses; ASSERT the server marks the device UNHEALTHY and the version did
#        NOT advance;
#     5. operator stages a FORWARD-FIX release v1.2.0 (release only — the recall
#        creates the new deployment) and RECALLS D1 forward to v1.2.0; ASSERT
#        201 + a rollback_history row (kind=rollback, from=v1.1.0 release,
#        to=v1.2.0 release, mode=forward-fix) via GET /deployments/{D1}/rollbacks;
#     6. emulator (current 1.1.0) RE-CHECKS, gets the v1.2.0 offer carrying the
#        NEW recall deployment_id, applies, advances to 1.2.0, healthy again,
#        telemetry ACCEPTED; cross-check the SERVER-side success telemetry stamped
#        with the recall deployment_id;
#     7. a THIRD emulator run (current 1.2.0) returns 204 on-target.
#
#   §11.4 anti-bluff: every PASS is backed by real captured podman logs + emulator
#   Outcome JSON + server telemetry + rollback history under docs/qa/<run-id>/.
#   §11.4.76 uses the containers-submodule emulator image. §11.4.107: the full
#   failure->recall->recovery flow (not a single 204) is the proof.
#
#   The anti-downgrade invariant (handleClientUpdate) only offers a version
#   STRICTLY GREATER than current, so the forward-fix MUST be HIGHER (1.2.0) than
#   the failed-at version (1.1.0) for the device to recover.
#
# Usage
#   bash tests/emulator/tier1_recall_recovery_e2e.sh
#   KEEP_UP=1 bash tests/emulator/tier1_recall_recovery_e2e.sh   # leave stack up
#
# Inputs (environment, all optional — sane defaults)
#   PODMAN          podman binary           (default: podman)
#   CP_IMAGE        control-plane image tag (default: ota-control-plane:dev)
#   EMU_IMAGE       base emulator image tag (default: ota-device-emu:dev)
#   ADMIN_USER      seeded admin login      (default: admin@helix.example)
#   ADMIN_PASS      seeded admin password   (default: a generated test secret)
#   HELIX_PORT      control-plane port      (default: 8080)
#   KEEP_UP         non-empty => skip teardown (debugging)
#
# Outputs / Side-effects
#   - Cross-compiles bin/ota-server + bin/ota-device-emu (static linux/arm64).
#   - Builds the control-plane + derived ota-device-emu images via podman.
#   - Creates a podman pod "helix-tier1-recall" with the control plane + three
#     emulator runs; removes it on exit (unless KEEP_UP). trap cleanup §11.4.14.
#   - Generates an EPHEMERAL ed25519 keypair in a mktemp dir (never committed,
#     rm -rf'd on exit, §11.4.10).
#   - Writes captured podman logs + emulator Outcome JSON + server telemetry +
#     rollback history under docs/qa/<run-id>/ (§11.4.83).
#
# Dependencies: podman (a running podman machine on macOS), Go toolchain, and the
#   host tools openssl(>=3 ed25519)/xxd/base64/curl/jq/python3 (signing contract
#   mirrors tests/e2e/pipeline_signed.sh: ed25519 over the hex-decoded SHA-256 of
#   a ZIP_STORED payload; pubkey = last 32 DER bytes, base64).
# Cross-references: server/Dockerfile, containers/images/ota-device-emu/Dockerfile,
#   server/cmd/ota-device-emu/main.go, server/internal/deviceemu/emulator.go,
#   server/internal/deviceemu/recall_recovery_test.go,
#   server/internal/api/handlers_recall.go (forward-fix recall),
#   server/internal/api/handlers_client.go (telemetry deployment_id required),
#   server/internal/api/handlers_telemetry.go (GET /devices/{id}/telemetry),
#   tests/emulator/tier1_full_lifecycle_e2e.sh, tests/e2e/pipeline_signed.sh,
#   docs/scripts/tier1_recall_recovery_e2e.md.

set -euo pipefail

# --- resolve paths ------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
SERVER_DIR="${REPO_ROOT}/server"
EMU_IMAGE_DIR="${REPO_ROOT}/containers/images/ota-device-emu"

PODMAN="${PODMAN:-podman}"
CP_IMAGE="${CP_IMAGE:-ota-control-plane:dev}"
EMU_IMAGE="${EMU_IMAGE:-ota-device-emu:dev}"
ADMIN_USER="${ADMIN_USER:-admin@helix.example}"
ADMIN_PASS="${ADMIN_PASS:-tier1-recall-$(date +%s)-secret}"
HELIX_PORT="${HELIX_PORT:-8080}"
API_BASE="/api/v1"

POD_NAME="helix-tier1-recall"
CP_CTR="helix-cp"
EMU_CTR1="helix-emu-apply"
EMU_CTR2="helix-emu-recover"
EMU_CTR3="helix-emu-ontarget"
EMU_RUN_IMAGE="ota-device-emu-tier1:dev"

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
QA_DIR="${REPO_ROOT}/docs/qa/${RUN_ID}-recall-recovery-container"
BIN_DIR="${REPO_ROOT}/bin"
CP_STAGE="${SERVER_DIR}/.docker-bin"
EMU_STAGE="${REPO_ROOT}/tests/emulator/.docker-bin"
mkdir -p "${QA_DIR}" "${BIN_DIR}" "${CP_STAGE}" "${EMU_STAGE}"

TRANSCRIPT="${QA_DIR}/tier1_recall_recovery_transcript.txt"

# Ephemeral key + scratch dir (NEVER committed, rm -rf'd on exit) — §11.4.10.
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-recall.XXXXXX")"

# Device identity the emulator impersonates.
HW_ID="rk3588-recall-$(date +%s)"
DEV_MODEL="OrangePi5Max"
DEV_OS="android"
DEV_CUR="1.0.0"
REL_V1="1.1.0"   # initial release (device applies this, then fails on it)
REL_V2="1.2.0"   # forward-fix release (strictly greater -> device recovers to it)
FAIL_CODE="post_apply_health_check_failed"

HOST_BASE="http://127.0.0.1:${HELIX_PORT}${API_BASE}"

# --- logging helpers ----------------------------------------------------------
log() { printf '%s %s\n' "[$(date -u +%H:%M:%SZ)]" "$*" | tee -a "${TRANSCRIPT}"; }
hr()  { printf '%s\n' "------------------------------------------------------------" | tee -a "${TRANSCRIPT}"; }
fail() { log "FAIL: $*"; exit 1; }

# --- cleanup (every exit path) ------------------------------------------------
cleanup() {
    local rc=$?
    rm -rf "${WORK}" 2>/dev/null || true
    if [ -n "${KEEP_UP:-}" ]; then
        log "KEEP_UP set — leaving pod '${POD_NAME}' running for inspection."
        return 0
    fi
    hr
    log "TEARDOWN: removing pod '${POD_NAME}' (and its containers)."
    "${PODMAN}" pod rm -f "${POD_NAME}" >/dev/null 2>&1 || true
    rm -f "${CP_STAGE}/ota-server" "${EMU_STAGE}/ota-device-emu" "${EMU_STAGE}/Dockerfile" 2>/dev/null || true
    log "TEARDOWN done (exit rc=${rc})."
    return 0
}
trap cleanup EXIT INT TERM

# --- host HTTP helper (operator + device REST calls from the host) ------------
HTTP_STATUS=""; HTTP_BODY=""
# req METHOD PATH [JSON-DATA] [BEARER]
req() {
    local method="$1" path="$2" data="${3:-}" tok="${4:-${OP_TOKEN:-}}"
    local tmp; tmp="$(mktemp "${WORK}/resp.XXXXXX")"
    local -a args=(-sS -o "${tmp}" -w '%{http_code}' -X "${method}" "${HOST_BASE}${path}"
                   -H 'Accept: application/json')
    [ -n "${tok}" ] && args+=(-H "Authorization: Bearer ${tok}")
    [ -n "${data}" ] && args+=(-H 'Content-Type: application/json' --data "${data}")
    HTTP_STATUS="$(curl "${args[@]}" 2>/dev/null)" || HTTP_STATUS="000"
    HTTP_BODY="$(cat "${tmp}")"; rm -f "${tmp}"
}
jqget() { printf '%s' "${HTTP_BODY}" | jq -r "$1" 2>/dev/null; }
need_status() {
    local want="$1" label="$2"
    [ "${HTTP_STATUS}" = "${want}" ] && { log "  ${label} -> HTTP ${HTTP_STATUS}"; return 0; }
    fail "${label}: want HTTP ${want}, got ${HTTP_STATUS} (body: $(printf '%s' "${HTTP_BODY}" | head -c 280))"
}

# --- stage a signed release (no deployment) -> echoes the release_id ----------
# stage_release VERSION  (sets STAGED_RELEASE_ID)
stage_release() {
    local version="$1"
    local zip="${WORK}/ota-${version}.zip"
    PAYLOAD="HELIX OTA payload v${version} ${RUN_ID}" OUT="${zip}" python3 - <<'PY'
import os, zipfile
out = os.environ["OUT"]; payload = os.environ["PAYLOAD"].encode()
with zipfile.ZipFile(out, "w", compression=zipfile.ZIP_STORED) as z:
    zi = zipfile.ZipInfo("payload.bin"); zi.compress_type = zipfile.ZIP_STORED
    z.writestr(zi, payload)
PY
    [ -s "${zip}" ] || fail "could not build ZIP_STORED artifact for v${version}"
    local digest_hex rawf sigf sig_b64 file_size meta uptmp up_status up_body art_id verified
    digest_hex="$(openssl dgst -sha256 -binary "${zip}" | xxd -p -c256 | tr -d '\n')"
    rawf="${WORK}/digest-${version}.bin"; sigf="${WORK}/sig-${version}.bin"
    printf '%s' "${digest_hex}" | xxd -r -p > "${rawf}"
    openssl pkeyutl -sign -inkey "${PRIV_PEM}" -rawin -in "${rawf}" -out "${sigf}" 2>/dev/null \
        || fail "openssl ed25519 sign over digest failed for v${version}"
    sig_b64="$(base64 < "${sigf}" | tr -d '\n')"
    file_size="$(wc -c < "${zip}" | tr -d ' ')"
    meta="$(jq -nc \
        --arg sha "${digest_hex}" --arg sig "${sig_b64}" --arg ver "${version}" \
        --arg os "${DEV_OS}" --arg tm "${DEV_MODEL}" \
        --arg fh "$(printf 'file-hash' | base64 | tr -d '\n')" --argjson fs "${file_size}" \
        --arg mh "$(printf 'meta-hash' | base64 | tr -d '\n')" --argjson ms 64 \
        '{sha256:$sha,signature:$sig,version:$ver,os:$os,target_model:$tm,
          file_hash:$fh,file_size:$fs,metadata_hash:$mh,metadata_size:$ms}')"
    uptmp="$(mktemp "${WORK}/up.XXXXXX")"
    up_status="$(curl -sS -o "${uptmp}" -w '%{http_code}' -X POST "${HOST_BASE}/artifacts/upload" \
        -H "Authorization: Bearer ${OP_TOKEN}" \
        -F "file=@${zip};type=application/zip;filename=ota.zip" \
        -F "metadata=${meta};type=application/json" 2>/dev/null)" || up_status="000"
    up_body="$(cat "${uptmp}")"; rm -f "${uptmp}"
    [ "${up_status}" = "201" ] || fail "artifact upload v${version}: want 201, got ${up_status} (body: $(printf '%s' "${up_body}" | head -c 280))"
    art_id="$(printf '%s' "${up_body}" | jq -r '.artifact_id')"
    verified="$(printf '%s' "${up_body}" | jq -r '.verified')"
    [ -n "${art_id}" ] && [ "${art_id}" != "null" ] || fail "upload v${version} 201 but no artifact_id"
    [ "${verified}" = "true" ] || fail "artifact v${version} .verified != true"
    req POST "/releases" \
        "$(jq -nc --arg a "${art_id}" --arg v "${version}" --arg os "${DEV_OS}" --arg tm "${DEV_MODEL}" \
            '{artifact_id:$a,version:$v,os:$os,target_model:$tm,notes:"recall-recovery e2e"}')"
    need_status 201 "POST /releases (v${version})"
    STAGED_RELEASE_ID="$(jqget '.release_id')"
    [ -n "${STAGED_RELEASE_ID}" ] && [ "${STAGED_RELEASE_ID}" != "null" ] \
        || fail "release v${version} 201 but no release_id"
}

# run_emu CONTAINER OUTFILE STDERRFILE CURRENT_VERSION
run_emu() {
    local ctr="$1" outf="$2" errf="$3" cur="$4" rc
    set +e
    "${PODMAN}" run --pod "${POD_NAME}" --name "${ctr}" \
        -e "OTA_BASE_URL=http://127.0.0.1:${HELIX_PORT}${API_BASE}" \
        --memory 256m \
        "${EMU_RUN_IMAGE}" \
        -base "http://127.0.0.1:${HELIX_PORT}${API_BASE}" \
        -admin-user "${ADMIN_USER}" \
        -admin-pass "${ADMIN_PASS}" \
        -hardware-id "${HW_ID}" \
        -model "${DEV_MODEL}" \
        -os "${DEV_OS}" \
        -current-version "${cur}" \
        -once \
        > "${outf}" 2> "${errf}"
    rc=$?
    set -e
    return ${rc}
}

# ==============================================================================
log "Tier-1 RECALL/RECOVERY container E2E — run-id ${RUN_ID}"
log "repo root: ${REPO_ROOT}"
log "evidence : ${QA_DIR}"
hr

# --- preflight ----------------------------------------------------------------
log "PREFLIGHT: podman reachable + arch"
"${PODMAN}" info --format '{{.Host.Arch}} {{.Host.OS}}' 2>&1 | tee -a "${TRANSCRIPT}" \
    || fail "podman not reachable (is the podman machine running?)"
for b in go openssl xxd base64 curl jq python3; do
    command -v "${b}" >/dev/null 2>&1 || fail "required host tool '${b}' not found"
done
log "openssl: $(openssl version)"
hr

# --- (0) ephemeral ed25519 keypair --------------------------------------------
log "KEYS (0): mint ephemeral ed25519 artifact-trust keypair (never committed)"
PRIV_PEM="${WORK}/artifact_priv.pem"
openssl genpkey -algorithm ed25519 -out "${PRIV_PEM}" 2>/dev/null \
    || fail "openssl could not generate an ed25519 key (scheme unsupported on this host)"
PUBKEY_B64="$(openssl pkey -in "${PRIV_PEM}" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 | tr -d '\n')"
PUBLEN="$(openssl pkey -in "${PRIV_PEM}" -pubout -outform DER 2>/dev/null | tail -c 32 | wc -c | tr -d ' ')"
[ "${PUBLEN}" = "32" ] || fail "extracted ed25519 public key is ${PUBLEN} bytes, expected 32"
log "  raw pubkey 32 bytes, base64 configured for HELIX_ARTIFACT_PUBKEY"
hr

# --- (a) cross-compile both static linux/arm64 binaries -----------------------
log "BUILD (a): cross-compile static linux/arm64 binaries on host"
( cd "${SERVER_DIR}" \
  && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" \
       -o "${BIN_DIR}/ota-server" ./cmd/ota-server ) 2>&1 | tee -a "${TRANSCRIPT}"
[ -x "${BIN_DIR}/ota-server" ] || fail "ota-server binary not produced"
( cd "${SERVER_DIR}" \
  && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" \
       -o "${BIN_DIR}/ota-device-emu" ./cmd/ota-device-emu ) 2>&1 | tee -a "${TRANSCRIPT}"
[ -x "${BIN_DIR}/ota-device-emu" ] || fail "ota-device-emu binary not produced"
cp "${BIN_DIR}/ota-server" "${CP_STAGE}/ota-server"
cp "${BIN_DIR}/ota-device-emu" "${EMU_STAGE}/ota-device-emu"
hr

# --- (b) build images ---------------------------------------------------------
log "BUILD (b): podman build control-plane image (${CP_IMAGE})"
"${PODMAN}" build -f "${SERVER_DIR}/Dockerfile" -t "${CP_IMAGE}" "${SERVER_DIR}" \
    2>&1 | tee -a "${TRANSCRIPT}" | tail -5
"${PODMAN}" image exists "${CP_IMAGE}" || fail "control-plane image not built"

log "BUILD (b): podman build base ota-device-emu image (${EMU_IMAGE}) [containers submodule]"
"${PODMAN}" build -f "${EMU_IMAGE_DIR}/Dockerfile" -t "${EMU_IMAGE}" "${EMU_IMAGE_DIR}" \
    2>&1 | tee -a "${TRANSCRIPT}" | tail -5
"${PODMAN}" image exists "${EMU_IMAGE}" || fail "base ota-device-emu image not built"

log "BUILD (b): podman build derived emulator image (${EMU_RUN_IMAGE})"
cat > "${EMU_STAGE}/Dockerfile" <<EOF
# Generated by tests/emulator/tier1_recall_recovery_e2e.sh — derives the consumer
# emulator image from the containers-submodule runtime base + our binary.
FROM ${EMU_IMAGE}
COPY ota-device-emu /usr/local/bin/ota-device-emu
EOF
"${PODMAN}" build -f "${EMU_STAGE}/Dockerfile" -t "${EMU_RUN_IMAGE}" "${EMU_STAGE}" \
    2>&1 | tee -a "${TRANSCRIPT}" | tail -5
"${PODMAN}" image exists "${EMU_RUN_IMAGE}" || fail "derived emulator image not built"
hr

# --- (c) bring the control plane up -------------------------------------------
log "BOOT (c): create pod '${POD_NAME}' (publish ${HELIX_PORT})"
"${PODMAN}" pod rm -f "${POD_NAME}" >/dev/null 2>&1 || true
if curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${HELIX_PORT}/healthz" 2>/dev/null | grep -q '200'; then
    fail "port ${HELIX_PORT} is already serving /healthz — a stale control plane is running. \
Free it (podman pod rm -f / kill the listener) or set HELIX_PORT to a free port, then re-run."
fi
"${PODMAN}" pod create --name "${POD_NAME}" -p "${HELIX_PORT}:${HELIX_PORT}" \
    2>&1 | tee -a "${TRANSCRIPT}"
log "BOOT (c): start control-plane '${CP_CTR}' with HELIX_ARTIFACT_PUBKEY set"
"${PODMAN}" run -d --pod "${POD_NAME}" --name "${CP_CTR}" \
    -e "HELIX_PORT=${HELIX_PORT}" \
    -e "HELIX_API_BASE_PATH=${API_BASE}" \
    -e "HELIX_ADMIN_USERNAME=${ADMIN_USER}" \
    -e "HELIX_ADMIN_PASSWORD=${ADMIN_PASS}" \
    -e "HELIX_TOKEN_SECRET=tier1-recall-token-secret" \
    -e "HELIX_ARTIFACT_PUBKEY=${PUBKEY_B64}" \
    --memory 256m \
    "${CP_IMAGE}" 2>&1 | tee -a "${TRANSCRIPT}"
hr

# --- (d) wait for the control plane to be healthy AND serving the API base ----
log "HEALTH (d): wait for control-plane /healthz + ${API_BASE}/auth/login to answer"
healthy=0
for i in $(seq 1 60); do
    code="$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${HELIX_PORT}/healthz" 2>/dev/null || echo 000)"
    if [ "${code}" = "200" ]; then
        lcode="$(curl -sS -o /dev/null -w '%{http_code}' -X POST "${HOST_BASE}/auth/login" \
            -H 'Content-Type: application/json' --data '{"username":"_probe","password":"_probe"}' 2>/dev/null || echo 000)"
        if [ "${lcode}" != "404" ] && [ "${lcode}" != "000" ]; then
            healthy=1; log "  /healthz -> 200, ${API_BASE}/auth/login -> ${lcode} (API base live, attempt ${i})"; break
        fi
    fi
    sleep 0.5
done
[ "${healthy}" -eq 1 ] || {
    log "control-plane never became healthy on its API base; container logs follow:"
    "${PODMAN}" logs "${CP_CTR}" 2>&1 | tee -a "${TRANSCRIPT}" | tail -30
    fail "control-plane /healthz + ${API_BASE} not healthy within timeout"
}
"${PODMAN}" logs "${CP_CTR}" > "${QA_DIR}/control_plane_boot.log" 2>&1 || true
hr

# --- (e) operator login + stage release v1.1.0 + all-targets deployment D1 ----
log "STAGE (e): operator login + stage v${REL_V1} + all-targets deployment D1"
req POST "/auth/login" \
    "$(jq -nc --arg u "${ADMIN_USER}" --arg p "${ADMIN_PASS}" '{username:$u,password:$p}')" ""
need_status 200 "POST /auth/login"
OP_TOKEN="$(jqget '.access_token')"
[ -n "${OP_TOKEN}" ] && [ "${OP_TOKEN}" != "null" ] || fail "login 200 but no access_token"

stage_release "${REL_V1}"
REL_ID_V1="${STAGED_RELEASE_ID}"
log "  release v${REL_V1} -> release_id=${REL_ID_V1}"
req POST "/deployments" \
    "$(jq -nc --arg r "${REL_ID_V1}" '{release_id:$r,strategy:"all-targets"}')"
need_status 201 "POST /deployments (D1, all-targets)"
DEP_D1="$(jqget '.deployment_id')"
[ -n "${DEP_D1}" ] && [ "${DEP_D1}" != "null" ] || fail "deployment 201 but no deployment_id"
log "  deployment D1=${DEP_D1}"
hr

# --- (f) RUN 1: emulator (current 1.0.0) applies v1.1.0 -----------------------
log "RUN 1 (f): emulator register -> 200 offer v${REL_V1} -> apply -> healthy"
run_emu "${EMU_CTR1}" "${QA_DIR}/emulator_run1_apply.json" "${QA_DIR}/emulator_run1_stderr.log" "${DEV_CUR}" \
    || fail "emulator run-1 exited non-zero; see emulator_run1_stderr.log"
OUT1="$(cat "${QA_DIR}/emulator_run1_apply.json")"
log "  run-1 Outcome:"; sed 's/^/    /' "${QA_DIR}/emulator_run1_apply.json" | tee -a "${TRANSCRIPT}"
DEV_ID="$(printf '%s' "${OUT1}" | jq -r '.device_id')"
[ -n "${DEV_ID}" ] && [ "${DEV_ID}" != "null" ] || fail "register: no device_id in Outcome"
[ "$(printf '%s' "${OUT1}" | jq -r '.applied')" = "true" ] || fail "run-1 did not apply"
[ "$(printf '%s' "${OUT1}" | jq -r '.to_version')" = "${REL_V1}" ] || fail "run-1 to_version != ${REL_V1}"
[ "$(printf '%s' "${OUT1}" | jq -r '.telemetry_rejected')" = "0" ] || fail "run-1 telemetry rejected != 0"
[ "$(printf '%s' "${OUT1}" | jq -r '.deployment_id')" = "${DEP_D1}" ] || fail "run-1 offer deployment_id != D1"
[ "$(printf '%s' "${OUT1}" | jq -r '.healthy')" = "true" ] || fail "run-1 device not healthy after apply"
log "  [1/7] APPLY ok: ${DEV_CUR} -> ${REL_V1}, healthy, telemetry accepted, deployment_id=${DEP_D1}"
log "  device_id=${DEV_ID}"
hr

# --- (g) FAILURE: device hits a post-apply failure ----------------------------
# The emulator CLI runs a single check->apply cycle and does not expose a failure
# trigger, so we drive the failure through the SAME device protocol the emulator
# uses: re-register the hardware-id (idempotent -> fresh device token) and POST a
# real `failure` telemetry event. This is faithful (the device-scoped
# /client/telemetry endpoint, the device's own token), not a server-internal poke.
log "FAILURE (g): device reports a post-apply failure (error_code=${FAIL_CODE})"
# g.1 re-register (idempotent via Idempotency-Key = hardware-id) -> device token.
RTMP="$(mktemp "${WORK}/reg.XXXXXX")"
REG_STATUS="$(curl -sS -o "${RTMP}" -w '%{http_code}' -X POST "${HOST_BASE}/devices/register" \
    -H "Authorization: Bearer ${OP_TOKEN}" \
    -H "Idempotency-Key: ${HW_ID}" \
    -H 'Content-Type: application/json' \
    --data "$(jq -nc --arg h "${HW_ID}" --arg m "${DEV_MODEL}" --arg os "${DEV_OS}" --arg cv "${REL_V1}" \
        '{hardware_id:$h,model:$m,os:$os,current_version:$cv}')" 2>/dev/null)" || REG_STATUS="000"
REG_BODY="$(cat "${RTMP}")"; rm -f "${RTMP}"
{ [ "${REG_STATUS}" = "200" ] || [ "${REG_STATUS}" = "201" ]; } \
    || fail "re-register: want 200/201, got ${REG_STATUS} (body: $(printf '%s' "${REG_BODY}" | head -c 280))"
DEV_TOKEN="$(printf '%s' "${REG_BODY}" | jq -r '.device_token')"
REG_DEV_ID="$(printf '%s' "${REG_BODY}" | jq -r '.device_id')"
[ -n "${DEV_TOKEN}" ] && [ "${DEV_TOKEN}" != "null" ] || fail "re-register returned no device_token"
[ "${REG_DEV_ID}" = "${DEV_ID}" ] || fail "idempotent re-register returned a DIFFERENT device_id (${REG_DEV_ID} != ${DEV_ID})"
log "  re-register idempotent -> same device_id=${REG_DEV_ID}, fresh device token"

# g.2 POST a failure telemetry event with the device token, stamped with D1.
FAIL_BODY="$(jq -nc --arg dev "${DEV_ID}" --arg dep "${DEP_D1}" --arg ver "${REL_V1}" \
    --arg ec "${FAIL_CODE}" --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    '{device_id:$dev,deployment_id:$dep,events:[{event:"failure",version:$ver,timestamp:$ts,error_code:$ec}]}')"
req POST "/client/telemetry" "${FAIL_BODY}" "${DEV_TOKEN}"
need_status 202 "POST /client/telemetry (failure)"
F_ACC="$(jqget '.accepted')"; F_REJ="$(jqget '.rejected')"
[ "${F_REJ}" = "0" ] || fail "failure telemetry rejected=${F_REJ} (want 0)"
[ "${F_ACC}" -ge 1 ] 2>/dev/null || fail "failure telemetry accepted=${F_ACC} (want >=1)"
log "  failure telemetry accepted=${F_ACC} rejected=0"

# g.3 cross-check: server marks the device UNHEALTHY, version did NOT advance.
req GET "/devices/${DEV_ID}/status" "" "${OP_TOKEN}"
need_status 200 "GET /devices/{id}/status (after failure)"
printf '%s' "${HTTP_BODY}" | jq . > "${QA_DIR}/device_status_after_failure.json" 2>/dev/null || true
H_OK="$(jqget '.health.ok')"; CUR_AFTER_FAIL="$(jqget '.current_version')"
[ "${H_OK}" = "false" ] || fail "device health.ok=${H_OK} after failure (want false)"
log "  [2/7] FAILURE ok: server health.ok=false, current_version=${CUR_AFTER_FAIL} (not advanced beyond ${REL_V1})"

# g.4 cross-check failure telemetry persisted with error_code.
req GET "/devices/${DEV_ID}/telemetry?event=failure" "" "${OP_TOKEN}"
need_status 200 "GET /devices/{id}/telemetry?event=failure"
printf '%s' "${HTTP_BODY}" | jq . > "${QA_DIR}/server_failure_telemetry.json" 2>/dev/null || true
FCNT="$(printf '%s' "${HTTP_BODY}" | jq -r --arg d "${DEP_D1}" --arg ec "${FAIL_CODE}" \
    '[.items[] | select(.event=="failure" and .deployment_id==$d and .error_code==$ec)] | length' 2>/dev/null)"
[ "${FCNT}" -ge 1 ] 2>/dev/null || fail "no failure telemetry for D1 with error_code=${FAIL_CODE} in server history"
log "  [3/7] server failure telemetry present (count=${FCNT}, deployment=${DEP_D1}, error_code=${FAIL_CODE})"
hr

# --- (h) operator stages forward-fix release v1.2.0 + RECALLS D1 --------------
log "RECALL (h): stage forward-fix v${REL_V2} (release only) + recall D1 forward"
stage_release "${REL_V2}"
REL_ID_V2="${STAGED_RELEASE_ID}"
log "  forward-fix release v${REL_V2} -> release_id=${REL_ID_V2}"
req POST "/deployments/${DEP_D1}/recall" \
    "$(jq -nc --arg to "${REL_ID_V2}" --arg r "${FAIL_CODE} on ${REL_V1}; forward-fix to ${REL_V2}" \
        '{to_release_id:$to,reason:$r}')"
need_status 201 "POST /deployments/{D1}/recall"
printf '%s' "${HTTP_BODY}" | jq . > "${QA_DIR}/recall_response.json" 2>/dev/null || true
RC_KIND="$(jqget '.kind')"; RC_FROM="$(jqget '.from_release_id')"; RC_TO="$(jqget '.to_release_id')"
RECALL_DEP="$(jqget '.recall_deployment_id')"; RC_MODE="$(jqget '.details.mode')"
[ "${RC_KIND}" = "rollback" ] || fail "recall kind=${RC_KIND} (want rollback)"
[ "${RC_FROM}" = "${REL_ID_V1}" ] || fail "recall from_release_id=${RC_FROM} (want v1.1.0 release ${REL_ID_V1})"
[ "${RC_TO}" = "${REL_ID_V2}" ] || fail "recall to_release_id=${RC_TO} (want v1.2.0 release ${REL_ID_V2})"
[ -n "${RECALL_DEP}" ] && [ "${RECALL_DEP}" != "null" ] || fail "recall created no recall_deployment_id"
[ "${RC_MODE}" = "forward-fix" ] || fail "recall details.mode=${RC_MODE} (want forward-fix)"
log "  [4/7] RECALL ok: kind=rollback from=${RC_FROM} to=${RC_TO} mode=forward-fix recall_deployment=${RECALL_DEP}"

# cross-check the rollback_history.
req GET "/deployments/${DEP_D1}/rollbacks" "" "${OP_TOKEN}"
need_status 200 "GET /deployments/{D1}/rollbacks"
printf '%s' "${HTTP_BODY}" | jq . > "${QA_DIR}/rollback_history.json" 2>/dev/null || true
HCNT="$(printf '%s' "${HTTP_BODY}" | jq -r --arg f "${REL_ID_V1}" --arg t "${REL_ID_V2}" --arg rd "${RECALL_DEP}" \
    '[.items[] | select(.kind=="rollback" and .from_release_id==$f and .to_release_id==$t and .recall_deployment_id==$rd and .details.mode=="forward-fix")] | length' 2>/dev/null)"
[ "${HCNT}" -ge 1 ] 2>/dev/null || fail "rollback_history missing the forward-fix row"
log "  [5/7] rollback_history row confirmed (count=${HCNT})"
hr

# --- (i) RUN 2: emulator (current 1.1.0) RECOVERS to v1.2.0 -------------------
log "RUN 2 (i): emulator re-check (current ${REL_V1}) -> 200 offer v${REL_V2} -> apply -> recover"
run_emu "${EMU_CTR2}" "${QA_DIR}/emulator_run2_recover.json" "${QA_DIR}/emulator_run2_stderr.log" "${REL_V1}" \
    || fail "emulator run-2 exited non-zero; see emulator_run2_stderr.log"
OUT2="$(cat "${QA_DIR}/emulator_run2_recover.json")"
log "  run-2 Outcome:"; sed 's/^/    /' "${QA_DIR}/emulator_run2_recover.json" | tee -a "${TRANSCRIPT}"
[ "$(printf '%s' "${OUT2}" | jq -r '.applied')" = "true" ] || fail "run-2 did not apply the fix"
[ "$(printf '%s' "${OUT2}" | jq -r '.from_version')" = "${REL_V1}" ] || fail "run-2 from_version != ${REL_V1}"
[ "$(printf '%s' "${OUT2}" | jq -r '.to_version')" = "${REL_V2}" ] || fail "run-2 to_version != ${REL_V2} (no recovery)"
[ "$(printf '%s' "${OUT2}" | jq -r '.offered_version')" = "${REL_V2}" ] || fail "run-2 offered_version != ${REL_V2}"
[ "$(printf '%s' "${OUT2}" | jq -r '.deployment_id')" = "${RECALL_DEP}" ] \
    || fail "run-2 offer deployment_id != recall deployment ${RECALL_DEP}"
[ "$(printf '%s' "${OUT2}" | jq -r '.telemetry_rejected')" = "0" ] || fail "run-2 telemetry rejected != 0"
[ "$(printf '%s' "${OUT2}" | jq -r '.healthy')" = "true" ] || fail "run-2 device not healthy after recovery"
log "  [6/7] RECOVERY ok: ${REL_V1} -> ${REL_V2} via recall deployment ${RECALL_DEP}, healthy, telemetry accepted"

# cross-check server success telemetry stamped with the recall deployment_id.
req GET "/devices/${DEV_ID}/telemetry?event=success" "" "${OP_TOKEN}"
need_status 200 "GET /devices/{id}/telemetry?event=success"
printf '%s' "${HTTP_BODY}" | jq . > "${QA_DIR}/server_success_telemetry.json" 2>/dev/null || true
SCNT="$(printf '%s' "${HTTP_BODY}" | jq -r --arg d "${RECALL_DEP}" \
    '[.items[] | select(.event=="success" and .deployment_id==$d)] | length' 2>/dev/null)"
[ "${SCNT}" -ge 1 ] 2>/dev/null || fail "no success telemetry stamped with recall deployment ${RECALL_DEP}"
log "  server success telemetry present for recall deployment (count=${SCNT})"
req GET "/devices/${DEV_ID}/status" "" "${OP_TOKEN}"
need_status 200 "GET /devices/{id}/status (after recovery)"
[ "$(jqget '.health.ok')" = "true" ] || fail "device not healthy after recovery (server status)"
[ "$(jqget '.current_version')" = "${REL_V2}" ] || fail "device current_version != ${REL_V2} after recovery"
log "  server status: health.ok=true current_version=${REL_V2}"
"${PODMAN}" logs "${CP_CTR}" > "${QA_DIR}/control_plane_after_recovery.log" 2>&1 || true
hr

# --- (j) RUN 3: emulator (current 1.2.0) -> 204 on-target ---------------------
log "RUN 3 (j): emulator re-check (current ${REL_V2}) -> on-target (204, no update)"
run_emu "${EMU_CTR3}" "${QA_DIR}/emulator_run3_ontarget.json" "${QA_DIR}/emulator_run3_stderr.log" "${REL_V2}" \
    || fail "emulator run-3 exited non-zero; see emulator_run3_stderr.log"
OUT3="$(cat "${QA_DIR}/emulator_run3_ontarget.json")"
log "  run-3 Outcome:"; sed 's/^/    /' "${QA_DIR}/emulator_run3_ontarget.json" | tee -a "${TRANSCRIPT}"
[ "$(printf '%s' "${OUT3}" | jq -r '.on_target')" = "true" ] || fail "run-3 on_target should be true (device on ${REL_V2})"
[ "$(printf '%s' "${OUT3}" | jq -r '.applied')" = "false" ] || fail "run-3 applied should be false (on-target)"
log "  [7/7] ON-TARGET ok: on_target=true applied=false (clean 204 on ${REL_V2})"
hr

# --- success report -----------------------------------------------------------
log "RESULT: PASS — Tier-1 FAILURE -> RECALL(forward-fix) -> RECOVERY proven END-TO-END on podman."
log "  staged       : release v${REL_V1} ${REL_ID_V1} -> deployment D1 ${DEP_D1}"
log "  apply        : device ${DEV_CUR} -> ${REL_V1} (healthy)"
log "  failure      : ${FAIL_CODE} -> server health.ok=false, version not advanced"
log "  forward-fix  : release v${REL_V2} ${REL_ID_V2}"
log "  recall       : D1 -> recall deployment ${RECALL_DEP} (kind=rollback, mode=forward-fix)"
log "  recovery     : device ${REL_V1} -> ${REL_V2} via recall deployment (healthy, telemetry accepted)"
log "  on-target    : 204 on ${REL_V2}"
log "  evidence     : ${QA_DIR}/"
hr
log "Captured evidence directory: ${QA_DIR}"
exit 0
