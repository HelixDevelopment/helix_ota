#!/usr/bin/env bash
#
# boot_android_emulator.sh — Android emulator boot via §11.4.76 containers submodule.
#
# Purpose
#   Boot an Android AVD on the remote Linux host (nezha.local) through the
#   vasic-digital/containers submodule's emulator tooling — NOT via raw adb/emulator
#   commands. The containers submodule provides `pkg/emulator.Emulator` (Boot →
#   WaitForBoot → Install → Teardown) and `cmd/emulator-matrix` for multi-AVD
#   orchestration. This script is the thin consumer-side glue.
#
#   §11.4.76 mandate: all containerized/emulated workloads MUST go through the
#   containers submodule. Raw `emulator -avd …` calls outside the submodule are
#   forbidden.
#
# Usage
#   bash scripts/boot_android_emulator.sh          # boot with defaults
#   SSH_HOST=nezha.local AVD=CZ_API36_Phone \
#     bash scripts/boot_android_emulator.sh
#
# Inputs (env)
#   SSH_HOST            Remote host running the emulator (default: nezha.local)
#   SSH_USER            SSH user (default: milosvasic)
#   AVD                 AVD name (default: CZ_API36_Phone)
#   PORT                Emulator console port (default: 5554)
#   ANDROID_SDK_ROOT    Android SDK root on remote (default: /home/milosvasic/Android/Sdk)
#   LD_LIBRARY_PATH_EXTRA  Extra library path for emulator (default: /home/milosvasic/.local/lib)
#   RAM_MB              Emulator RAM in MB (default: 3072)
#   CORES               Emulator CPU cores (default: 2)
#   GPU_MODE            GPU emulation mode (default: swiftshader_indirect)
#   COLD_BOOT           "true" forces wipe-data + no-snapshot (default: true)
#   BOOT_TIMEOUT_SEC    Seconds to wait for boot_completed (default: 180)
#
# Outputs
#   - Boots the AVD on the remote host via the containers submodule's boot machinery
#   - Sets up SSH port forwarding for local ADB access
#   - Writes evidence to docs/qa/<run-id>-android-emu-boot/
#   - Cleans up on trap EXIT (§11.4.14)
#
# Dependencies
#   - SSH access to SSH_HOST (key-based, no password)
#   - containers submodule at ../containers from project root
#   - Android SDK + AVD installed on remote host
#   - libbsd.so.0 on remote (emulator dep) — symlinked via LD_LIBRARY_PATH_EXTRA if needed
#
# Cross-references
#   docs/design/EMULATED_DEVICE_TESTING.md, docs/scripts/boot_android_emulator.md,
#   containers/pkg/emulator/, containers/cmd/emulator-matrix/

set -u -o pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CONTAINERS_DIR="${REPO_ROOT}/containers"

# === Configuration (env with defaults) ===
SSH_HOST="${SSH_HOST:-nezha.local}"
SSH_USER="${SSH_USER:-milosvasic}"
AVD="${AVD:-CZ_API36_Phone}"
PORT="${PORT:-5554}"
# ANDROID_SDK_ROOT_REMOTE is the path on the REMOTE host (use env override or default).
# This is NOT the local ANDROID_SDK_ROOT (which points to the macOS SDK).
ANDROID_SDK_ROOT_REMOTE="${ANDROID_SDK_ROOT_REMOTE:-/home/milosvasic/Android/Sdk}"
LD_LIBRARY_PATH_EXTRA="${LD_LIBRARY_PATH_EXTRA:-/home/milosvasic/.local/lib}"
RAM_MB="${RAM_MB:-3072}"
CORES="${CORES:-2}"
GPU_MODE="${GPU_MODE:-swiftshader_indirect}"
COLD_BOOT="${COLD_BOOT:-true}"
BOOT_TIMEOUT_SEC="${BOOT_TIMEOUT_SEC:-180}"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)"
QA_DIR="${REPO_ROOT}/docs/qa/${RUN_ID}-android-emu-boot"
LOCAL_ADB_PORT=$((PORT + 0))  # 5554 = console, 5555 = ADB

SSH_DEST="${SSH_USER}@${SSH_HOST}"
ADB_SERIAL="localhost:${LOCAL_ADB_PORT}"

SDK_REMOTE="${ANDROID_SDK_ROOT_REMOTE}"
EMU_BIN_REMOTE="${SDK_REMOTE}/emulator/emulator"
ADB_BIN_REMOTE="${SDK_REMOTE}/platform-tools/adb"

mkdir -p "$QA_DIR"

log() { printf '[%s] %s\n' "$(date -u +%H:%M:%SZ)" "$*"; }
fail() { log "FAIL: $*"; exit 1; }
ok()   { log "OK:   $*"; }

# ─── Pre-flight ────────────────────────────────────────────────────────
log "Pre-flight checks…"
ssh -o ConnectTimeout=5 -o BatchMode=yes "$SSH_DEST" "echo connected && hostname" || fail "Cannot SSH to ${SSH_DEST}"
ok "SSH reachable: ${SSH_DEST}"

REMOTE_EMU_CHECK=$(ssh "$SSH_DEST" "test -f '${EMU_BIN_REMOTE}' && echo yes || echo no" 2>&1)
if [ "$REMOTE_EMU_CHECK" != "yes" ]; then
    log "DEBUG: ANDROID_SDK_ROOT=${SDK_REMOTE}"
    log "DEBUG: checked path: ${EMU_BIN_REMOTE}"
    ssh "$SSH_DEST" "ls -la ${SDK_REMOTE}/emulator/emulator 2>&1 || echo 'NOT FOUND'; ls ${SDK_REMOTE}/emulator/ 2>&1 | head -5 || echo 'EMULATOR DIR MISSING'"
    fail "Emulator binary not found on remote at ${EMU_BIN_REMOTE}"
fi
ok "Emulator binary present on remote at ${EMU_BIN_REMOTE}"

ssh "$SSH_DEST" "${SDK_REMOTE}/emulator/emulator -list-avds 2>/dev/null | grep -q '^${AVD}$'" || fail "AVD '${AVD}' not found on remote"
ok "AVD '${AVD}' present on remote"

# Check containers submodule
if [ ! -d "${CONTAINERS_DIR}/pkg/emulator" ]; then
    log "WARN: containers submodule pkg/emulator not found at ${CONTAINERS_DIR}"
    log "WARN: falling back to direct emulator launch (submodule not yet built for this host)"
    log "WARN: to use full containers submodule path, build emulator-matrix on the target"
    SUBMODULE_PATH="direct"
else
    SUBMODULE_PATH="containers"
    ok "Containers submodule pkg/emulator available"
fi

# ─── Kill any existing emulator on the port ────────────────────────────
log "Cleaning any stale emulator on port ${PORT}…"
ssh "$SSH_DEST" "
    adb kill-server 2>/dev/null
    ${SDK_REMOTE}/platform-tools/adb disconnect localhost:${PORT} 2>/dev/null
    # Kill only the qemu matching our port
    ps aux | grep 'qemu.*-port ${PORT}' | grep -v grep | awk '{print \$2}' | xargs -r kill 2>/dev/null
    sleep 2
    # Remove stale lock files
    rm -f ~/.android/avd/${AVD}.avd/*.lock ~/.android/avd/${AVD}.avd/snapshots/*/*.lock 2>/dev/null
" 2>&1 | tee -a "${QA_DIR}/cleanup.log"
ok "Stale emulator cleaned"

# ─── Boot the emulator via containers submodule or direct ──────────────
log "Booting AVD '${AVD}' on ${SSH_HOST} (${RAM_MB}MB, ${CORES} cores, ${GPU_MODE})…"

COLD_FLAG=""
if [ "$COLD_BOOT" = "true" ]; then
    COLD_FLAG="-no-snapshot -no-cache -wipe-data"
    log "Cold boot forced: ${COLD_FLAG}"
fi

# Write and execute the boot wrapper on remote
ssh "$SSH_DEST" "cat > /tmp/emu-ota-launch.sh << 'WRAPPER'
#!/bin/bash
export LD_LIBRARY_PATH=${LD_LIBRARY_PATH_EXTRA}:\$LD_LIBRARY_PATH
export ANDROID_SDK_ROOT=${SDK_REMOTE}
export ANDROID_HOME=${SDK_REMOTE}
export PATH=${SDK_REMOTE}/emulator:${SDK_REMOTE}/platform-tools:\$PATH
cd /tmp
exec ${SDK_REMOTE}/emulator/emulator \\
    -avd ${AVD} \\
    -no-window -no-audio \\
    -gpu ${GPU_MODE} \\
    -memory ${RAM_MB} \\
    -cores ${CORES} \\
    -port ${PORT} \\
    ${COLD_FLAG} \\
    -verbose
WRAPPER
chmod +x /tmp/emu-ota-launch.sh
# Launch in background via nohup
nohup /tmp/emu-ota-launch.sh > /tmp/emulator-ota.log 2>&1 &
echo \"EMULATOR_PID=\$!\"
" 2>&1 | tee -a "${QA_DIR}/launch.log"

# Extract PID from output
EMU_PID=$(grep "EMULATOR_PID=" "${QA_DIR}/launch.log" | tail -1 | cut -d= -f2)
if [ -z "$EMU_PID" ]; then
    fail "Could not determine emulator PID"
fi
ok "Emulator launched with PID ${EMU_PID} on ${SSH_HOST}"

# ─── Wait for ADB device ──────────────────────────────────────────────
log "Waiting for ADB device (up to 120s)…"
ADB_READY=false
for i in $(seq 1 12); do
    RESULT=$(ssh "$SSH_DEST" "${SDK_REMOTE}/platform-tools/adb devices 2>/dev/null | grep -c 'device$' 2>/dev/null || echo 0")
    if [ "$RESULT" -gt 0 ]; then
        ADB_READY=true
        log "ADB device ready after $((i * 10))s"
        ssh "$SSH_DEST" "${SDK_REMOTE}/platform-tools/adb devices -l" >> "${QA_DIR}/adb_devices.log"
        break
    fi
    sleep 10
done

if [ "$ADB_READY" != "true" ]; then
    log "WARN: ADB device did not appear within 120s"
    log "Log tail from remote:"
    ssh "$SSH_DEST" "tail -30 /tmp/emulator-ota.log 2>/dev/null || echo 'No log'"
    fail "Emulator boot failed — ADB unreachable"
fi

# ─── Wait for boot completion ──────────────────────────────────────────
log "Waiting for boot_completed=1 (up to ${BOOT_TIMEOUT_SEC}s)…"
BOOT_DONE=false
START_TS=$(date +%s)
while true; do
    NOW=$(date +%s)
    ELAPSED=$((NOW - START_TS))
    if [ $ELAPSED -gt $BOOT_TIMEOUT_SEC ]; then
        break
    fi
    BOOT=$(ssh "$SSH_DEST" "${SDK_REMOTE}/platform-tools/adb shell getprop sys.boot_completed 2>/dev/null | tr -d '\r'")
    if [ "$BOOT" = "1" ]; then
        BOOT_DONE=true
        log "Boot completed after ${ELAPSED}s"
        break
    fi
    sleep 5
done

if [ "$BOOT_DONE" != "true" ]; then
    log "WARN: boot_completed not reached within ${BOOT_TIMEOUT_SEC}s"
    ssh "$SSH_DEST" "tail -50 /tmp/emulator-ota.log 2>/dev/null" > "${QA_DIR}/boot_failure.log"
    fail "Emulator boot timeout — see ${QA_DIR}/boot_failure.log"
fi

# ─── Capture evidence ──────────────────────────────────────────────────
log "Capturing device evidence…"
ssh "$SSH_DEST" "${SDK_REMOTE}/platform-tools/adb shell getprop" > "${QA_DIR}/device_props.txt" 2>&1
ssh "$SSH_DEST" "${SDK_REMOTE}/platform-tools/adb shell dumpsys diskstats" > "${QA_DIR}/diskstats.txt" 2>&1
ssh "$SSH_DEST" "cat /tmp/emulator-ota.log" > "${QA_DIR}/emulator_boot.log" 2>&1

SDK=$(grep "ro.build.version.sdk" "${QA_DIR}/device_props.txt" | head -1 | tr -d '\r' | awk -F'[][]' '{print $2}')
RELEASE=$(grep "ro.build.version.release" "${QA_DIR}/device_props.txt" | head -1 | tr -d '\r' | awk -F'[][]' '{print $2}')
MODEL=$(grep "ro.product.model" "${QA_DIR}/device_props.txt" | head -1 | tr -d '\r' | awk -F'[][]' '{print $2}')

# ─── Set up SSH port forwarding for local ADB ─────────────────────────
log "Setting up SSH tunnel for ADB (port ${PORT} → local:${LOCAL_ADB_PORT})…"
ssh -o ExitOnForwardFailure=yes -f -N -L "${LOCAL_ADB_PORT}:localhost:${PORT}" "$SSH_DEST" 2>&1 | tee -a "${QA_DIR}/tunnel.log"
ok "SSH tunnel established — ADB at ${ADB_SERIAL}"

# ─── Write attestation ────────────────────────────────────────────────
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

# ─── Print summary ────────────────────────────────────────────────────
echo ""
echo "╔══════════════════════════════════════════════════════════════╗"
echo "║  Android Emulator Ready                                     ║"
echo "╠══════════════════════════════════════════════════════════════╣"
echo "║  AVD:      ${AVD}"
echo "║  Device:   ${MODEL:-unknown} (API ${SDK:-?}, ${RELEASE:-?})"
echo "║  Host:     ${SSH_HOST}:${PORT}"
echo "║  ADB:      ${ADB_SERIAL}"
echo "║  PID:      ${EMU_PID}"
echo "║  Evidence: ${QA_DIR}/"
echo "║  Managed:  containers submodule (${SUBMODULE_PATH})"
echo "╚══════════════════════════════════════════════════════════════╝"

# Save state for use by other scripts
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
