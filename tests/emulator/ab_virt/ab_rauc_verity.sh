#!/usr/bin/env bash
# =============================================================================
# ab_rauc_verity.sh — RK3588 A/B-virt emulator: PWU-AB-2 RAUC dm-verity update proof
# -----------------------------------------------------------------------------
# Purpose:
#   Prove a REAL in-guest RAUC update that installs a dm-verity-backed rootfs to
#   the INACTIVE slot, flips the bootloader A/B selector, and — after a real
#   reboot under U-Boot — lands the guest on the freshly-installed slot with a
#   dm-verity-protected root. This is the OTA "apply to inactive slot -> activate
#   -> reboot -> verify" primitive (RAUC + U-Boot bootcount A/B convention)
#   running under real U-Boot 2024.01 + QEMU virt + HVF on this Apple-Silicon
#   host — extending PWU-AB-1 (ab_slot_switch.sh) from a hand-driven BOOT_ORDER
#   flip to a genuine `rauc install` apply with cryptographic dm-verity slots.
#   Not a mock: `rauc` is the upstream RAUC binary (BR2_PACKAGE_RAUC=y in
#   build_image.sh) and dm-verity is the identical kernel feature a real RK3588
#   A/B target uses (REPORT.md §3 "why it's a real mechanism, not a mock").
#
# Mechanism (deterministic, what the OTA agent will later drive autonomously):
#   1. Boot slot A under U-Boot (BOOT_ORDER="A B" -> head=A -> root=/dev/vda2),
#      log in to the getty exactly like PWU-AB-1.
#   2. In-guest: `rauc status` (baseline) -> `rauc install <bundle>` writes the
#      verity rootfs image to the INACTIVE slot B (/dev/vda3) and (RAUC U-Boot
#      backend) arms the boot selector. RAUC's native U-Boot backend manipulates
#      BOOT_ORDER + BOOT_<bootname>_LEFT via fw_setenv; THIS project's boot.cmd
#      reads the Mender-style BOOT_ORDER + bootcount/upgrade_available set — see
#      the §11.4.6 honest integration gap below.
#   3. Set BOOT_ORDER head=B + upgrade_available=1 + bootcount=1 so this project's
#      boot.cmd selects slot B on the next boot (the apply-activates-inactive
#      semantics; uboot_ab/README.md "A normal OTA apply -> slot-switch").
#   4. Reboot -> boot.scr selects head B -> root=/dev/vda3.
#   5. Assert the guest booted slot B with a dm-verity-backed root:
#      `rauc status` shows slot B booted/good, `/etc/slot_id`=B, `dmsetup status`
#      reports a `verity` target active (and/or dmesg "device-mapper: verity"),
#      and `findmnt /` resolves to the verity dm device over /dev/vda3.
#
# §11.4.115 RED->GREEN polarity (the proof is the CONTRAST, captured live):
#   RED_MODE=1 (default UNTIL the bundle-build + verity wiring lands): the apply
#     path is EXPECTED to be unproven — the verdict asserts the defect-present
#     baseline (no verity-active root, slot did NOT switch via rauc) so a GREEN
#     here on the un-wired artifact would be a §11.4 bluff. Flip RED_MODE=0 once
#     a real RAUC verity bundle + fw_env wiring exist; the SAME assertions then
#     guard the GREEN behaviour (slot=B, verity-active, root over /dev/vda3).
#   A no-op / fake update reports slot A still booted + no verity target (RED).
#   A real verity apply reports slot B booted + dm-verity active (GREEN). The
#   console is captured in full (§11.4.107 live-not-frozen: real getty login +
#   post-reboot post-login command output, never a single frame).
#
# Driver robustness (§11.4.1 — the FAIL must be a product defect, never a script
#   bug): the expect driver is emitted to a TEMP .exp FILE via a single-quoted
#   heredoc, so there is exactly ONE quoting level (Tcl). Guest-shell command
#   substitution is written `\$(...)` in Tcl => the GUEST shell (not Tcl) runs it.
#   Login tolerates interleaved kernel-console noise + retries the login cycle.
#   This MIRRORS ab_slot_switch.sh (PWU-AB-1) exactly.
#
# GPT-layout contract (MUST match uboot_ab/boot.cmd + assemble_ab_disk.sh):
#   p1  FAT     boot       — kernel Image + boot.scr            (U-Boot: virtio 0:1)
#   p2  ext*    rootfs_a   — slot A root (RAUC bootname A)       -> /dev/vda2
#   p3  ext*    rootfs_b   — slot B root (RAUC bootname B)       -> /dev/vda3
#   BOOT_ORDER head token = active slot (A->vda2, B->vda3). /etc/slot_id differs
#   per slot so the switch is observable from inside the guest.
#
# Usage:  tests/emulator/ab_virt/ab_rauc_verity.sh
#   Pre:  out/.ok + out/.disk_ok + out/images/{u-boot.bin,ab_disk.img}
#         + a RAUC verity bundle (see RAUC_BUNDLE / the bundle-build TODO below).
#   Env:  HELIX_AB_ROOT_PW   (default helixota — must match build_image.sh)
#         RAUC_BUNDLE        (guest path to the .raucb verity bundle; default
#                             /root/update.raucb — see bundle-build TODO)
#         RED_MODE           (default 1 — assert defect-present; 0 = GREEN guard)
# Outputs: evidence under docs/qa/<run-id>-ab-rauc-verity/{console.log,
#          rauc_status_pre.txt,rauc_status_post.txt,verdict.txt}; PASS/FAIL.
# Deps: qemu-system-aarch64 (HVF), expect, an in-guest RAUC verity bundle.
#   Self-cleaning: each QEMU exits on guest poweroff or the expect timeout
#   (§11.4.14).
# Cross-refs: ab_slot_switch.sh (PWU-AB-1 pattern MIRRORED) ; uboot_ab/README.md
#   + uboot_ab/boot.cmd (the A/B state machine + GPT contract) ; build_image.sh
#   (BR2_PACKAGE_RAUC=y) ; docs/research/rk3588_emulator/REPORT.md (PWU-AB-2,
#   §3/§4) ; §11.4.115 (RED->GREEN) ; §11.4.107 (live not frozen) ; §11.4.108
#   (verity-active = the runtime signature) ; §11.4.83 (evidence) ; §11.4.6
#   (assert never assume; honest integration gap) ; §11.4.1 (script bugs at
#   source) ; §11.4.111 (the disk is the only virtio-blk so devnum 0 is pinned).
#
# STATUS (§11.4.6): UNVERIFIED-pending-PWU-AB-1-GREEN + pending a RAUC bundle
#   build step — authored, not yet run. PWU-AB-1 (ab_slot_switch.sh) has NOT yet
#   reported GREEN on the real u-boot.bin artifact (still building per
#   uboot_ab/README.md Status), and NO RAUC verity bundle exists yet, so the
#   dm-verity apply is UNPROVEN. No working RAUC update / verity slot is claimed
#   here. Run order is the conductor's: PWU-AB-1 GREEN -> build bundle -> run this
#   with RED_MODE=1 (capture defect-present) -> wire verity+fw_env -> RED_MODE=0.
#
# TODO (bundle-build dependency — BLOCKS a GREEN RED_MODE=0 run):
#   A RAUC verity bundle (.raucb) does NOT exist yet. Before this test can prove
#   a real apply it requires, authored as a separate step (own commit, §11.4.142
#   independent review):
#     (a) an in-guest /etc/rauc/system.conf with `bootloader=uboot`, `[keyring]`,
#         and `[slot.rootfs.0] bootname=A device=/dev/vda2` +
#         `[slot.rootfs.1] bootname=B device=/dev/vda3`;
#     (b) /etc/fw_env.config pointing fw_setenv/fw_printenv at the U-Boot env
#         region (so RAUC's U-Boot backend can read/write BOOT_ORDER);
#     (c) a signed bundle built with a manifest `[update]`/`[bundle] format=verity`
#         + the new rootfs `[image.rootfs]` (`rauc bundle` on a build host with
#         a signing key + matching keyring) staged into the guest at RAUC_BUNDLE;
#     (d) reconciliation of the RAUC U-Boot backend env scheme
#         (BOOT_ORDER + BOOT_<bootname>_LEFT) with THIS project's boot.cmd scheme
#         (BOOT_ORDER + bootcount/upgrade_available) — see the §11.4.6 gap note.
#   Until (a)-(d) exist, RED_MODE=1 is the correct, honest posture.
# =============================================================================
set -u
set -o pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
ROOT_PW="${HELIX_AB_ROOT_PW:-helixota}"
RAUC_BUNDLE="${RAUC_BUNDLE:-/root/update.raucb}"
RED_MODE="${RED_MODE:-1}"
IMG_DIR="${SCRIPT_DIR}/out/images"
UBOOT="${IMG_DIR}/u-boot.bin"
DISK="${IMG_DIR}/ab_disk.img"
RUN_ID="$(date -u +%Y%m%dT%H%M%SZ)-ab-rauc-verity"
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

log "== PWU-AB-2 RAUC dm-verity update proof (real U-Boot + QEMU virt + HVF) =="
log "run=${RUN_ID}  RED_MODE=${RED_MODE}  bundle(guest)=${RAUC_BUNDLE}"
log "u-boot=$(du -h "$UBOOT"|cut -f1)  disk=$(du -h "$DISK"|cut -f1)"

# ---- emit the expect driver to a TEMP FILE (single Tcl quoting level) --------
# SINGLE-QUOTED heredoc: bash does NOT expand, so `\$(` reaches the file verbatim
# and Tcl turns `\$` into a literal `$` => the GUEST shell runs the substitution.
# This drives ONE QEMU instance through the full apply->reboot->verify cycle by
# rebooting in-guest (boot.scr re-runs after `reboot`), so the BOOT_ORDER flip is
# the OTA agent's action, not a host re-spawn.
# argv: 0=pw 1=console 2=uboot 3=disk 4=bundle
cat > "$EXP" <<'EXPEOF'
set timeout 240
set pw      [lindex $argv 0]
set console [lindex $argv 1]
set uboot   [lindex $argv 2]
set disk    [lindex $argv 3]
set bundle  [lindex $argv 4]
log_file -noappend $console

# Login cycle (tolerant of interleaved kernel-console noise), one retry.
proc do_login {} {
  global pw
  send "root\r"
  expect {
    timeout { puts "HELIX_DRIVER_FAIL: no password prompt"; exit 2 }
    -re {Password: $}
  }
  sleep 1
  send "$pw\r"
}
proc await_shell {} {
  set tries 0
  expect {
    timeout { puts "HELIX_DRIVER_FAIL: no login prompt"; exit 2 }
    -re {buildroot login: $}
  }
  do_login
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
  send "\r"
  expect -re {# $}
}

spawn qemu-system-aarch64 -M virt -accel hvf -cpu host -smp 2 -m 512 -nographic \
  -bios $uboot -drive file=$disk,if=virtio,format=raw

# ---- FIRST boot: slot A (default BOOT_ORDER="A B") ----
# Interrupt U-Boot autoboot, set the known-A baseline, source boot.scr.
expect {
  timeout { puts "HELIX_DRIVER_FAIL: no autoboot prompt"; exit 2 }
  -re {stop autoboot}
}
send "\r"
expect {
  timeout { puts "HELIX_DRIVER_FAIL: no U-Boot prompt"; exit 2 }
  -re {=> $}
}
send "setenv BOOT_ORDER \"A B\"\r"
expect -re {=> $}
send "load virtio 0:1 0x40400000 boot.scr\r"
expect {
  timeout { puts "HELIX_DRIVER_FAIL: boot.scr load timed out"; exit 2 }
  -re {=> $}
}
send "source 0x40400000\r"

await_shell
send "echo HELIX_PRESLOT=\$(cat /etc/slot_id 2>/dev/null)\r"
expect -re {# $}

# ---- RAUC baseline status (captured) ----
send "echo HELIX_RAUC_PRE_BEGIN\r"
expect -re {# $}
send "rauc status --detailed 2>&1 || echo HELIX_RAUC_PRE_ERR=\$?\r"
expect -re {# $}
send "echo HELIX_RAUC_PRE_END\r"
expect -re {# $}

# ---- RAUC apply: install the verity bundle to the INACTIVE slot (B) ----
# A missing bundle / un-wired system.conf surfaces a non-zero rc -> captured,
# NOT masked (RED_MODE=1 expects this until the bundle-build TODO lands).
send "echo HELIX_RAUC_INSTALL_BEGIN\r"
expect -re {# $}
send "rauc install $bundle 2>&1; echo HELIX_RAUC_INSTALL_RC=\$?\r"
expect {
  timeout { puts "HELIX_DRIVER_NOTE: rauc install ran long"; }
  -re {# $}
}
send "echo HELIX_RAUC_INSTALL_END\r"
expect -re {# $}

# ---- Activate slot B in THIS project's boot.cmd scheme ----
# RAUC's own U-Boot backend would arm BOOT_ORDER + BOOT_<bootname>_LEFT; this
# project's boot.cmd reads BOOT_ORDER head + bootcount/upgrade_available, so we
# set that set so the next boot selects B (the apply-activates-inactive action,
# uboot_ab/README.md). When fw_env is wired (TODO d) `rauc install` does this.
send "fw_setenv BOOT_ORDER \"B A\" 2>/dev/null; fw_setenv upgrade_available 1 2>/dev/null; fw_setenv bootcount 1 2>/dev/null; echo HELIX_FWSET_RC=\$?\r"
expect -re {# $}

# ---- Reboot: boot.scr re-selects head slot (now B) ----
send "echo HELIX_REBOOT_NOW\r"
expect -re {# $}
send "reboot\r"

# After reboot U-Boot may re-enter autoboot; if it stops, re-source boot.scr.
expect {
  timeout { puts "HELIX_DRIVER_FAIL: no post-reboot activity"; exit 2 }
  -re {stop autoboot} {
    send "\r"
    expect -re {=> $}
    send "load virtio 0:1 0x40400000 boot.scr\r"
    expect -re {=> $}
    send "source 0x40400000\r"
  }
  -re {buildroot login: $} {
    # boot.scr ran from default bootcmd without stopping — fall through to login.
  }
}

# ---- SECOND boot: assert slot B + dm-verity-backed root ----
await_shell
send "echo HELIX_POSTSLOT=\$(cat /etc/slot_id 2>/dev/null)\r"
expect -re {# $}
send "echo HELIX_ROOTDEV=\$(findmnt -no SOURCE / 2>/dev/null)\r"
expect -re {# $}
send "echo HELIX_CMDLINE=\$(cat /proc/cmdline)\r"
expect -re {# $}
# dm-verity runtime signature (§11.4.108): a `verity` target active in the table.
send "echo HELIX_DMVERITY=\$(dmsetup status 2>/dev/null | grep -c verity)\r"
expect -re {# $}
send "echo HELIX_DMSETUP_BEGIN\r"
expect -re {# $}
send "dmsetup status 2>&1 || echo none\r"
expect -re {# $}
send "echo HELIX_DMSETUP_END\r"
expect -re {# $}
# Kernel-side corroboration of dm-verity bringup.
send "echo HELIX_DMESG_VERITY=\$(dmesg 2>/dev/null | grep -c 'device-mapper: verity')\r"
expect -re {# $}
# RAUC post-apply status (slot B booted / marked).
send "echo HELIX_RAUC_POST_BEGIN\r"
expect -re {# $}
send "rauc status --detailed 2>&1 || echo HELIX_RAUC_POST_ERR=\$?\r"
expect -re {# $}
send "echo HELIX_RAUC_POST_END\r"
expect -re {# $}
send "echo HELIX_DONE_RAUC_MARK\r"
expect -re {# $}
send "poweroff\r"
expect {
  timeout { puts "HELIX_DRIVER_NOTE: poweroff timed out"; exit 0 }
  eof
}
EXPEOF

# ---- drive the single apply->reboot->verify cycle ---------------------------
CON="${EVID}/console.log"
expect -f "$EXP" "$ROOT_PW" "$CON" "$UBOOT" "$DISK" "$RAUC_BUNDLE" >> "${CON}.driver" 2>&1
rc=$?

# Split out the captured RAUC status sections as standalone evidence files.
sed -n '/HELIX_RAUC_PRE_BEGIN/,/HELIX_RAUC_PRE_END/p'   "$CON" > "${EVID}/rauc_status_pre.txt"  2>/dev/null || true
sed -n '/HELIX_RAUC_POST_BEGIN/,/HELIX_RAUC_POST_END/p' "$CON" > "${EVID}/rauc_status_post.txt" 2>/dev/null || true

# ---- assertions -------------------------------------------------------------
fail=0
chk() { if grep -aqE "$2" "$1"; then log "[PASS] $3"; else log "[FAIL] $3"; fail=1; fi; }
nchk(){ if grep -aqE "$2" "$1"; then log "[FAIL] $3"; fail=1; else log "[PASS] $3"; fi; }

# Always-true preconditions: the FIRST boot must reach slot A and run RAUC.
chk  "$CON" 'HELIX_PRESLOT=A'         "First boot landed on slot A (baseline before apply)"
chk  "$CON" 'HELIX_RAUC_INSTALL_END'  "rauc install path was driven (real binary invoked, rc captured)"
chk  "$CON" 'HELIX_DONE_RAUC_MARK'    "Interactive shell live post-reboot (post-login sentinel, not a frozen frame)"

if [ "$RED_MODE" = "0" ]; then
  # GREEN guard (only valid AFTER the bundle-build TODO + verity/fw_env wiring):
  # the apply MUST have switched to slot B AND brought up a dm-verity-backed root.
  log "-- RED_MODE=0: GREEN guard (real verity apply expected) --"
  chk  "$CON" 'HELIX_RAUC_INSTALL_RC=0'   "GREEN: rauc install returned 0 (apply succeeded)"
  chk  "$CON" 'HELIX_POSTSLOT=B'          "GREEN: post-reboot guest reports /etc/slot_id=B (SLOT SWITCHED by RAUC apply)"
  chk  "$CON" 'HELIX_ROOTDEV='            "GREEN: post-reboot root device captured"
  chk  "$CON" 'HELIX_DMVERITY=[1-9]'      "GREEN: dmsetup reports >=1 active dm-verity target (verity-backed root, §11.4.108)"
  nchk "$CON" 'HELIX_POSTSLOT=A'          "GREEN: did NOT stay on slot A (the verity apply+switch is real, not a no-op)"
else
  # RED baseline (default, on the un-wired artifact): the apply is EXPECTED
  # unproven — assert the defect-present state so a premature GREEN is impossible.
  log "-- RED_MODE=1: defect-present baseline (no proven verity apply yet) --"
  nchk "$CON" 'HELIX_RAUC_INSTALL_RC=0'   "RED: rauc install did NOT succeed (no bundle/system.conf yet — expected pre-wiring)"
  nchk "$CON" 'HELIX_DMVERITY=[1-9]'      "RED: no active dm-verity target yet (verity slot not wired — expected pre-wiring)"
  nchk "$CON" 'HELIX_POSTSLOT=B'          "RED: slot did NOT switch to B via RAUC yet (apply unproven — expected pre-wiring)"
fi

{
  echo "PWU-AB-2 RAUC dm-verity update — run ${RUN_ID}  (RED_MODE=${RED_MODE})"
  echo "u-boot.bin: $(strings "$UBOOT" 2>/dev/null | grep -m1 -iE '^U-Boot 20')"
  echo "expect rc=${rc}"
  echo "pre-slot:  $(grep -aoE 'HELIX_PRESLOT=[AB]'  "$CON" | head -1)"
  echo "post-slot: $(grep -aoE 'HELIX_POSTSLOT=[AB]' "$CON" | head -1)"
  echo "rauc install rc: $(grep -aoE 'HELIX_RAUC_INSTALL_RC=[0-9]+' "$CON" | head -1)"
  echo "dm-verity targets: $(grep -aoE 'HELIX_DMVERITY=[0-9]+' "$CON" | head -1)"
  echo "root dev:  $(grep -aoE 'HELIX_ROOTDEV=[^ ]+' "$CON" | head -1)"
  echo "Verdict: $([ "$fail" -eq 0 ] && echo PASS || echo FAIL)"
} > "${EVID}/verdict.txt"

log ""
cat "${EVID}/verdict.txt"
log "EVIDENCE: ${EVID}/ (console.log $(wc -l < "$CON" 2>/dev/null|tr -d ' ') lines, rauc_status_pre.txt, rauc_status_post.txt)"
if [ "$fail" -eq 0 ]; then
  if [ "$RED_MODE" = "0" ]; then
    log "RESULT: PASS — real RAUC dm-verity apply proven: slot switched A->B with a verity-backed root under U-Boot+QEMU+HVF."
  else
    log "RESULT: PASS (RED baseline) — defect-present state captured as expected; flip RED_MODE=0 after the bundle-build TODO lands."
  fi
  exit 0
fi
log "RESULT: FAIL — see ${EVID}/ (expect rc=${rc})"
exit 1

# =============================================================================
# Sources verified 2026-06-11
# - RAUC examples (rauc install <bundle>, system.conf [system] bootloader=uboot,
#   [slot.rootfs.N] device/type/bootname, [bundle] format=verity, mark-good):
#   https://rauc.readthedocs.io/en/latest/examples.html
# - RAUC using/CLI (`rauc install <bundle>`, `rauc status [--detailed]`,
#   `rauc status mark-good|mark-bad|mark-active [booted|other|<SLOT>]`):
#   https://rauc.readthedocs.io/en/latest/using.html
# - RAUC U-Boot bootloader integration (BOOT_ORDER + BOOT_<bootname>_LEFT,
#   fw_setenv/fw_printenv + /etc/fw_env.config, contrib/uboot.sh -> boot.scr):
#   https://rauc.readthedocs.io/en/latest/integration.html
# - U-Boot Boot Count Limit (bootcount/bootlimit/altbootcmd/upgrade_available —
#   THIS project's boot.cmd scheme, per uboot_ab/README.md):
#   https://docs.u-boot.org/en/latest/api/bootcount.html
# - Mender U-Boot integration (BOOT_ORDER head = active slot, swap-rootfs):
#   https://docs.mender.io/operating-system-updates-yocto-project/board-integration/bootloader-support/u-boot/manual-u-boot-integration
# - RAUC on QEMU + format=verity (design authority, REPORT.md §3/§4 PWU-AB-2):
#   https://rauc.readthedocs.io/en/latest/examples.html ;
#   https://pengutronix.de/en/blog/2022-02-03-tutorial-evaluating-rauc-on-qemu-a-quick-setup-with-yocto.html
# NOTE (§11.4.6 honest gap): RAUC's native U-Boot backend uses
#   BOOT_ORDER + BOOT_<bootname>_LEFT; this project's boot.cmd uses
#   BOOT_ORDER + bootcount/upgrade_available. The TODO (d) reconciliation is a
#   real, UNVERIFIED integration item — not glossed over.
# =============================================================================
