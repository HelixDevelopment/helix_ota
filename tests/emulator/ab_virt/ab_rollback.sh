#!/usr/bin/env bash
# =============================================================================
# ab_rollback.sh — RK3588 A/B-virt emulator: PWU-AB-3 corrupt-slot AUTO-ROLLBACK
# -----------------------------------------------------------------------------
# STATUS (§11.4.6): UNVERIFIED-pending — authored, NOT yet run by the conductor;
#   persistent-env power-cycle rollback is a separate later PWU. This script is
#   the design + skeleton of the auto-rollback proof. No working rollback is
#   claimed here until the conductor assembles the disk, boots it, and the
#   verdict.txt below reads PASS from a real run (see UNVERIFIED list at bottom).
#
# Purpose:
#   Prove the REAL bootloader-driven A/B AUTO-ROLLBACK on this Apple-Silicon
#   host: when the head slot of BOOT_ORDER has CONSUMED its update probation
#   (an UNHEALTHY slot whose in-guest healthy-boot marker never cleared the
#   counter, so bootcount climbs past bootlimit), U-Boot's bootcount guard in
#   boot.cmd MUST swap BOOT_ORDER so the OTHER (previous-good) slot becomes the
#   head, and the guest MUST then boot + observably report the PREVIOUS-GOOD
#   slot. This is the RAUC/Mender "swap-the-rootfs altbootcmd" rollback action
#   running under a real U-Boot 2024.01 + QEMU virt + HVF — not a mock. It is
#   the auto-rollback headline of the A/B OTA primitive.
#
# Scenario modelled — a BAD slot-B update rolls back to good slot A:
#   The OTA agent applied an update to slot B and armed it
#   (BOOT_ORDER="B A", upgrade_available=1). Slot B is UNHEALTHY: it never
#   reaches the in-guest healthy-boot marker, so the counter is never reset and
#   bootcount climbs. Once bootcount > bootlimit the boot.cmd step-2 guard fires:
#   swap "B A" -> "A B", reset bootcount=1, then select head A -> /dev/vda2.
#   Result: the head was B (the bad update) but the guest BOOTS A (rollback).
#
# §11.4.115 RED->GREEN polarity (the proof is the CONTRAST, captured live):
#   * Run BADB-ROLLBACK: BOOT_ORDER="B A" + bootcount=2 + bootlimit=1
#       -> guard fires -> swaps to "A B" -> boots /dev/vda2 -> guest slot_id A.
#       A broken rollback (guard absent / not firing) would leave head=B and
#       boot /dev/vda3 -> slot_id=B (RED — the bad update kept booting). A real
#       rollback reports slot_id=A (GREEN). The U-Boot console MUST carry the
#       "rolling back (altbootcmd swap)" line emitted by the guard.
#   * Run CONTROL-NOROLLBACK: BOOT_ORDER="B A" + bootcount=1 + bootlimit=1
#       (NOT past the limit) -> guard does NOT fire -> boots head B -> slot_id B.
#       This control proves the rollback is TRIGGERED BY the exhausted counter,
#       not by anything else (a metamorphic relation: same order, the ONLY
#       difference is bootcount, and ONLY the past-limit run rolls back).
#
# State-machine refs (full description: uboot_ab/README.md "Slot-select ->
# rollback state machine", boot.cmd lines 53-69 "bootlimit / altbootcmd guard"):
#   * bootcount  — U-Boot increments it each reboot while upgrade_available!=0;
#                  here we SET it directly at the U-Boot prompt to simulate a
#                  slot that has already consumed its attempts (the persistent
#                  saveenv-across-power-cycle path is a later PWU; PWU-AB-1 +
#                  this PWU-AB-3 drive the env in a SINGLE in-RAM session).
#   * bootlimit  — when bootcount > bootlimit, the guard rolls back. bootlimit=1
#                  => a freshly-updated slot gets exactly one attempt.
#   * BOOT_ORDER — head = slot tried first; the guard swaps "B A" <-> "A B".
#   * upgrade_available — 1 while the update is on probation; the UNHEALTHY slot
#                  never clears it (the in-guest healthy-boot marker is a later
#                  PWU, so an unhealthy slot is simulated by simply never
#                  resetting the counter — README "UNHEALTHY" branch).
#
# Mechanism (deterministic, SINGLE in-RAM U-Boot session — no persistent env):
#   interrupt U-Boot autoboot -> `setenv bootcount 2` + `setenv bootlimit 1`
#   + `setenv upgrade_available 1` + `setenv BOOT_ORDER "B A"`
#   -> `load virtio 0:1 <addr> boot.scr` -> `source <addr>` (runs boot.cmd:
#   step-2 guard sees bootcount>bootlimit, swaps "B A"->"A B", selects head A,
#   booti's the kernel with root=/dev/vda2 helix_slot=A). See uboot_ab/boot.cmd.
#
# Driver robustness (§11.4.1 — the FAIL must be a product defect, never a script
#   bug): the expect driver is emitted to a TEMP .exp FILE via a single-quoted
#   heredoc, so there is exactly ONE quoting level (Tcl). Guest-shell command
#   substitution is written `\$(...)` in Tcl => the guest shell (not Tcl) runs it.
#   Login tolerates interleaved kernel-console noise + retries the login cycle.
#   Mirrors ab_slot_switch.sh (PWU-AB-1, GREEN) structure exactly.
#
# Usage:  tests/emulator/ab_virt/ab_rollback.sh
#   Pre:  out/.ok + out/.disk_ok + out/images/{u-boot.bin,ab_disk.img}.
#   Env:  HELIX_AB_ROOT_PW (default helixota — must match build_image.sh).
# Outputs: evidence under docs/qa/<run-id>-ab-rollback/{consoleROLLBACK.log,
#          consoleCONTROL.log,verdict.txt}; PASS/FAIL verdict.
# Deps: qemu-system-aarch64 (HVF), expect. Self-cleaning: each QEMU exits on guest
#   poweroff or the expect timeout (§11.4.14).
# Cross-refs: uboot_ab/README.md ; uboot_ab/boot.cmd (lines 53-69 guard) ;
#   docs/research/rk3588_emulator/REPORT.md ; ab_slot_switch.sh (PWU-AB-1) ;
#   §11.4.115 (RED->GREEN polarity) ; §11.4.107 (live not frozen) ;
#   §11.4.83 (evidence) ; §11.4.6 (assert, never assume; honest UNVERIFIED) ;
#   §11.4.1 (script bugs fixed at source) ;
#   §11.4.111 (the disk is the only virtio-blk so devnum 0 is pinned) ;
#   §11.4.133 (target-safety — read-only-to-host: no host disk/env touched).
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
ROOT_PW="${HELIX_AB_ROOT_PW:-helixota}"
IMG_DIR="${SCRIPT_DIR}/out/images"
UBOOT="${IMG_DIR}/u-boot.bin"
DISK="${IMG_DIR}/ab_disk.img"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-ab-rollback"
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

log "== PWU-AB-3 corrupt-slot AUTO-ROLLBACK proof (real U-Boot + QEMU virt + HVF) =="
log "run=${RUN_ID}  u-boot=$(du -h "$UBOOT"|cut -f1)  disk=$(du -h "$DISK"|cut -f1)"

# ---- emit the expect driver to a TEMP FILE (single Tcl quoting level) --------
# SINGLE-QUOTED heredoc: bash does NOT expand, so `\$(` reaches the file verbatim
# and Tcl turns `\$` into a literal `$` => the GUEST shell runs the substitution.
# argv: 0=pw 1=order 2=bootcount 3=bootlimit 4=console 5=uboot 6=disk 7=mark
cat > "$EXP" <<'EXPEOF'
set timeout 150
set pw        [lindex $argv 0]
set order     [lindex $argv 1]
set bootcount [lindex $argv 2]
set bootlimit [lindex $argv 3]
set console   [lindex $argv 4]
set uboot     [lindex $argv 5]
set disk      [lindex $argv 6]
set mark      [lindex $argv 7]
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
# Arm the A/B env exactly as a slot on update-probation that has consumed its
# attempts: head = the (bad) updated slot, upgrade_available=1 (on probation),
# bootcount/bootlimit set so the guard decides whether the slot rolls back.
send "setenv BOOT_ORDER $order\r"
expect -re {=> $}
send "setenv upgrade_available 1\r"
expect -re {=> $}
send "setenv bootlimit $bootlimit\r"
expect -re {=> $}
send "setenv bootcount $bootcount\r"
expect -re {=> $}
send "printenv BOOT_ORDER bootcount bootlimit upgrade_available\r"
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

drive_boot() {  # $1=order  $2=bootcount  $3=bootlimit  $4=console  $5=mark
  expect -f "$EXP" "$ROOT_PW" "$1" "$2" "$3" "$4" "$UBOOT" "$DISK" "$5" >> "${4}.driver" 2>&1
  return $?
}

log ""
log "-- Run ROLLBACK: BOOT_ORDER='B A' bootcount=2 bootlimit=1 (PAST limit ->"
log "   guard swaps to 'A B' -> expect boot /dev/vda2, slot_id=A = AUTO-ROLLBACK) --"
drive_boot "B A" "2" "1" "${EVID}/consoleROLLBACK.log" "ROLLBACK"; rcR=$?
log "-- Run CONTROL: BOOT_ORDER='B A' bootcount=1 bootlimit=1 (NOT past limit ->"
log "   guard does NOT fire -> expect boot /dev/vda3, slot_id=B = no rollback) --"
drive_boot "B A" "1" "1" "${EVID}/consoleCONTROL.log" "CONTROL"; rcC=$?

# ---- anti-bluff assertions (the proof is the CONTRAST) ----------------------
fail=0
chk() { if grep -aqE "$2" "$1"; then log "[PASS] $3"; else log "[FAIL] $3"; fail=1; fi; }
nchk(){ if grep -aqE "$2" "$1"; then log "[FAIL] $3"; fail=1; else log "[PASS] $3"; fi; }

# Run ROLLBACK: the guard fired (console line), BOOT_ORDER swapped to "A B",
# the guest BOOTED the previous-good slot A (slot_id=A, root vda2, cmdline A),
# the interactive shell is live, and it did NOT keep booting the bad slot B.
chk  "${EVID}/consoleROLLBACK.log" 'rolling back \(altbootcmd swap\)' "Run ROLLBACK: U-Boot guard emitted the altbootcmd-swap rollback line"
chk  "${EVID}/consoleROLLBACK.log" "BOOT_ORDER='A B'"                 "Run ROLLBACK: guard swapped BOOT_ORDER 'B A' -> 'A B' (head now A)"
chk  "${EVID}/consoleROLLBACK.log" 'HELIX_SLOTID=A'                   "Run ROLLBACK: guest userspace reports /etc/slot_id=A (ROLLED BACK to good slot)"
chk  "${EVID}/consoleROLLBACK.log" 'HELIX_ROOTDEV=/dev/vda2'          "Run ROLLBACK: guest findmnt confirms root = /dev/vda2 (slot A)"
chk  "${EVID}/consoleROLLBACK.log" 'helix_slot=A'                     "Run ROLLBACK: kernel cmdline carries helix_slot=A (boot.scr selected A after swap)"
chk  "${EVID}/consoleROLLBACK.log" 'HELIX_DONE_ROLLBACK_MARK'         "Run ROLLBACK: interactive shell live (post-login sentinel, not a frozen frame)"
nchk "${EVID}/consoleROLLBACK.log" 'HELIX_SLOTID=B'                   "Run ROLLBACK: did NOT keep booting the bad slot B (rollback is real, not a no-op)"
# Run CONTROL: same order but bootcount NOT past limit => NO rollback. Proves
# the rollback was triggered by the exhausted counter, not by anything else.
nchk "${EVID}/consoleCONTROL.log" 'rolling back \(altbootcmd swap\)'  "Run CONTROL: guard did NOT fire (bootcount not past bootlimit)"
chk  "${EVID}/consoleCONTROL.log" 'HELIX_SLOTID=B'                    "Run CONTROL: guest booted head slot B (slot_id=B) — no rollback when counter not exhausted"
chk  "${EVID}/consoleCONTROL.log" 'HELIX_ROOTDEV=/dev/vda3'           "Run CONTROL: guest findmnt confirms root = /dev/vda3 (slot B)"
chk  "${EVID}/consoleCONTROL.log" 'HELIX_DONE_CONTROL_MARK'           "Run CONTROL: interactive shell live (post-login sentinel, not a frozen frame)"
nchk "${EVID}/consoleCONTROL.log" 'HELIX_SLOTID=A'                    "Run CONTROL: did NOT roll back to A (metamorphic: only the past-limit run rolls back)"

{
  echo "PWU-AB-3 corrupt-slot AUTO-ROLLBACK — run ${RUN_ID}"
  echo "u-boot.bin: $(strings "$UBOOT" | grep -m1 -iE '^U-Boot 20')"
  echo "Run ROLLBACK rc=${rcR}  Run CONTROL rc=${rcC}"
  echo "Run ROLLBACK (head was bad slot B, expect rolled-back to A): $(grep -aoE 'HELIX_SLOTID=[AB]' "${EVID}/consoleROLLBACK.log" | head -1)  $(grep -aoE 'HELIX_ROOTDEV=/dev/vda[0-9]' "${EVID}/consoleROLLBACK.log" | head -1)"
  echo "Run CONTROL  (head slot B, expect no rollback -> B):         $(grep -aoE 'HELIX_SLOTID=[AB]' "${EVID}/consoleCONTROL.log" | head -1)  $(grep -aoE 'HELIX_ROOTDEV=/dev/vda[0-9]' "${EVID}/consoleCONTROL.log" | head -1)"
  echo "Verdict: $([ "$fail" -eq 0 ] && echo PASS || echo FAIL)"
} > "${EVID}/verdict.txt"

log ""
cat "${EVID}/verdict.txt"
log "EVIDENCE: ${EVID}/ (consoleROLLBACK.log $(wc -l < "${EVID}/consoleROLLBACK.log" 2>/dev/null|tr -d ' ') lines, consoleCONTROL.log $(wc -l < "${EVID}/consoleCONTROL.log" 2>/dev/null|tr -d ' ') lines)"
if [ "$fail" -eq 0 ]; then
  log "RESULT: PASS — real bootloader-driven A/B AUTO-ROLLBACK proven (bad slot B past bootlimit -> guard swap -> boots previous-good slot A=vda2) under U-Boot+QEMU+HVF."
  exit 0
fi
log "RESULT: FAIL — see ${EVID}/ (qemu/expect rcR=${rcR} rcC=${rcC})"
exit 1
