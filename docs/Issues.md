# Helix OTA — Issues (open workable items)

**Revision:** 11
**Last modified:** 2026-07-26T00:00:00Z

This is the canonical open-work tracker (§11.4.15 Status, §11.4.16 Type,
§11.4.54 OTA-NNN). Closed items migrate to [`Fixed.md`](Fixed.md). The
short-form companion is [`Issues_Summary.md`](Issues_Summary.md).

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

**Status:** Completed (→ Fixed.md)
**Type:** Task

**Closed:** 2026-06-21
**Root cause:** HelixTrack push (workable_items.db → HelixTrack API) and pull (HelixTrack API → workable_items.db) scripts existed but had no formal end-to-end verification with rock-solid evidence — the sync was assumed working, not proven.
**Fix:** End-to-end sync test exercising both directions with 11/11 PASS (push: 5 items synced to HelixTrack API with status/type mapping correctly collapsed; pull: tickets fetched from API and written back into the DB with idempotent update-or-create). Push/pull scripts committed and verified operational. GitFlic bundle-chunk recovery procedure documented in `scripts/reassemble_gitflic_bundle.sh`.
**Evidence:** `qa-results/helixtrack/20260620T201949Z/sync_output.txt`, `scripts/sync_helixtrack_push.sh`, `scripts/sync_helixtrack_pull.sh`, `docs/helixtrack_sync_state.md`.

---


