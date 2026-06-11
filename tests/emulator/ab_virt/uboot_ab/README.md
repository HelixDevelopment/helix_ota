# U-Boot A/B boot script — RK3588 A/B-virt emulator

**Revision:** 1
**Last modified:** 2026-06-11T12:00:00Z

Authoritative design: `../../../../docs/research/rk3588_emulator/REPORT.md` (§3, §4,
PWU-AB-1). This directory holds the U-Boot side of the dev-host RK3588 / Orange
Pi 5 Max A/B emulator: a **real bootloader A/B slot-selector + bootcount
auto-rollback engine** — the same mechanism a real RK3588 / embedded U-Boot A/B
target uses (REPORT §3 "why it's a real mechanism, not a mock").

## Files

| File | Role |
|---|---|
| `boot.cmd` | U-Boot script SOURCE. Compiled to `boot.scr` by `../assemble_ab_disk.sh` via `mkimage` and placed on the FAT boot partition (GPT p1). The qemu_arm64 U-Boot default `bootcmd` loads + sources it. |
| `uboot.env` | Default U-Boot environment text (one `KEY=VALUE` per line). The version-controlled source-of-truth for the A/B env defaults; consumed by `mkenvimage` (binary env blob, later PWU) and mirrored by `boot.cmd`'s lazy-default guards (PWU-AB-1). |
| `README.md` | This document. |

## Compiling `boot.cmd` → `boot.scr`

`assemble_ab_disk.sh` runs (inside the aarch64 build container, where U-Boot
tools are available):

```sh
mkimage -A arm64 -O linux -T script -C none -d boot.cmd boot.scr
```

- `-A arm64` — target architecture.
- `-O linux` / `-T script` / `-C none` — a plain (uncompressed) U-Boot script image.
- `-d boot.cmd` — input source; `boot.scr` — output blob copied onto boot partition p1.

`mkimage` is the canonical U-Boot script compiler
(https://docs.u-boot.org/en/latest/usage/cmd/source.html — the `source` command
runs a script image produced by `mkimage -T script`).

## Environment variables + state machine

Variable names + semantics are exactly the cited U-Boot + Mender/RAUC set:

| Variable | Meaning | Source |
|---|---|---|
| `BOOT_ORDER` | Space-separated slot list; **HEAD** is the slot tried first (`"A B"` ⇒ try A then B). | Mender/RAUC convention. |
| `bootcount` | Attempt counter. U-Boot inits it to **1** on power-on and increments it each reboot **while `upgrade_available` is non-zero** — U-Boot does this BEFORE `boot.scr` runs; the script only READS it. | U-Boot bootcount doc. |
| `bootlimit` | Threshold. When `bootcount > bootlimit`, U-Boot runs `altbootcmd` instead of `bootcmd`. `1` ⇒ a freshly-updated slot gets exactly one try. | U-Boot bootcount doc. |
| `upgrade_available` | `1` = an update is on probation (counter armed + saved); `0` = confirmed-good / no update in flight (counter frozen). | Mender/RAUC convention (gates whether `bootcount` is saved). |
| `altbootcmd` | Alternate boot action run on `bootcount > bootlimit`. Defaults to `run bootcmd` so it re-sources `boot.scr`, whose in-script rollback guard swaps `BOOT_ORDER`. | U-Boot bootcount doc. |

### Slot-select → rollback state machine

```
power-on / reboot
      │  U-Boot: bootcount initialised to 1 (power-on) or, if upgrade_available!=0,
      │          incremented by 1 (reboot), saved BEFORE boot.scr runs
      ▼
 boot.scr (boot.cmd):
   1. apply lazy defaults (only if env empty)
   2. if bootcount > bootlimit  ──► ROLLBACK:
          swap BOOT_ORDER  ("A B"⇄"B A"),  bootcount=1,  saveenv
   3. active_slot = head token of BOOT_ORDER   (setexpr sub)
   4. root_part   = (A ⇒ 2,  B ⇒ 3)            ── matches the GPT layout
   5. load kernel Image from FAT boot part (virtio 0:1)
   6. booti with  root=/dev/vda<root_part>  helix_slot=<active_slot>
      ▼
 guest userspace boots on the active slot
      │
      ├─ HEALTHY  ► userspace marker (in-guest, NOT in U-Boot) sets
      │             upgrade_available=0 + bootcount=0  ⇒ slot CONFIRMED good,
      │             counter frozen, no further rollback.
      │
      └─ UNHEALTHY / fails to reach the marker ► next reboot increments bootcount;
                    once bootcount > bootlimit the step-2 guard swaps BOOT_ORDER
                    and the OTHER (previous-good) slot becomes head ⇒ AUTO-ROLLBACK.
```

The **healthy-boot reset is intentionally NOT in `boot.cmd`** — per the U-Boot
doc it is "the responsibility of some application code (typically a Linux
application) to reset `bootcount` to 0 when the system booted successfully". In
this emulator that marker runs in the guest userspace (a later PWU wires it +
the OTA agent that flips `BOOT_ORDER` / arms `upgrade_available`).

### A normal OTA apply → slot-switch (what the OTA agent will drive)

1. Agent writes the new rootfs to the **inactive** slot (B if A is active).
2. Agent sets `BOOT_ORDER="B A"`, `upgrade_available=1`, `bootcount=1`, `saveenv`.
3. Reboot → `boot.scr` selects head `B` → boots `/dev/vda3`.
4. Healthy: in-guest marker clears `upgrade_available`+`bootcount` ⇒ B confirmed.
5. Unhealthy: `bootcount` climbs past `bootlimit` ⇒ step-2 guard swaps back to
   `"A B"` ⇒ boots `/dev/vda2` (the previous-good A slot) = **auto-rollback**.

## GPT layout contract (MUST match `../assemble_ab_disk.sh`)

`boot.cmd` and `assemble_ab_disk.sh` share ONE fixed layout. Drift here breaks
the boot — it is the load-bearing coherence invariant:

| Part | Partlabel | FS | Role | `boot.cmd` reference |
|---|---|---|---|---|
| p1 | `boot` | FAT | kernel `Image` + `boot.scr` | `load virtio 0:1 ... Image` |
| p2 | `rootfs_a` | ext2/4 | slot A root (seeded from `rootfs.ext2`) | `root=/dev/vda2` when `active_slot=A` |
| p3 | `rootfs_b` | ext2/4 | slot B root (distinct copy, `/etc/slot_id=B`) | `root=/dev/vda3` when `active_slot=B` |

`/etc/slot_id` differs per slot (A vs B) so a slot switch is **observable** from
inside the guest (the PWU-AB-1 RED→GREEN evidence) — `findmnt /` plus
`cat /etc/slot_id` proves which physical partition is mounted as root.

## Status (§11.4.6 — honest)

- `boot.cmd` + `uboot.env` are **authored**; the GPT layout above is internally
  coherent with `../assemble_ab_disk.sh`.
- **UNVERIFIED-pending-u-boot.bin:** the script has NOT been run under real
  U-Boot (the `out/images/u-boot.bin` artifact is still building). A real
  slot switch / auto-rollback is therefore **unproven** until the conductor
  assembles the disk and boots it. No working slot-switch is claimed here.

## Sources verified 2026-06-11

- U-Boot Boot Count Limit (bootcount/bootlimit/altbootcmd/upgrade_available):
  https://docs.u-boot.org/en/latest/api/bootcount.html
- Mender U-Boot integration (BOOT_ORDER, swap-the-rootfs altbootcmd):
  https://docs.mender.io/operating-system-updates-yocto-project/board-integration/bootloader-support/u-boot/manual-u-boot-integration
- U-Boot `source` / `mkimage -T script`:
  https://docs.u-boot.org/en/latest/usage/cmd/source.html
- QEMU virt + U-Boot aarch64:
  https://docs.u-boot.org/en/stable/board/emulation/qemu-arm.html
- RAUC A/B + QEMU + format=verity:
  https://rauc.readthedocs.io/en/latest/examples.html
