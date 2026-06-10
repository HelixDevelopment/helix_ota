#!/usr/bin/env bash
#
# tier1_full_lifecycle_e2e.sh — Tier-1 emulated-device FULL OTA-LIFECYCLE E2E on podman.
#
# Purpose
#   Close the "honest gap" the prior Tier-1 e2e (tier1_container_e2e.sh) noted: it
#   only exercised the clean on-target 204 path because NO deployment was staged.
#   This harness drives the COMPLETE OTA round-trip against a CONTAINERIZED Helix
#   OTA control plane with NO live hardware:
#     1. boot the control plane (ota-server) in a podman pod, configured with a
#        real EPHEMERAL ed25519 trust key (HELIX_ARTIFACT_PUBKEY) so signed
#        artifact uploads verify;
#     2. via the admin REST API, STAGE A REAL DEPLOYMENT at a version HIGHER than
#        the device's current: mint key -> sign + upload artifact (201, verified)
#        -> create release -> create all-targets deployment for the device's
#        model/os;
#     3. run the ota-device-emu container (OTA_CURRENT_VERSION below the release);
#     4. ASSERT the device gets a 200 update offer carrying deployment_id, reports
#        the FULL telemetry lifecycle ACCEPTED (rejected=0), advances its version,
#        and that a SECOND run returns 204 on-target;
#     5. cross-check the SERVER-side telemetry history (GET /devices/{id}/telemetry)
#        shows the download_started -> installing -> installed -> verifying ->
#        success lifecycle stamped with the deployment_id.
#
#   §11.4 anti-bluff: every PASS is backed by real captured podman logs + the
#   emulator Outcome JSON + the server telemetry history under docs/qa/<run-id>/.
#   §11.4.76 uses the containers-submodule emulator image. §11.4.107: the full
#   lifecycle (not a single 204) is the proof the feature really works.
#
# Usage
#   bash tests/emulator/tier1_full_lifecycle_e2e.sh
#   KEEP_UP=1 bash tests/emulator/tier1_full_lifecycle_e2e.sh   # leave stack up
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
#   - Creates a podman pod "helix-tier1-full" with the control plane + two
#     emulator runs; removes it on exit (unless KEEP_UP). trap cleanup §11.4.14.
#   - Generates an EPHEMERAL ed25519 keypair in a mktemp dir (never committed,
#     rm -rf'd on exit, §11.4.10).
#   - Writes captured podman logs + emulator Outcome JSON + server telemetry
#     history under docs/qa/<run-id>/ (§11.4.83).
#
# Dependencies: podman (a running podman machine on macOS), Go toolchain, and the
#   host tools openssl(>=3 ed25519)/xxd/base64/curl/jq/python3 (the signing
#   contract mirrors tests/e2e/pipeline_signed.sh: ed25519 over the hex-decoded
#   SHA-256 of a ZIP_STORED payload; pubkey = last 32 DER bytes, base64).
# Cross-references: server/Dockerfile, containers/images/ota-device-emu/Dockerfile,
#   server/cmd/ota-device-emu/main.go, server/internal/deviceemu/emulator.go,
#   server/internal/api/handlers_artifact.go (resolvePublicKey),
#   server/internal/api/handlers_telemetry.go (GET /devices/{id}/telemetry),
#   tests/e2e/pipeline_signed.sh, tests/emulator/tier1_container_e2e.sh,
#   docs/scripts/tier1_full_lifecycle_e2e.md.

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
ADMIN_PASS="${ADMIN_PASS:-tier1-full-$(date +%s)-secret}"
HELIX_PORT="${HELIX_PORT:-8080}"
API_BASE="/api/v1"

POD_NAME="helix-tier1-full"
CP_CTR="helix-cp"
EMU_CTR1="helix-emu-1"
EMU_CTR2="helix-emu-2"
EMU_RUN_IMAGE="ota-device-emu-tier1:dev"

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
QA_DIR="${REPO_ROOT}/docs/qa/${RUN_ID}-full-lifecycle"
BIN_DIR="${REPO_ROOT}/bin"
CP_STAGE="${SERVER_DIR}/.docker-bin"
EMU_STAGE="${REPO_ROOT}/tests/emulator/.docker-bin"
mkdir -p "${QA_DIR}" "${BIN_DIR}" "${CP_STAGE}" "${EMU_STAGE}"

TRANSCRIPT="${QA_DIR}/tier1_full_lifecycle_transcript.txt"

# Ephemeral key + scratch dir (NEVER committed, rm -rf'd on exit) — §11.4.10.
WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-full.XXXXXX")"

# Device identity the emulator impersonates. The release is staged ABOVE current.
HW_ID="rk3588-full-$(date +%s)"
DEV_MODEL="OrangePi5Max"
DEV_OS="android"
DEV_CUR="1.0.0"
REL_VERSION="1.1.0"   # strictly greater than DEV_CUR -> an update IS offered

# Host reaches the pod via the published port; the emulator container shares the
# pod net ns and reaches the control plane on 127.0.0.1.
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

# --- host HTTP helper (operator REST calls from the host) ---------------------
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

# ==============================================================================
log "Tier-1 FULL-LIFECYCLE container E2E — run-id ${RUN_ID}"
log "repo root: ${REPO_ROOT}"
log "evidence : ${QA_DIR}"
hr

# --- preflight: podman + host signing tools -----------------------------------
log "PREFLIGHT: podman reachable + arch"
"${PODMAN}" info --format '{{.Host.Arch}} {{.Host.OS}}' 2>&1 | tee -a "${TRANSCRIPT}" \
    || fail "podman not reachable (is the podman machine running?)"
for b in go openssl xxd base64 curl jq python3; do
    command -v "${b}" >/dev/null 2>&1 || fail "required host tool '${b}' not found"
done
log "openssl: $(openssl version)"
hr

# --- (0) ephemeral ed25519 keypair (the control-plane trust key) --------------
log "KEYS (0): mint ephemeral ed25519 artifact-trust keypair (never committed)"
PRIV_PEM="${WORK}/artifact_priv.pem"
openssl genpkey -algorithm ed25519 -out "${PRIV_PEM}" 2>/dev/null \
    || fail "openssl could not generate an ed25519 key (scheme unsupported on this host)"
PUBKEY_B64="$(openssl pkey -in "${PRIV_PEM}" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 | tr -d '\n')"
PUBLEN="$(openssl pkey -in "${PRIV_PEM}" -pubout -outform DER 2>/dev/null | tail -c 32 | wc -c | tr -d ' ')"
[ "${PUBLEN}" = "32" ] || fail "extracted ed25519 public key is ${PUBLEN} bytes, expected 32"
log "  raw pubkey 32 bytes, base64 configured for HELIX_ARTIFACT_PUBKEY"
hr

# --- (a) cross-compile both static linux/arm64 binaries on host ---------------
log "BUILD (a): cross-compile static linux/arm64 binaries on host"
( cd "${SERVER_DIR}" \
  && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" \
       -o "${BIN_DIR}/ota-server" ./cmd/ota-server ) 2>&1 | tee -a "${TRANSCRIPT}"
[ -x "${BIN_DIR}/ota-server" ] || fail "ota-server binary not produced"
( cd "${SERVER_DIR}" \
  && CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" \
       -o "${BIN_DIR}/ota-device-emu" ./cmd/ota-device-emu ) 2>&1 | tee -a "${TRANSCRIPT}"
[ -x "${BIN_DIR}/ota-device-emu" ] || fail "ota-device-emu binary not produced"
log "built: $(ls -la "${BIN_DIR}/ota-server" "${BIN_DIR}/ota-device-emu" | awk '{print $5, $9}')"
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
# Generated by tests/emulator/tier1_full_lifecycle_e2e.sh — derives the consumer
# emulator image from the containers-submodule runtime base + our binary.
FROM ${EMU_IMAGE}
COPY ota-device-emu /usr/local/bin/ota-device-emu
EOF
"${PODMAN}" build -f "${EMU_STAGE}/Dockerfile" -t "${EMU_RUN_IMAGE}" "${EMU_STAGE}" \
    2>&1 | tee -a "${TRANSCRIPT}" | tail -5
"${PODMAN}" image exists "${EMU_RUN_IMAGE}" || fail "derived emulator image not built"
hr

# --- (c) bring the control plane up with the trust key ------------------------
log "BOOT (c): create pod '${POD_NAME}' (publish ${HELIX_PORT})"
"${PODMAN}" pod rm -f "${POD_NAME}" >/dev/null 2>&1 || true
# Stale-listener guard: a prior run's pod (or any other process) still serving on
# ${HELIX_PORT} would make our published port silently bind elsewhere and a later
# request hit the WRONG server (observed: /healthz 200 but /api/v1/... 404). Fail
# fast with a clear message instead of producing a misleading FAIL downstream.
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
    -e "HELIX_TOKEN_SECRET=tier1-full-token-secret" \
    -e "HELIX_ARTIFACT_PUBKEY=${PUBKEY_B64}" \
    --memory 256m \
    "${CP_IMAGE}" 2>&1 | tee -a "${TRANSCRIPT}"
hr

# --- (d) wait for the control plane to be healthy AND serving the API base ----
# Gate on BOTH /healthz (200) AND the API base path answering (a 4xx/200 to
# POST /auth/login — NOT a 404 from a wrong/stale server). This proves OUR CP (at
# ${API_BASE}) is the one answering before we stage anything.
log "HEALTH (d): wait for control-plane /healthz + ${API_BASE}/auth/login to answer"
healthy=0
for i in $(seq 1 60); do
    code="$(curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${HELIX_PORT}/healthz" 2>/dev/null || echo 000)"
    if [ "${code}" = "200" ]; then
        # /auth/login must NOT 404 (a 401/400/200 means the API base path is live).
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
log "control-plane boot log captured -> ${QA_DIR}/control_plane_boot.log"
hr

# --- (e) STAGE A REAL DEPLOYMENT via the admin REST API (from the host) -------
# Signing contract (mirrors tests/e2e/pipeline_signed.sh):
#   sha256_hex = lowercase-hex SHA-256 over the WHOLE ZIP file bytes
#   signature  = base64( ed25519.Sign( priv, hex_decode(sha256_hex) ) )
log "STAGE (e): operator stages a real deployment at v${REL_VERSION} (> device v${DEV_CUR})"

# e.0 login
req POST "/auth/login" \
    "$(jq -nc --arg u "${ADMIN_USER}" --arg p "${ADMIN_PASS}" '{username:$u,password:$p}')" ""
need_status 200 "POST /auth/login"
OP_TOKEN="$(jqget '.access_token')"
[ -n "${OP_TOKEN}" ] && [ "${OP_TOKEN}" != "null" ] || fail "login 200 but no access_token"
log "  obtained operator access token"

# e.1 build + sign + upload a real artifact
ZIP="${WORK}/ota.zip"
PAYLOAD="HELIX OTA payload v${REL_VERSION} ${RUN_ID}" OUT="${ZIP}" python3 - <<'PY'
import os, zipfile
out = os.environ["OUT"]; payload = os.environ["PAYLOAD"].encode()
with zipfile.ZipFile(out, "w", compression=zipfile.ZIP_STORED) as z:
    zi = zipfile.ZipInfo("payload.bin"); zi.compress_type = zipfile.ZIP_STORED
    z.writestr(zi, payload)
PY
[ -s "${ZIP}" ] || fail "could not build ZIP_STORED artifact"
DIGEST_HEX="$(openssl dgst -sha256 -binary "${ZIP}" | xxd -p -c256 | tr -d '\n')"
RAWF="${WORK}/digest.bin"; SIGF="${WORK}/sig.bin"
printf '%s' "${DIGEST_HEX}" | xxd -r -p > "${RAWF}"
openssl pkeyutl -sign -inkey "${PRIV_PEM}" -rawin -in "${RAWF}" -out "${SIGF}" 2>/dev/null \
    || fail "openssl ed25519 sign over digest failed (cannot stage a signed artifact)"
SIG_B64="$(base64 < "${SIGF}" | tr -d '\n')"
FILE_SIZE="$(wc -c < "${ZIP}" | tr -d ' ')"
META="$(jq -nc \
    --arg sha "${DIGEST_HEX}" --arg sig "${SIG_B64}" --arg ver "${REL_VERSION}" \
    --arg os "${DEV_OS}" --arg tm "${DEV_MODEL}" \
    --arg fh "$(printf 'file-hash' | base64 | tr -d '\n')" --argjson fs "${FILE_SIZE}" \
    --arg mh "$(printf 'meta-hash' | base64 | tr -d '\n')" --argjson ms 64 \
    '{sha256:$sha,signature:$sig,version:$ver,os:$os,target_model:$tm,
      file_hash:$fh,file_size:$fs,metadata_hash:$mh,metadata_size:$ms}')"
UPTMP="$(mktemp "${WORK}/up.XXXXXX")"
UP_STATUS="$(curl -sS -o "${UPTMP}" -w '%{http_code}' -X POST "${HOST_BASE}/artifacts/upload" \
    -H "Authorization: Bearer ${OP_TOKEN}" \
    -F "file=@${ZIP};type=application/zip;filename=ota.zip" \
    -F "metadata=${META};type=application/json" 2>/dev/null)" || UP_STATUS="000"
UP_BODY="$(cat "${UPTMP}")"; rm -f "${UPTMP}"
[ "${UP_STATUS}" = "201" ] || fail "artifact upload: want 201, got ${UP_STATUS} (body: $(printf '%s' "${UP_BODY}" | head -c 280))"
ART_ID="$(printf '%s' "${UP_BODY}" | jq -r '.artifact_id')"
VERIFIED="$(printf '%s' "${UP_BODY}" | jq -r '.verified')"
[ -n "${ART_ID}" ] && [ "${ART_ID}" != "null" ] || fail "upload 201 but no artifact_id"
[ "${VERIFIED}" = "true" ] || fail "artifact .verified != true (signature did not verify server-side)"
log "  artifact upload -> HTTP 201 verified=true artifact_id=${ART_ID}"

# e.2 create the release
req POST "/releases" \
    "$(jq -nc --arg a "${ART_ID}" --arg v "${REL_VERSION}" --arg os "${DEV_OS}" --arg tm "${DEV_MODEL}" \
        '{artifact_id:$a,version:$v,os:$os,target_model:$tm,notes:"full-lifecycle e2e"}')"
need_status 201 "POST /releases (v${REL_VERSION})"
REL_ID="$(jqget '.release_id')"
[ -n "${REL_ID}" ] && [ "${REL_ID}" != "null" ] || fail "release 201 but no release_id"
log "  release_id=${REL_ID}"

# e.3 create an all-targets deployment for the device's model/os
req POST "/deployments" \
    "$(jq -nc --arg r "${REL_ID}" '{release_id:$r,strategy:"all-targets"}')"
need_status 201 "POST /deployments (all-targets)"
DEP_ID="$(jqget '.deployment_id')"
[ -n "${DEP_ID}" ] && [ "${DEP_ID}" != "null" ] || fail "deployment 201 but no deployment_id"
log "  deployment_id=${DEP_ID} target_count=$(jqget '.target_count')"
hr

# --- (f) RUN 1: emulator drives the FULL lifecycle ----------------------------
# The device starts on ${DEV_CUR} (< ${REL_VERSION}), so the control plane MUST
# offer a 200 update carrying the deployment_id. The emulator self-serves that id
# (otaprotocol.UpdateAvailable.DeploymentID), applies, and reports the lifecycle.
log "RUN 1 (f): emulator register -> 200 offer -> apply -> full telemetry lifecycle"
set +e
"${PODMAN}" run --pod "${POD_NAME}" --name "${EMU_CTR1}" \
    -e "OTA_BASE_URL=http://127.0.0.1:${HELIX_PORT}${API_BASE}" \
    --memory 256m \
    "${EMU_RUN_IMAGE}" \
    -base "http://127.0.0.1:${HELIX_PORT}${API_BASE}" \
    -admin-user "${ADMIN_USER}" \
    -admin-pass "${ADMIN_PASS}" \
    -hardware-id "${HW_ID}" \
    -model "${DEV_MODEL}" \
    -os "${DEV_OS}" \
    -current-version "${DEV_CUR}" \
    -once \
    > "${QA_DIR}/emulator_run1_outcome.json" 2> "${QA_DIR}/emulator_run1_stderr.log"
EMU1_RC=$?
set -e
log "emulator run-1 exit rc=${EMU1_RC}"
log "emulator run-1 Outcome JSON:"
sed 's/^/    /' "${QA_DIR}/emulator_run1_outcome.json" | tee -a "${TRANSCRIPT}"
if [ -s "${QA_DIR}/emulator_run1_stderr.log" ]; then
    log "emulator run-1 stderr:"; sed 's/^/    /' "${QA_DIR}/emulator_run1_stderr.log" | tee -a "${TRANSCRIPT}"
fi
hr

# --- (g) ASSERT the full lifecycle from the run-1 Outcome ---------------------
log "ASSERT (g): full OTA lifecycle from the emulator Outcome"
OUT1="$(cat "${QA_DIR}/emulator_run1_outcome.json")"
[ "${EMU1_RC}" -eq 0 ] || fail "emulator run-1 exited non-zero (rc=${EMU1_RC}); see emulator_run1_stderr.log"

DEV_ID="$(printf '%s' "${OUT1}" | jq -r '.device_id')"
[ -n "${DEV_ID}" ] && [ "${DEV_ID}" != "null" ] || fail "register: no device_id in Outcome"
log "  [1/6] REGISTER ok: device_id=${DEV_ID}"

# 200 update offer (the device was BEHIND -> NOT on target on the first check).
[ "$(printf '%s' "${OUT1}" | jq -r '.on_target')" = "false" ] \
    || fail "update-check: on_target should be false on first check (update was due)"
OFFERED="$(printf '%s' "${OUT1}" | jq -r '.offered_version')"
[ "${OFFERED}" = "${REL_VERSION}" ] \
    || fail "update-check: offered_version=${OFFERED}, want ${REL_VERSION} (no 200 offer)"
log "  [2/6] 200 UPDATE OFFER ok: offered_version=${OFFERED} (device was behind)"

# the offer carried a deployment_id, self-served by the device.
OUT_DEP="$(printf '%s' "${OUT1}" | jq -r '.deployment_id')"
[ -n "${OUT_DEP}" ] && [ "${OUT_DEP}" != "null" ] && [ "${OUT_DEP}" != "" ] \
    || fail "offer carried no deployment_id (the device could not stamp telemetry)"
[ "${OUT_DEP}" = "${DEP_ID}" ] \
    || fail "offer deployment_id=${OUT_DEP} != staged ${DEP_ID}"
log "  [3/6] DEPLOYMENT_ID ok: offer carried ${OUT_DEP} (== staged deployment)"

# applied + version advanced.
[ "$(printf '%s' "${OUT1}" | jq -r '.applied')" = "true" ] || fail "device did not apply the update"
[ "$(printf '%s' "${OUT1}" | jq -r '.from_version')" = "${DEV_CUR}" ] || fail "from_version != ${DEV_CUR}"
[ "$(printf '%s' "${OUT1}" | jq -r '.to_version')" = "${REL_VERSION}" ] \
    || fail "to_version != ${REL_VERSION} (version did not advance)"
log "  [4/6] APPLY+ADVANCE ok: ${DEV_CUR} -> ${REL_VERSION} (applied=true)"

# telemetry lifecycle ACCEPTED (rejected=0).
ACC="$(printf '%s' "${OUT1}" | jq -r '.telemetry_accepted')"
REJ="$(printf '%s' "${OUT1}" | jq -r '.telemetry_rejected')"
[ "${REJ}" = "0" ] || fail "telemetry rejected=${REJ} (want 0); the lifecycle batch was not accepted"
[ "${ACC}" -ge 1 ] 2>/dev/null || fail "telemetry accepted=${ACC} (want >=1)"
[ "$(printf '%s' "${OUT1}" | jq -r '.healthy')" = "true" ] || fail "device not healthy after success"
log "  [5/6] TELEMETRY ACCEPTED ok: accepted=${ACC} rejected=0 healthy=true"

# --- (h) cross-check the SERVER-side telemetry history ------------------------
log "  [6/6] cross-check server telemetry history: GET /devices/${DEV_ID}/telemetry"
req GET "/devices/${DEV_ID}/telemetry" "" "${OP_TOKEN}"
need_status 200 "GET /devices/{id}/telemetry"
printf '%s' "${HTTP_BODY}" | jq . > "${QA_DIR}/server_telemetry_history.json" 2>/dev/null \
    || printf '%s' "${HTTP_BODY}" > "${QA_DIR}/server_telemetry_history.json"
log "  server telemetry history captured -> ${QA_DIR}/server_telemetry_history.json"
# The full success lifecycle: download_started -> installing -> installed ->
# verifying -> success, each stamped with the deployment_id.
for ev in download_started installing installed verifying success; do
    cnt="$(printf '%s' "${HTTP_BODY}" | jq -r --arg e "${ev}" '[.items[] | select(.event==$e)] | length' 2>/dev/null)"
    [ "${cnt}" -ge 1 ] 2>/dev/null \
        || fail "server telemetry history missing the '${ev}' event (got count=${cnt})"
    log "    lifecycle event '${ev}' present in server history (count=${cnt})"
done
# the success event must carry the staged deployment_id.
SUC_DEP="$(printf '%s' "${HTTP_BODY}" | jq -r '[.items[] | select(.event=="success")][0].deployment_id' 2>/dev/null)"
[ "${SUC_DEP}" = "${DEP_ID}" ] \
    || fail "server 'success' event deployment_id=${SUC_DEP} != staged ${DEP_ID}"
log "    server 'success' event carries deployment_id=${SUC_DEP} (== staged)"
"${PODMAN}" logs "${CP_CTR}" > "${QA_DIR}/control_plane_after_run1.log" 2>&1 || true
hr

# --- (i) RUN 2: same device -> 204 on-target ----------------------------------
log "RUN 2 (i): same device re-checks -> must be ON-TARGET (204, no update)"
set +e
"${PODMAN}" run --pod "${POD_NAME}" --name "${EMU_CTR2}" \
    -e "OTA_BASE_URL=http://127.0.0.1:${HELIX_PORT}${API_BASE}" \
    --memory 256m \
    "${EMU_RUN_IMAGE}" \
    -base "http://127.0.0.1:${HELIX_PORT}${API_BASE}" \
    -admin-user "${ADMIN_USER}" \
    -admin-pass "${ADMIN_PASS}" \
    -hardware-id "${HW_ID}" \
    -model "${DEV_MODEL}" \
    -os "${DEV_OS}" \
    -current-version "${REL_VERSION}" \
    -once \
    > "${QA_DIR}/emulator_run2_outcome.json" 2> "${QA_DIR}/emulator_run2_stderr.log"
EMU2_RC=$?
set -e
log "emulator run-2 exit rc=${EMU2_RC}"
log "emulator run-2 Outcome JSON:"
sed 's/^/    /' "${QA_DIR}/emulator_run2_outcome.json" | tee -a "${TRANSCRIPT}"
OUT2="$(cat "${QA_DIR}/emulator_run2_outcome.json")"
[ "${EMU2_RC}" -eq 0 ] || fail "emulator run-2 exited non-zero (rc=${EMU2_RC})"
[ "$(printf '%s' "${OUT2}" | jq -r '.on_target')" = "true" ] \
    || fail "run-2: on_target should be true (device is on ${REL_VERSION})"
[ "$(printf '%s' "${OUT2}" | jq -r '.applied')" = "false" ] \
    || fail "run-2: applied should be false (nothing to apply on-target)"
printf '%s' "${OUT2}" | grep -q '204' \
    || log "  note: run-2 on_target=true (204 path); Outcome note: $(printf '%s' "${OUT2}" | jq -r '.note')"
log "  RUN 2 ok: on_target=true, applied=false (clean 204 on-target re-check)"
hr

# --- success report -----------------------------------------------------------
log "RESULT: PASS — Tier-1 emulated-device FULL OTA LIFECYCLE proven END-TO-END on podman."
log "  staged        : artifact(201,verified) -> release ${REL_ID} -> deployment ${DEP_ID}"
log "  register      : device_id=${DEV_ID}"
log "  200 offer     : offered_version=${REL_VERSION} carrying deployment_id=${DEP_ID}"
log "  apply         : ${DEV_CUR} -> ${REL_VERSION} (applied=true, version advanced)"
log "  telemetry     : accepted=${ACC} rejected=0 (download_started/installing/installed/verifying/success)"
log "  server xcheck : GET /devices/${DEV_ID}/telemetry shows the lifecycle w/ deployment_id"
log "  re-check      : 204 on-target (device now on ${REL_VERSION})"
log "  evidence      : ${QA_DIR}/"
hr
log "Captured evidence directory: ${QA_DIR}"
exit 0
