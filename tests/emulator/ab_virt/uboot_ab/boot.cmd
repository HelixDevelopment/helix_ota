# =============================================================================
# boot.cmd — RK3588 A/B-virt emulator U-Boot A/B boot script (PWU-AB-1)
# -----------------------------------------------------------------------------
# Source for boot.scr (compiled by assemble_ab_disk.sh via:
#   mkimage -A arm64 -O linux -T script -C none -d boot.cmd boot.scr
# and placed on the FAT boot partition (GPT p1) as /boot.scr; the qemu_arm64
# U-Boot default bootcmd loads + sources it).
#
# Implements 2-slot (A/B) selection + bootcount auto-rollback per:
#   - U-Boot Boot Count Limit  https://docs.u-boot.org/en/latest/api/bootcount.html
#     (bootcount / bootlimit / altbootcmd / upgrade_available semantics)
#   - Mender/RAUC U-Boot integration (BOOT_ORDER convention)
#     https://docs.mender.io/operating-system-updates-yocto-project/board-integration/bootloader-support/u-boot/manual-u-boot-integration
#
# State machine (full description in uboot_ab/README.md):
#   * BOOT_ORDER        — space-separated slot list; HEAD = slot to try first
#                         ("A B" => try A then B). RAUC/Mender convention.
#   * bootcount         — incremented by U-Boot CONFIG_BOOTCOUNT_LIMIT each reboot
#                         while upgrade_available != 0 (U-Boot increments it
#                         BEFORE this script runs; here we only READ it).
#   * bootlimit         — when bootcount > bootlimit, U-Boot runs altbootcmd
#                         instead of bootcmd. This script ALSO swaps defensively
#                         so a single boot.scr both selects the head slot AND
#                         demotes a slot that has consumed its attempts.
#   * upgrade_available — 1 while an update is "on probation" (bootcount saved +
#                         incremented). A healthy-boot userspace marker sets it
#                         to 0 and resets bootcount to 0 (done IN-GUEST, not here
#                         — see README), freezing the counter = slot confirmed.
#
# Bootloader-env-driven by design (no compiled-in slot): the OTA agent flips
# BOOT_ORDER + arms upgrade_available/bootcount in the U-Boot env exactly as a
# real RK3588/embedded U-Boot A/B target does.
#
# GPT layout this script REQUIRES (MUST match assemble_ab_disk.sh exactly):
#   p1  FAT     boot       — this boot.scr + the kernel Image   (load src: virtio 0:1)
#   p2  ext*    rootfs_a   — root for slot A                     (root=/dev/vda2)
#   p3  ext*    rootfs_b   — root for slot B                     (root=/dev/vda3)
# Under QEMU `-machine virt` the single A/B virtio-blk disk enumerates on iface
# "virtio" devnum 0; the kernel partition is p1, slot roots are /dev/vda2 (A) and
# /dev/vda3 (B).
# =============================================================================

# ---- defaults (only set if the env did not already provide them) ------------
# A fresh disk ships these as the uboot.env default text (see uboot_ab/uboot.env).
test -n "${BOOT_ORDER}"        || setenv BOOT_ORDER "A B"
test -n "${bootlimit}"         || setenv bootlimit 1
test -n "${bootcount}"         || setenv bootcount 1
test -n "${upgrade_available}" || setenv upgrade_available 0

setenv console_args "console=ttyAMA0"
setenv extra_args   "rootwait rw"

# ---- bootlimit / altbootcmd guard -------------------------------------------
# U-Boot's CONFIG_BOOTCOUNT_LIMIT runs altbootcmd (not bootcmd) once
# bootcount > bootlimit. To keep the rollback self-contained in this single
# boot.scr, this script ALSO checks here and, if the head slot has exhausted its
# attempts, swaps BOOT_ORDER so the OTHER slot becomes the head — the "swap the
# rootfs partition" altbootcmd action from the Mender/U-Boot integration doc.
if test ${bootcount} -gt ${bootlimit}; then
  echo "A/B: bootcount=${bootcount} > bootlimit=${bootlimit} -> rolling back (altbootcmd swap)"
  if test "${BOOT_ORDER}" = "A B"; then
    setenv BOOT_ORDER "B A"
  else
    setenv BOOT_ORDER "A B"
  fi
  # Re-arm exactly one attempt on the now-head (previously-good) slot.
  setenv bootcount 1
  saveenv
fi

# ---- select the head slot of BOOT_ORDER -------------------------------------
# Extract the FIRST whitespace-separated token of BOOT_ORDER as the active slot.
# `setexpr <name> sub <regex> <replacement> <source>` is U-Boot's in-place
# regex substitution; here it strips " <rest>" leaving just the head token.
setexpr active_slot sub "[ ].*$" "" "${BOOT_ORDER}"
test -n "${active_slot}" || setenv active_slot "A"

# Map the slot letter to its rootfs partition number on the A/B disk.
if test "${active_slot}" = "A"; then
  setenv root_part 2
else
  setenv root_part 3
fi
echo "A/B: BOOT_ORDER='${BOOT_ORDER}' active_slot=${active_slot} root=/dev/vda${root_part} bootcount=${bootcount} upgrade_available=${upgrade_available}"

# ---- load the kernel from the FAT boot partition (p1) -----------------------
# load <iface> <devnum>:<part> <addr> <file>
setenv kernel_addr 0x40400000
if load virtio 0:1 ${kernel_addr} Image; then
  echo "A/B: loaded kernel Image from boot partition (virtio 0:1)"
else
  echo "A/B: ERROR kernel load failed from boot partition (virtio 0:1)"
fi

# ---- assemble bootargs + boot -----------------------------------------------
setenv bootargs "${console_args} root=/dev/vda${root_part} ${extra_args} helix_slot=${active_slot}"
echo "A/B: booting slot ${active_slot} with bootargs: ${bootargs}"
booti ${kernel_addr} - ${fdtcontroladdr}
