# PWU-AB-1 / PWU-AB-3 — A/B video evidence recordings

**Revision:** 1
**Last modified:** 2026-06-19T01:30:00Z
**Status:** active
**Type:** Task — evidence collection
**Scope:** Recorded asciinema casts of the real U-Boot 2024.01 + QEMU virt + TCG
console output, converted to MP4 + GIF, for the A/B slot-switch and auto-rollback
demos.

---

## Recording files

| File | Size | Format | Content |
|---|---|---|---|
| `helix_ota-ab-slot-switch.mp4` | 1.3 MB | MP4 (h.264) | Two sequential boots: slot A then slot B |
| `helix_ota-ab-slot-switch.cast` | 55 KB | asciinema v3 | Raw cast of slot switch demo |
| `helix_ota-ab-rollback.mp4` | 1.4 MB | MP4 (h.264) | Two boots: ROLLBACK then CONTROL |
| `helix_ota-ab-rollback.cast` | 60 KB | asciinema v3 | Raw cast of rollback demo |

## Recording 1: A/B Slot Switch (PWU-AB-1)

### Console key frames

**Boot A: BOOT_ORDER="A B"**
```
=> setenv BOOT_ORDER A B
=> printenv BOOT_ORDER -> BOOT_ORDER=A B
=> source boot.scr
A/B: active_slot=A root=/dev/vda2
# echo HELIX_SLOTID=$(cat /etc/slot_id) -> HELIX_SLOTID=A
# echo HELIX_ROOTDEV=$(findmnt -no SOURCE /) -> HELIX_ROOTDEV=/dev/vda2
```

**Boot B: BOOT_ORDER="B A"**
```
=> setenv BOOT_ORDER B A
=> printenv BOOT_ORDER -> BOOT_ORDER=B A
=> source boot.scr
A/B: active_slot=B root=/dev/vda3
# echo HELIX_SLOTID=$(cat /etc/slot_id) -> HELIX_SLOTID=B
# echo HELIX_ROOTDEV=$(findmnt -no SOURCE /) -> HELIX_ROOTDEV=/dev/vda3
```

## Recording 2: Auto-Rollback (PWU-AB-3)

### Console key frames

**ROLLBACK: BOOT_ORDER="B A", bootcount=2 > bootlimit=1**
```
=> setenv bootcount 2, bootlimit 1, upgrade_available 1
=> source boot.scr
A/B: bootcount=2 > bootlimit=1 -> rolling back (altbootcmd swap)
Saving Environment to Flash...
A/B: BOOT_ORDER=A B active_slot=A root=/dev/vda2
# echo HELIX_SLOTID=A
```

**CONTROL: BOOT_ORDER="B A", bootcount=1 (not exhausted)**
```
=> setenv bootcount 1, bootlimit 1, upgrade_available 1
=> source boot.scr
A/B: BOOT_ORDER=B A active_slot=B root=/dev/vda3
# echo HELIX_SLOTID=B
```

## Summary

| Feature | Verdict | Key evidence |
|---|---|---|
| Slot A boot | PASS | BOOT_ORDER="A B" -> root=/dev/vda2 -> slot_id=A |
| Slot B boot | PASS | BOOT_ORDER="B A" -> root=/dev/vda3 -> slot_id=B |
| Auto-rollback | PASS | bc=2 > bl=1 -> guard fires -> swaps to slot A |
| Control (no rollback) | PASS | bc=1 = bl=1 -> boots head B directly |
