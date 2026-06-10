#!/usr/bin/env bash
#
# tier1_fleet_e2e.sh — Tier-1 MULTI-DEVICE FLEET OTA E2E on podman.
#
# Purpose
#   Prove multi-device emulation ("create emulators", plural): boot N distinct
#   ota-device-emu containers CONCURRENTLY as a FLEET against ONE containerized
#   Helix OTA control plane, with NO live hardware. The operator stages ONE real
#   all-targets deployment at a version ABOVE the fleet's current; then every
#   emulator (a distinct hardware_id / distinct device) independently registers,
#   gets a 200 update offer carrying the deployment_id, applies the update,
#   reports the FULL telemetry lifecycle ACCEPTED (rejected=0), and advances its
#   version. The harness asserts ALL N devices completed, captures per-device
#   evidence + a fleet summary, and cross-checks the server fleet telemetry.
#
#   §11.4 anti-bluff: every PASS is backed by real captured per-device Outcome
#   JSON + the server fleet-telemetry overview under docs/qa/<run-id>/. §11.4.76
#   uses the containers-submodule emulator image. §11.4.119: each emulator is its
#   OWN distinct device — no shared exclusive resource is co-driven; the fleet is
#   genuinely concurrent (containers run in parallel).
#
# Usage
#   bash tests/emulator/tier1_fleet_e2e.sh
#   FLEET_SIZE=8 bash tests/emulator/tier1_fleet_e2e.sh   # N devices (default 5)
#   KEEP_UP=1   bash tests/emulator/tier1_fleet_e2e.sh    # leave stack up
#
# Inputs (environment, all optional — sane defaults)
#   PODMAN          podman binary           (default: podman)
#   CP_IMAGE        control-plane image tag (default: ota-control-plane:dev)
#   EMU_IMAGE       base emulator image tag (default: ota-device-emu:dev)
#   ADMIN_USER      seeded admin login      (default: admin@helix.example)
#   ADMIN_PASS      seeded admin password   (default: a generated test secret)
#   HELIX_PORT      control-plane port      (default: 8080)
#   FLEET_SIZE      number of emulators     (default: 5)
#   KEEP_UP         non-empty => skip teardown (debugging)
#
# Outputs / Side-effects
#   - Cross-compiles bin/ota-server + bin/ota-device-emu (static linux/arm64).
#   - Builds the control-plane + derived ota-device-emu images via podman.
#   - Creates a podman pod "helix-tier1-fleet" with the control plane + N
#     emulator containers; removes it on exit (unless KEEP_UP). trap §11.4.14.
#   - Ephemeral ed25519 keypair in a mktemp dir (never committed, §11.4.10).
#   - Writes per-device Outcome JSON + logs + a fleet summary + the server
#     telemetry overview under docs/qa/<run-id>/ (§11.4.83).
#
# Dependencies: podman (running machine), Go toolchain, openssl(>=3 ed25519)/
#   xxd/base64/curl/jq/python3 on the host (signing per pipeline_signed.sh).
# Cross-references: tests/emulator/tier1_full_lifecycle_e2e.sh,
#   tests/emulator/tier1_container_e2e.sh, tests/e2e/pipeline_signed.sh,
#   server/internal/api/handlers_telemetry.go (GET /telemetry/overview),
#   docs/scripts/tier1_fleet_e2e.md.

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
ADMIN_PASS="${ADMIN_PASS:-tier1-fleet-$(date +%s)-secret}"
HELIX_PORT="${HELIX_PORT:-8080}"
FLEET_SIZE="${FLEET_SIZE:-5}"
API_BASE="/api/v1"

POD_NAME="helix-tier1-fleet"
CP_CTR="helix-cp"
EMU_RUN_IMAGE="ota-device-emu-tier1:dev"

RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
QA_DIR="${REPO_ROOT}/docs/qa/${RUN_ID}-fleet"
FLEET_DIR="${QA_DIR}/fleet"
BIN_DIR="${REPO_ROOT}/bin"
CP_STAGE="${SERVER_DIR}/.docker-bin"
EMU_STAGE="${REPO_ROOT}/tests/emulator/.docker-bin"
mkdir -p "${QA_DIR}" "${FLEET_DIR}" "${BIN_DIR}" "${CP_STAGE}" "${EMU_STAGE}"

TRANSCRIPT="${QA_DIR}/tier1_fleet_transcript.txt"
FLEET_SUMMARY="${QA_DIR}/fleet_summary.txt"

WORK="$(mktemp -d "${TMPDIR:-/tmp}/helix-fleet.XXXXXX")"

DEV_MODEL="OrangePi5Max"
DEV_OS="android"
DEV_CUR="1.0.0"
REL_VERSION="1.2.0"   # > DEV_CUR -> every fleet device is offered an update
RUN_STAMP="$(date +%s)"

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
    log "TEARDOWN: removing pod '${POD_NAME}' (and its ${FLEET_SIZE}+1 containers)."
    "${PODMAN}" pod rm -f "${POD_NAME}" >/dev/null 2>&1 || true
    rm -f "${CP_STAGE}/ota-server" "${EMU_STAGE}/ota-device-emu" "${EMU_STAGE}/Dockerfile" 2>/dev/null || true
    log "TEARDOWN done (exit rc=${rc})."
    return 0
}
trap cleanup EXIT INT TERM

# --- host HTTP helper ---------------------------------------------------------
HTTP_STATUS=""; HTTP_BODY=""
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
log "Tier-1 FLEET container E2E — run-id ${RUN_ID}, fleet size ${FLEET_SIZE}"
log "repo root: ${REPO_ROOT}"
log "evidence : ${QA_DIR}"
hr

# --- preflight ----------------------------------------------------------------
log "PREFLIGHT: podman reachable + arch + host signing tools"
"${PODMAN}" info --format '{{.Host.Arch}} {{.Host.OS}}' 2>&1 | tee -a "${TRANSCRIPT}" \
    || fail "podman not reachable (is the podman machine running?)"
for b in go openssl xxd base64 curl jq python3; do
    command -v "${b}" >/dev/null 2>&1 || fail "required host tool '${b}' not found"
done
[ "${FLEET_SIZE}" -ge 2 ] 2>/dev/null || fail "FLEET_SIZE must be >= 2 to prove a fleet (got ${FLEET_SIZE})"
log "openssl: $(openssl version)"
hr

# --- (0) ephemeral ed25519 keypair --------------------------------------------
log "KEYS (0): mint ephemeral ed25519 artifact-trust keypair (never committed)"
PRIV_PEM="${WORK}/artifact_priv.pem"
openssl genpkey -algorithm ed25519 -out "${PRIV_PEM}" 2>/dev/null \
    || fail "openssl could not generate an ed25519 key"
PUBKEY_B64="$(openssl pkey -in "${PRIV_PEM}" -pubout -outform DER 2>/dev/null | tail -c 32 | base64 | tr -d '\n')"
PUBLEN="$(openssl pkey -in "${PRIV_PEM}" -pubout -outform DER 2>/dev/null | tail -c 32 | wc -c | tr -d ' ')"
[ "${PUBLEN}" = "32" ] || fail "extracted ed25519 public key is ${PUBLEN} bytes, expected 32"
hr

# --- (a) cross-compile binaries -----------------------------------------------
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
log "built: $(ls -la "${BIN_DIR}/ota-server" "${BIN_DIR}/ota-device-emu" | awk '{print $5, $9}')"
hr

# --- (b) build images ---------------------------------------------------------
log "BUILD (b): podman build control-plane + emulator images"
"${PODMAN}" build -f "${SERVER_DIR}/Dockerfile" -t "${CP_IMAGE}" "${SERVER_DIR}" \
    2>&1 | tee -a "${TRANSCRIPT}" | tail -3
"${PODMAN}" image exists "${CP_IMAGE}" || fail "control-plane image not built"
"${PODMAN}" build -f "${EMU_IMAGE_DIR}/Dockerfile" -t "${EMU_IMAGE}" "${EMU_IMAGE_DIR}" \
    2>&1 | tee -a "${TRANSCRIPT}" | tail -3
"${PODMAN}" image exists "${EMU_IMAGE}" || fail "base ota-device-emu image not built"
cat > "${EMU_STAGE}/Dockerfile" <<EOF
# Generated by tests/emulator/tier1_fleet_e2e.sh.
FROM ${EMU_IMAGE}
COPY ota-device-emu /usr/local/bin/ota-device-emu
EOF
"${PODMAN}" build -f "${EMU_STAGE}/Dockerfile" -t "${EMU_RUN_IMAGE}" "${EMU_STAGE}" \
    2>&1 | tee -a "${TRANSCRIPT}" | tail -3
"${PODMAN}" image exists "${EMU_RUN_IMAGE}" || fail "derived emulator image not built"
hr

# --- (c) boot the control plane -----------------------------------------------
log "BOOT (c): create pod '${POD_NAME}', start control plane with trust key"
"${PODMAN}" pod rm -f "${POD_NAME}" >/dev/null 2>&1 || true
# Stale-listener guard (see tier1_full_lifecycle_e2e.sh rationale).
if curl -sS -o /dev/null -w '%{http_code}' "http://127.0.0.1:${HELIX_PORT}/healthz" 2>/dev/null | grep -q '200'; then
    fail "port ${HELIX_PORT} is already serving /healthz — a stale control plane is running. \
Free it or set HELIX_PORT to a free port, then re-run."
fi
"${PODMAN}" pod create --name "${POD_NAME}" -p "${HELIX_PORT}:${HELIX_PORT}" \
    2>&1 | tee -a "${TRANSCRIPT}"
"${PODMAN}" run -d --pod "${POD_NAME}" --name "${CP_CTR}" \
    -e "HELIX_PORT=${HELIX_PORT}" \
    -e "HELIX_API_BASE_PATH=${API_BASE}" \
    -e "HELIX_ADMIN_USERNAME=${ADMIN_USER}" \
    -e "HELIX_ADMIN_PASSWORD=${ADMIN_PASS}" \
    -e "HELIX_TOKEN_SECRET=tier1-fleet-token-secret" \
    -e "HELIX_ARTIFACT_PUBKEY=${PUBKEY_B64}" \
    --memory 256m \
    "${CP_IMAGE}" 2>&1 | tee -a "${TRANSCRIPT}"

log "HEALTH (c): wait for /healthz + ${API_BASE}/auth/login to answer"
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
[ "${healthy}" -eq 1 ] || { "${PODMAN}" logs "${CP_CTR}" 2>&1 | tail -30 | tee -a "${TRANSCRIPT}"; fail "control-plane not healthy on its API base"; }
"${PODMAN}" logs "${CP_CTR}" > "${QA_DIR}/control_plane_boot.log" 2>&1 || true
hr

# --- (d) stage ONE real deployment for the whole fleet ------------------------
log "STAGE (d): operator stages ONE all-targets deployment at v${REL_VERSION}"
req POST "/auth/login" \
    "$(jq -nc --arg u "${ADMIN_USER}" --arg p "${ADMIN_PASS}" '{username:$u,password:$p}')" ""
need_status 200 "POST /auth/login"
OP_TOKEN="$(jqget '.access_token')"
[ -n "${OP_TOKEN}" ] && [ "${OP_TOKEN}" != "null" ] || fail "login 200 but no access_token"

ZIP="${WORK}/ota.zip"
PAYLOAD="HELIX OTA fleet payload v${REL_VERSION} ${RUN_ID}" OUT="${ZIP}" python3 - <<'PY'
import os, zipfile
out = os.environ["OUT"]; payload = os.environ["PAYLOAD"].encode()
with zipfile.ZipFile(out, "w", compression=zipfile.ZIP_STORED) as z:
    zi = zipfile.ZipInfo("payload.bin"); zi.compress_type = zipfile.ZIP_STORED
    z.writestr(zi, payload)
PY
[ -s "${ZIP}" ] || fail "could not build ZIP_STORED artifact"
DIGEST_HEX="$(openssl dgst -sha256 -binary "${ZIP}" | xxd -p -c256 | tr -d '\n')"
printf '%s' "${DIGEST_HEX}" | xxd -r -p > "${WORK}/digest.bin"
openssl pkeyutl -sign -inkey "${PRIV_PEM}" -rawin -in "${WORK}/digest.bin" -out "${WORK}/sig.bin" 2>/dev/null \
    || fail "openssl ed25519 sign over digest failed"
SIG_B64="$(base64 < "${WORK}/sig.bin" | tr -d '\n')"
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
[ "$(printf '%s' "${UP_BODY}" | jq -r '.verified')" = "true" ] || fail "artifact .verified != true"
log "  artifact upload -> 201 verified=true artifact_id=${ART_ID}"

req POST "/releases" \
    "$(jq -nc --arg a "${ART_ID}" --arg v "${REL_VERSION}" --arg os "${DEV_OS}" --arg tm "${DEV_MODEL}" \
        '{artifact_id:$a,version:$v,os:$os,target_model:$tm,notes:"fleet e2e"}')"
need_status 201 "POST /releases (v${REL_VERSION})"
REL_ID="$(jqget '.release_id')"
req POST "/deployments" \
    "$(jq -nc --arg r "${REL_ID}" '{release_id:$r,strategy:"all-targets"}')"
need_status 201 "POST /deployments (all-targets)"
DEP_ID="$(jqget '.deployment_id')"
[ -n "${DEP_ID}" ] && [ "${DEP_ID}" != "null" ] || fail "deployment 201 but no deployment_id"
log "  release_id=${REL_ID} deployment_id=${DEP_ID}"
hr

# --- (e) launch the FLEET concurrently ----------------------------------------
# Each emulator is a DISTINCT device (distinct hardware_id). They run in parallel
# detached containers; each writes its own Outcome JSON. §11.4.119: no two share
# an exclusive resource — they are independent devices against one control plane.
log "FLEET (e): launch ${FLEET_SIZE} emulator containers CONCURRENTLY"
declare -a CTRS=() HWIDS=()
for n in $(seq 1 "${FLEET_SIZE}"); do
    idx="$(printf '%02d' "${n}")"
    ctr="helix-emu-${idx}"
    hwid="rk3588-fleet-${RUN_STAMP}-${idx}"
    CTRS+=("${ctr}")
    HWIDS+=("${hwid}")
    # Detached run; the container exits when the single -once cycle completes.
    "${PODMAN}" run -d --pod "${POD_NAME}" --name "${ctr}" \
        -e "OTA_BASE_URL=http://127.0.0.1:${HELIX_PORT}${API_BASE}" \
        --memory 192m \
        "${EMU_RUN_IMAGE}" \
        -base "http://127.0.0.1:${HELIX_PORT}${API_BASE}" \
        -admin-user "${ADMIN_USER}" \
        -admin-pass "${ADMIN_PASS}" \
        -hardware-id "${hwid}" \
        -model "${DEV_MODEL}" \
        -os "${DEV_OS}" \
        -current-version "${DEV_CUR}" \
        -once \
        >/dev/null 2>>"${TRANSCRIPT}" \
        || fail "could not launch emulator container ${ctr}"
    log "  launched ${ctr} (hardware_id=${hwid})"
done
hr

# --- (f) wait for the whole fleet to finish -----------------------------------
log "FLEET (f): wait for all ${FLEET_SIZE} emulator containers to exit"
deadline=$(( $(date +%s) + 180 ))
for ctr in "${CTRS[@]}"; do
    while :; do
        state="$("${PODMAN}" inspect -f '{{.State.Status}}' "${ctr}" 2>/dev/null || echo unknown)"
        [ "${state}" = "exited" ] && break
        [ "$(date +%s)" -ge "${deadline}" ] && { log "  timeout waiting for ${ctr} (state=${state})"; break; }
        sleep 0.5
    done
done
log "  fleet containers reached terminal state; collecting per-device evidence"
hr

# --- (g) collect per-device evidence + assert each device completed -----------
log "ASSERT (g): per-device OTA lifecycle across the fleet"
COMPLETED=0
: > "${FLEET_SUMMARY}"
printf '%-16s %-26s %-9s %-8s %-9s %-9s %-9s\n' \
    "container" "hardware_id" "rc" "applied" "from->to" "tel_acc" "tel_rej" >> "${FLEET_SUMMARY}"
for i in "${!CTRS[@]}"; do
    ctr="${CTRS[$i]}"; hwid="${HWIDS[$i]}"
    out_json="${FLEET_DIR}/${ctr}_outcome.json"
    log_file="${FLEET_DIR}/${ctr}.log"
    "${PODMAN}" logs "${ctr}" > "${out_json}" 2> "${log_file}" || true
    rc="$("${PODMAN}" inspect -f '{{.State.ExitCode}}' "${ctr}" 2>/dev/null || echo 99)"

    OUT="$(cat "${out_json}" 2>/dev/null || echo '')"
    dev_id="$(printf '%s' "${OUT}" | jq -r '.device_id' 2>/dev/null || echo '')"
    applied="$(printf '%s' "${OUT}" | jq -r '.applied' 2>/dev/null || echo '')"
    fromv="$(printf '%s' "${OUT}" | jq -r '.from_version' 2>/dev/null || echo '')"
    tov="$(printf '%s' "${OUT}" | jq -r '.to_version' 2>/dev/null || echo '')"
    acc="$(printf '%s' "${OUT}" | jq -r '.telemetry_accepted' 2>/dev/null || echo '')"
    rej="$(printf '%s' "${OUT}" | jq -r '.telemetry_rejected' 2>/dev/null || echo '')"
    dep="$(printf '%s' "${OUT}" | jq -r '.deployment_id' 2>/dev/null || echo '')"

    printf '%-16s %-26s %-9s %-8s %-9s %-9s %-9s\n' \
        "${ctr}" "${hwid}" "${rc}" "${applied}" "${fromv}->${tov}" "${acc}" "${rej}" >> "${FLEET_SUMMARY}"

    ok=1
    [ "${rc}" = "0" ] || ok=0
    [ -n "${dev_id}" ] && [ "${dev_id}" != "null" ] || ok=0
    [ "${applied}" = "true" ] || ok=0
    [ "${fromv}" = "${DEV_CUR}" ] || ok=0
    [ "${tov}" = "${REL_VERSION}" ] || ok=0
    [ "${rej}" = "0" ] || ok=0
    [ "${acc}" -ge 1 ] 2>/dev/null || ok=0
    [ "${dep}" = "${DEP_ID}" ] || ok=0

    if [ "${ok}" -eq 1 ]; then
        COMPLETED=$((COMPLETED+1))
        log "  [OK]   ${ctr}: device_id=${dev_id} ${fromv}->${tov} applied tel(acc=${acc},rej=0) dep=${dep}"
    else
        log "  [FAIL] ${ctr}: rc=${rc} device_id=${dev_id} applied=${applied} ${fromv}->${tov} tel(acc=${acc},rej=${rej}) dep=${dep}"
        log "         Outcome: $(printf '%s' "${OUT}" | tr '\n' ' ' | head -c 400)"
    fi
done
hr
log "FLEET SUMMARY (${COMPLETED}/${FLEET_SIZE} devices completed the full update):"
sed 's/^/    /' "${FLEET_SUMMARY}" | tee -a "${TRANSCRIPT}"
hr

# --- (h) server-side fleet cross-check: telemetry overview --------------------
log "ASSERT (h): server fleet telemetry overview cross-check"
req GET "/telemetry/overview" "" "${OP_TOKEN}"
need_status 200 "GET /telemetry/overview"
printf '%s' "${HTTP_BODY}" | jq . > "${QA_DIR}/server_telemetry_overview.json" 2>/dev/null \
    || printf '%s' "${HTTP_BODY}" > "${QA_DIR}/server_telemetry_overview.json"
SUCCESS_COUNT="$(printf '%s' "${HTTP_BODY}" | jq -r '.event_counts.success // 0' 2>/dev/null)"
log "  server-side success-event count across fleet: ${SUCCESS_COUNT} (want >= ${FLEET_SIZE})"
"${PODMAN}" logs "${CP_CTR}" > "${QA_DIR}/control_plane_after_fleet.log" 2>&1 || true
hr

# --- verdict ------------------------------------------------------------------
[ "${COMPLETED}" -eq "${FLEET_SIZE}" ] \
    || fail "only ${COMPLETED}/${FLEET_SIZE} fleet devices completed the full update"
[ "${SUCCESS_COUNT}" -ge "${FLEET_SIZE}" ] 2>/dev/null \
    || fail "server success-event count ${SUCCESS_COUNT} < fleet size ${FLEET_SIZE}"

log "RESULT: PASS — multi-device FLEET OTA proven END-TO-END on podman."
log "  fleet size    : ${FLEET_SIZE} distinct emulator containers (concurrent)"
log "  staged        : one all-targets deployment ${DEP_ID} at v${REL_VERSION}"
log "  per-device    : ${COMPLETED}/${FLEET_SIZE} registered + applied ${DEV_CUR}->${REL_VERSION}, telemetry accepted (rejected=0)"
log "  server xcheck : ${SUCCESS_COUNT} success events recorded fleet-wide"
log "  evidence      : ${QA_DIR}/ (fleet_summary.txt, fleet/*.json, server_telemetry_overview.json)"
hr
log "Captured evidence directory: ${QA_DIR}"
exit 0
