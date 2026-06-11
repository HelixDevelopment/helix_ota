#!/usr/bin/env bash
# =============================================================================
# boot_smoke.sh — RK3588 A/B-virt emulator: base-image boot smoke (PWU-AB-1 foundation)
# -----------------------------------------------------------------------------
# Purpose:
#   Boot the base aarch64 Buildroot guest image (built by build_image.sh) under
#   QEMU `-machine virt` + HVF on this Apple-Silicon host, and prove it reaches a
#   LIVE, INTERACTIVE Linux userspace — not a frozen frame (§11.4.107): a
#   deterministic `expect` driver waits for the real login prompt, logs in, runs
#   `uname`/`os-release`, and prints a sentinel that can ONLY appear from an
#   interactive shell AFTER a successful login. This is the foundation the A/B
#   layers (U-Boot bootcount + RAUC dm-verity slots) build on.
#
# Anti-bluff (§11.4 / §11.4.107): a PASS requires ALL of (a) the kernel booting on
#   the real Apple CPU via HVF, (b) the Buildroot userspace banner + getty login
#   prompt, (c) the post-login sentinel + a real `uname` line — captured in a full
#   boot transcript, not a single screenshot.
#
# Usage:  tests/emulator/ab_virt/boot_smoke.sh
#   Pre:  out/.ok + out/images/{Image,rootfs.ext2} present (run build_image.sh first).
#   Env:  HELIX_AB_ROOT_PW (default helixota — must match build_image.sh).
# Outputs: evidence transcript under docs/qa/<run-id>-ab-virt-boot/ ; PASS/FAIL verdict.
# Deps: qemu-system-aarch64 (HVF), expect. Self-cleaning: QEMU exits on guest poweroff
#   or the expect timeout (§11.4.14).
# Cross-refs: docs/research/rk3588_emulator/REPORT.md ; §11.4.83 evidence ; §11.4.6.
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
ROOT_PW="${HELIX_AB_ROOT_PW:-helixota}"
IMG_DIR="${SCRIPT_DIR}/out/images"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-ab-virt-boot"
EVID="${REPO_ROOT}/docs/qa/${RUN_ID}"
CONSOLE="${EVID}/console.log"

mkdir -p "$EVID"
log() { printf '%s\n' "$*"; }

# ---- preconditions ----------------------------------------------------------
[ -f "${SCRIPT_DIR}/out/.ok" ] || { log "ABORT: out/.ok absent — run build_image.sh first"; exit 3; }
[ -s "${IMG_DIR}/Image" ] && [ -s "${IMG_DIR}/rootfs.ext2" ] || { log "ABORT: image artifacts missing"; exit 3; }
command -v qemu-system-aarch64 >/dev/null || { log "ABORT: qemu-system-aarch64 not found"; exit 3; }
command -v expect >/dev/null || { log "ABORT: expect not found"; exit 3; }

log "== PWU-AB-1 base-image boot smoke (QEMU virt + HVF) =="
log "run=${RUN_ID}  image=$(du -h "${IMG_DIR}/Image" | cut -f1) kernel + $(du -h "${IMG_DIR}/rootfs.ext2" | cut -f1) rootfs"

# ---- deterministic expect driver: boot -> login -> prove interactive --------
# A writable overlay-free run: the rootfs is mounted rw but we poweroff cleanly.
expect -c "
set timeout 90
log_file -noappend ${CONSOLE}
spawn qemu-system-aarch64 -M virt -accel hvf -cpu host -smp 2 -m 512 -nographic \
  -kernel ${IMG_DIR}/Image \
  -append {rootwait root=/dev/vda console=ttyAMA0} \
  -drive file=${IMG_DIR}/rootfs.ext2,if=none,format=raw,id=hd0 \
  -device virtio-blk-device,drive=hd0
expect {
  timeout { puts {HELIX_DRIVER_FAIL: no login prompt}; exit 2 }
  -re {buildroot login:}
}
send \"root\r\"
expect {
  timeout { puts {HELIX_DRIVER_FAIL: no password prompt}; exit 2 }
  -re {Password:}
}
send \"${ROOT_PW}\r\"
expect {
  timeout { puts {HELIX_DRIVER_FAIL: no shell prompt}; exit 2 }
  -re {# }
}
send \"uname -a\r\"
expect -re {# }
send \"cat /etc/os-release | head -2\r\"
expect -re {# }
send \"echo HELIX_USERSPACE_LIVE_OK\r\"
expect -re {# }
send \"poweroff\r\"
expect {
  timeout { puts {HELIX_DRIVER_NOTE: poweroff timed out}; exit 0 }
  eof
}
" >> "${CONSOLE}.driver" 2>&1
QRC=$?

# ---- anti-bluff assertions (all must hold) ----------------------------------
fail=0
check() { if grep -aqE "$1" "$CONSOLE"; then log "[PASS] $2"; else log "[FAIL] $2"; fail=1; fi; }
check 'Booting Linux on physical CPU .*0x610f'           "kernel booted on the real Apple CPU via HVF (MIDR 0x610f*)"
check 'Welcome to Buildroot'                              "Buildroot userspace init reached"
check 'buildroot login:'                                  "getty login prompt presented (userspace up)"
check 'Linux buildroot .* aarch64 GNU/Linux'              "post-login 'uname -a' returned a live aarch64 kernel string"
check 'HELIX_USERSPACE_LIVE_OK'                           "INTERACTIVE sentinel printed AFTER login (live shell, not a frozen frame)"

log ""
if [ "$fail" -eq 0 ]; then
  log "RESULT: PASS — base A/B-virt image boots to a live interactive aarch64 userspace on QEMU+HVF."
  log "EVIDENCE: ${CONSOLE} ($(wc -l < "$CONSOLE" 2>/dev/null | tr -d ' ') lines)"
  exit 0
fi
log "RESULT: FAIL — see ${CONSOLE} (qemu/expect rc=${QRC})"
exit 1
