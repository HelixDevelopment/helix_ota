# Helix OTA — Issues (open workable items)

**Revision:** 7
**Last modified:** 2026-06-21T14:00:00Z

This is the canonical open-work tracker (§11.4.15 Status, §11.4.16 Type,
§11.4.54 OTA-NNN). Closed items migrate to [`Fixed.md`](Fixed.md). The
short-form companion is [`Issues_Summary.md`](Issues_Summary.md).

---

## §3. [OTA-003] Emulator Tier-2 — real Android A/B (update_engine/AVB/dm-verity auto-rollback)

**Status:** In progress
**Type:** Task

**Description:** Stand up the Tier-2 emulator — a real Android A/B
`update_engine` payload-apply with AVB/dm-verity verification and
auto-rollback, driven end-to-end against the control plane (per
`docs/design/EMULATED_DEVICE_TESTING.md`). This was previously
blocked on a Linux+KVM host; the Linux host `nezha.local` (x86_64,
62 GB RAM, 8 vCPUs, KVM enabled) is now available, unblocking
Cuttlefish (`cvd`) deployment for Tier-2 validation.

---

## §4. [OTA-004] Emulator Tier-3 — real RK3588 / Orange Pi 5 Max vendor HAL, U-Boot slot-switch, dm-verity on real partitions

**Status:** Operator-blocked
**Type:** Task

**Operator-Block-Details:**
- **WHAT:** Tier-3 validation on the physical board — real vendor HAL,
  U-Boot A/B slot-switch, and dm-verity over real partitions, exactly
  as a fielded device runs (per `docs/design/EMULATED_DEVICE_TESTING.md`).
- **WHY:** Requires the physical RK3588 / Orange Pi 5 Max hardware,
  which is not attached to this host. Self-resolution exhausted
  (§11.4.21): (a) no remote board over the available CLIs/ADB/SSH;
  (b) subagent delegation cannot substitute for absent silicon;
  (c) no repo tooling emulates real U-Boot slot-switch + on-silicon
  dm-verity; (d) no captured fallback reproduces real-partition AVB
  rollback; (e) research confirms the vendor HAL + bootloader path is
  board-specific. NOT structurally impossible (§11.4.112) —
  hardware-gated only.
- **UNBLOCK CONDITION:** A physical RK3588 / Orange Pi 5 Max board
  reachable over ADB/SSH (or physically attached) for flashing.
- **WHO:** Operator — attach / provide remote access to the board.

## §5. [OTA-021] HelixTrack bidirectional sync verification

**Status:** In progress
**Type:** Task

**Description:** Verify that changes to workable items docs/DB are immediately synced to HelixTrack tickets and vice versa. End-to-end sync test with recorded video evidence.

---


