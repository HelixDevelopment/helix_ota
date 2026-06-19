#!/bin/sh
# =============================================================================
# helix-ab-confirm.sh — Helix OTA healthy-boot marker (PWU-AB-4 §4)
# -----------------------------------------------------------------------------
# Purpose:
#   Confirm a just-applied slot as healthy, freezing the U-Boot bootcount
#   counter so no rollback fires. Runs at boot inside the guest, after the
#   system has reached a state where the OTA agent can be considered
#   healthy (network up, control plane reachable).
#
#   Per the U-Boot bootcount doc, the healthy-boot reset is "the
#   responsibility of some application code (typically a Linux application)"
#   — this script IS that application code. It is intentionally NOT in
#   boot.cmd (uboot_ab/README.md:76-80).
#
# Behaviour:
#   1. If upgrade_available != 1, exit 0 immediately (no pending update to
#      confirm — idempotent on an already-confirmed slot).
#   2. Verify system is genuinely healthy by probing the OTA control plane
#      health endpoint (the loop-closed signal — the agent registered + the
#      server is reachable).
#   3. On success: fw_setenv upgrade_available 0; fw_setenv bootcount 0
#      -> slot CONFIRMED good, counter frozen.
#   4. On failure: exit 1 (the bootcount keeps climbing; if it passes
#      bootlimit, the proven PWU-AB-3 boot.cmd guard auto-rollbacks).
#
# Idempotent + crash-safe: re-running on an already-confirmed slot is a
# no-op (step 1 catches upgrade_available=0). fw_setenv is atomic for
# single-variable writes.
#
# Dependencies:
#   - fw_setenv/fw_printenv (u-boot-tools) on PATH
#   - /etc/fw_env.config pointing at the U-Boot env storage (PWU-AB-4 §3)
#   - wget (BusyBox wget) for the health probe
#   - /etc/ota-server-url containing the OTA server host:port (optional;
#     defaults to "ota-server:8080")
#
# Design: docs/design/rk3588_ab_virt/PWU_AB_4_APPLY_PORT.md §4
# =============================================================================

set -eu

# ---- config ----------------------------------------------------------------
UPGRADE_AVAILABLE_FILE="/tmp/.helix_upgrade_available"
OTA_SERVER_URL_FILE="/etc/ota-server-url"

# ---- step 1: check if there is a pending update to confirm -----------------
UA=$(fw_printenv upgrade_available 2>/dev/null || echo "0")
if [ "$UA" != "1" ]; then
    logger -t helix-ab-confirm "upgrade_available=${UA} — nothing to confirm, exiting"
    exit 0
fi

# ---- step 2: verify system is genuinely healthy ----------------------------
# The health predicate is: the OTA control plane is reachable. This proves
# the agent's registration + poll path works, closing the loop that the
# apply started.
HOST="ota-server:8080"
if [ -f "$OTA_SERVER_URL_FILE" ]; then
    HOST=$(cat "$OTA_SERVER_URL_FILE")
fi

HEALTH_URL="http://${HOST}/healthz"
logger -t helix-ab-confirm "Probing OTA server health: ${HEALTH_URL}"

# BusyBox wget returns 0 on success; -q suppresses output, -O /dev/null
# discards the body. 5-second timeout via -T 5 (BusyBox wget).
if wget -q -T 5 -O /dev/null "$HEALTH_URL" 2>/dev/null; then
    # ---- step 3: confirm the slot ------------------------------------------
    logger -t helix-ab-confirm "Health check PASSED — confirming slot"

    fw_setenv upgrade_available 0
    fw_setenv bootcount 0

    logger -t helix-ab-confirm "Slot confirmed healthy: upgrade_available=0 bootcount=0"
    exit 0
else
    # ---- step 4: health check failed, do NOT confirm -----------------------
    logger -t helix-ab-confirm "Health check FAILED (${HEALTH_URL}) — NOT confirming slot"
    logger -t helix-ab-confirm "bootcount will increment and auto-rollback may fire"
    exit 1
fi
