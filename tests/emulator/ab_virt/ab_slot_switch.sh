#!/usr/bin/env bash
# =============================================================================
# ab_slot_switch.sh — RK3588 A/B-virt emulator: PWU-AB-1 FULL slot-switch proof
# -----------------------------------------------------------------------------
# Purpose:
#   Prove the REAL bootloader-driven A/B slot switch on this Apple-Silicon host:
#   the same disk + same u-boot.bin, booted twice, differing ONLY in the U-Boot
#   BOOT_ORDER environment variable, MUST route the kernel root= to a DIFFERENT
#   physical rootfs partition, AND the guest MUST observably report (from inside
#   userspace) the slot it was actually booted into (/etc/slot_id + the mounted
#   root device). This is the OTA A/B primitive (RAUC/Mender BOOT_ORDER
#   convention) running under a real U-Boot 2024.01 + QEMU virt + HVF — not a mock.
#
# §11.4.115 RED->GREEN polarity (the proof is the CONTRAST, captured live):
#   * Run A: BOOT_ORDER="A B"  -> head=A -> root=/dev/vda2 -> guest slot_id MUST be A
#   * Run B: BOOT_ORDER="B A"  -> head=B -> root=/dev/vda3 -> guest slot_id MUST be B
#   A no-op / fake switch would report the SAME slot in both runs (RED). A real
#   switch reports A then B (GREEN). Both consoles captured in full (§11.4.107
#   live-not-frozen: real getty login + post-login command output, not a frame).
#
# Mechanism (deterministic, no distro_bootcmd dependency): interrupt U-Boot
#   autoboot -> `setenv BOOT_ORDER <order>` -> `load virtio 0:1 <addr> boot.scr`
#   -> `source <addr>` (runs uboot_ab/boot.cmd, which selects the head slot and
#   booti's the kernel with root=/dev/vda<part>). See uboot_ab/boot.cmd + README.
#
# Driver robustness (§11.4.1 — the FAIL must be a product defect, never a script
#   bug): the expect driver is emitted to a TEMP .exp FILE via a single-quoted
#   heredoc, so there is exactly ONE quoting level (Tcl). Guest-shell command
#   substitution is written `\$(...)` in Tcl => the guest shell (not Tcl) runs it.
#   Login tolerates interleaved kernel-console noise + retries the login cycle.
#
# Usage:  tests/emulator/ab_virt/ab_slot_switch.sh
#   Pre:  out/.ok + out/.disk_ok + out/images/{u-boot.bin,ab_disk.img}.
#   Env:  HELIX_AB_ROOT_PW (default helixota — must match build_image.sh).
# Outputs: evidence under docs/qa/<run-id>-ab-slot-switch/{consoleA.log,consoleB.log,
#          verdict.txt}; PASS/FAIL verdict.
# Deps: qemu-system-aarch64 (HVF), expect. Self-cleaning: each QEMU exits on guest
#   poweroff or the expect timeout (§11.4.14).
# Cross-refs: uboot_ab/README.md ; docs/research/rk3588_emulator/REPORT.md ;
#   §11.4.115 (RED->GREEN) ; §11.4.107 (live not frozen) ; §11.4.83 (evidence) ;
#   §11.4.6 (assert, never assume) ; §11.4.1 (script bugs fixed at source) ;
#   §11.4.111 (the disk is the only virtio-blk so devnum 0 is pinned).
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
ROOT_PW="${HELIX_AB_ROOT_PW:-helixota}"
IMG_DIR="${SCRIPT_DIR}/out/images"
UBOOT="${IMG_DIR}/u-boot.bin"
DISK="${IMG_DIR}/ab_disk.img"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-ab-slot-switch"
EVID="${REPO_ROOT}/docs/qa/${RUN_ID}"
mkdir -p "$EVID"
EXP="${EVID}/drive.exp"
log() { printf '%s\n' "$*"; }

# ---- preconditions ----------------------------------------------------------
[ -f "${SCRIPT_DIR}/out/.ok" ]      || { log "ABORT: out/.ok absent — run build_image.sh"; exit 3; }
[ -f "${SCRIPT_DIR}/out/.disk_ok" ] || { log "ABORT: out/.disk_ok absent — run assemble_ab_disk.sh"; exit 3; }
[ -s "$UBOOT" ] || { log "ABORT: u-boot.bin missing"; exit 3; }
[ -s "$DISK" ]  || { log "ABORT: ab_disk.img missing"; exit 3; }
command -v qemu-system-aarch64 >/dev/null || { log "ABORT: qemu-system-aarch64 not found"; exit 3; }
command -v expect >/dev/null || { log "ABORT: expect not found"; exit 3; }

log "== PWU-AB-1 FULL slot-switch proof (real U-Boot + QEMU virt + HVF) =="
log "run=${RUN_ID}  u-boot=$(du -h "$UBOOT"|cut -f1)  disk=$(du -h "$DISK"|cut -f1)"

# ---- emit the expect driver to a TEMP FILE (single Tcl quoting level) --------
# SINGLE-QUOTED heredoc: bash does NOT expand, so `\$(` reaches the file verbatim
# and Tcl turns `\$` into a literal `$` => the GUEST shell runs the substitution.
# argv: 0=pw 1=order 2=console 3=uboot 4=disk 5=mark(A|B)
cat > "$EXP" <<'EXPEOF'
set timeout 150
set pw      [lindex $argv 0]
set order   [lindex $argv 1]
set console [lindex $argv 2]
set uboot   [lindex $argv 3]
set disk    [lindex $argv 4]
set mark    [lindex $argv 5]
log_file -noappend $console

spawn qemu-system-aarch64 -M virt -accel hvf -cpu host -smp 2 -m 512 -nographic \
  -bios $uboot -drive file=$disk,if=virtio,format=raw

# Interrupt U-Boot autoboot.
expect {
  timeout { puts "HELIX_DRIVER_FAIL: no autoboot prompt"; exit 2 }
  -re {stop autoboot}
}
send "\r"
expect {
  timeout { puts "HELIX_DRIVER_FAIL: no U-Boot prompt"; exit 2 }
  -re {=> $}
}
# Drive the A/B env exactly as the OTA agent will (BOOT_ORDER head = active slot).
send "setenv BOOT_ORDER $order\r"
expect -re {=> $}
send "printenv BOOT_ORDER\r"
expect -re {=> $}
# Load + source our A/B boot script from the FAT boot partition (virtio 0:1).
send "load virtio 0:1 0x40400000 boot.scr\r"
expect {
  timeout { puts "HELIX_DRIVER_FAIL: boot.scr load timed out"; exit 2 }
  -re {=> $}
}
send "source 0x40400000\r"

# Wait for the getty login prompt (tolerate interleaved kernel-console noise),
# then run the login cycle with one retry if the prompt re-displays.
expect {
  timeout { puts "HELIX_DRIVER_FAIL: no login prompt after source"; exit 2 }
  -re {buildroot login: $}
}
set tries 0
proc do_login {} {
  global pw
  send "root\r"
  expect {
    timeout { puts "HELIX_DRIVER_FAIL: no password prompt"; exit 2 }
    -re {Password: $}
  }
  # settle past any trailing kernel-log line before sending the secret
  sleep 1
  send "$pw\r"
}
do_login
# Either we land on a shell prompt, or the login re-prompts (retry once).
expect {
  timeout { puts "HELIX_DRIVER_FAIL: no shell after login"; exit 2 }
  -re {buildroot login: $} {
    incr tries
    if {$tries > 2} { puts "HELIX_DRIVER_FAIL: login kept re-prompting"; exit 2 }
    do_login
    exp_continue
  }
  -re {# $}
}
# Drain to a clean prompt.
send "\r"
expect -re {# $}
# Observable slot identity FROM INSIDE THE GUEST. `\$(...)` => guest shell runs it.
send "echo HELIX_SLOTID=\$(cat /etc/slot_id 2>/dev/null)\r"
expect -re {# $}
send "echo HELIX_ROOTDEV=\$(findmnt -no SOURCE / 2>/dev/null)\r"
expect -re {# $}
send "echo HELIX_CMDLINE=\$(cat /proc/cmdline)\r"
expect -re {# $}
send "echo HELIX_DONE_${mark}_MARK\r"
expect -re {# $}
send "poweroff\r"
expect {
  timeout { puts "HELIX_DRIVER_NOTE: poweroff timed out"; exit 0 }
  eof
}
EXPEOF

drive_boot() {  # $1=order  $2=console  $3=mark
  expect -f "$EXP" "$ROOT_PW" "$1" "$2" "$UBOOT" "$DISK" "$3" >> "${2}.driver" 2>&1
  return $?
}

log ""
log "-- Run A: BOOT_ORDER='A B' (head=A -> expect root=/dev/vda2, slot_id=A) --"
drive_boot "A B" "${EVID}/consoleA.log" "A"; rcA=$?
log "-- Run B: BOOT_ORDER='B A' (head=B -> expect root=/dev/vda3, slot_id=B) --"
drive_boot "B A" "${EVID}/consoleB.log" "B"; rcB=$?

# ---- anti-bluff assertions (the proof is the CONTRAST) ----------------------
fail=0
chk() { if grep -aqE "$2" "$1"; then log "[PASS] $3"; else log "[FAIL] $3"; fail=1; fi; }
nchk(){ if grep -aqE "$2" "$1"; then log "[FAIL] $3"; fail=1; else log "[PASS] $3"; fi; }

# Run A booted slot A: slot_id=A AND root vda2 AND kernel selected A AND NOT slot B.
chk  "${EVID}/consoleA.log" 'HELIX_SLOTID=A'             "Run A: guest userspace reports /etc/slot_id=A"
chk  "${EVID}/consoleA.log" 'HELIX_ROOTDEV=/dev/vda2'    "Run A: guest findmnt confirms root = /dev/vda2 (slot A)"
chk  "${EVID}/consoleA.log" 'helix_slot=A'               "Run A: kernel cmdline carries helix_slot=A (boot.scr selected A)"
chk  "${EVID}/consoleA.log" 'HELIX_DONE_A_MARK'          "Run A: interactive shell live (post-login sentinel, not a frozen frame)"
nchk "${EVID}/consoleA.log" 'HELIX_SLOTID=B'             "Run A: did NOT boot slot B"
# Run B booted slot B: slot_id=B AND root vda3 AND kernel selected B AND NOT slot A.
chk  "${EVID}/consoleB.log" 'HELIX_SLOTID=B'             "Run B: guest userspace reports /etc/slot_id=B (SLOT SWITCHED)"
chk  "${EVID}/consoleB.log" 'HELIX_ROOTDEV=/dev/vda3'    "Run B: guest findmnt confirms root = /dev/vda3 (slot B)"
chk  "${EVID}/consoleB.log" 'helix_slot=B'               "Run B: kernel cmdline carries helix_slot=B (boot.scr selected B)"
chk  "${EVID}/consoleB.log" 'HELIX_DONE_B_MARK'          "Run B: interactive shell live (post-login sentinel, not a frozen frame)"
nchk "${EVID}/consoleB.log" 'HELIX_SLOTID=A'             "Run B: did NOT boot slot A (the switch is real, not a no-op)"

{
  echo "PWU-AB-1 FULL slot-switch — run ${RUN_ID}"
  echo "u-boot.bin: $(strings "$UBOOT" | grep -m1 -iE '^U-Boot 20')"
  echo "Run A rc=${rcA}  Run B rc=${rcB}"
  echo "Run A: $(grep -aoE 'HELIX_SLOTID=[AB]' "${EVID}/consoleA.log" | head -1)  $(grep -aoE 'HELIX_ROOTDEV=/dev/vda[0-9]' "${EVID}/consoleA.log" | head -1)"
  echo "Run B: $(grep -aoE 'HELIX_SLOTID=[AB]' "${EVID}/consoleB.log" | head -1)  $(grep -aoE 'HELIX_ROOTDEV=/dev/vda[0-9]' "${EVID}/consoleB.log" | head -1)"
  echo "Verdict: $([ "$fail" -eq 0 ] && echo PASS || echo FAIL)"
} > "${EVID}/verdict.txt"

log ""
cat "${EVID}/verdict.txt"
log "EVIDENCE: ${EVID}/ (consoleA.log $(wc -l < "${EVID}/consoleA.log" 2>/dev/null|tr -d ' ') lines, consoleB.log $(wc -l < "${EVID}/consoleB.log" 2>/dev/null|tr -d ' ') lines)"
if [ "$fail" -eq 0 ]; then
  log "RESULT: PASS — real bootloader-driven A/B slot switch proven (A->vda2, B->vda3) under U-Boot+QEMU+HVF."
  exit 0
fi
log "RESULT: FAIL — see ${EVID}/ (qemu/expect rcA=${rcA} rcB=${rcB})"
exit 1
