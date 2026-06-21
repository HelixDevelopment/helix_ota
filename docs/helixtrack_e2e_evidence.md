# HelixTrack E2E Bidirectional Sync — Evidence

**Test run:** 2026-06-21T05:59:09Z
**API:** http://localhost:8080/do
**Local DB:** docs/workable_items.db
**Status:** PASS (11/11 checks passed)

## Summary

| Check | Result | Detail |
|-------|--------|--------|
| Dependency check | PASS | sqlite3, curl, jq all available |
| API reachability | PASS | HelixTrack API v1.0.0 at http://localhost:8080/do |
| JWT acquisition | PASS | Token obtained (349 chars) |
| DB pre-count | PASS | 16 items in local DB before sync |
| Push sync | PASS | 16 items pushed (0 failures) |
| Ticket list | PASS | 96 total tickets in HelixTrack API |
| Pull sync | PASS | 0 added, 96 updated |
| Count match | PASS | Pre-sync: 16, Post-sync: 16 — identical |
| ID set match | PASS | OTA-ID set identical before and after sync |
| Cross-surface OTA coverage | PASS | All 16 local OTA IDs present in HelixTrack (16 unique OTA IDs across 96 total tickets) |
| Cross-surface ID set equality | PASS | HT unique OTA IDs (16) = Local DB items (16) |

## DB Items (Post-Sync)

```
OTA-003|Task|Operator-blocked|Emulator Tier-2 — real Android A/B update_engine/AVB/dm-verity auto-rollback is host-gated
OTA-004|Task|Operator-blocked|Emulator Tier-3 — real RK3588 / Orange Pi 5 Max vendor HAL, U-Boot slot-switch, dm-verity on real partitions
OTA-005|Feature|Fixed (→ Fixed.md)|Per-device telemetry filters (?event/?since/?until)
OTA-006|Feature|Fixed (→ Fixed.md)|Group + group-members cursor pagination
OTA-007|Task|Fixed (→ Fixed.md)|Dashboard API client synced to new pagination/filter params
OTA-008|Feature|Fixed (→ Fixed.md)|Tier-1 Go OTA device-emulator
OTA-009|Task|Fixed (→ Fixed.md)|Comprehensive dashboard UI testing system
OTA-010|Task|Fixed (→ Fixed.md)|Autonomous e2e + security + HelixQA test suites
OTA-014|Task|Fixed (→ Fixed.md)|Docs Chain submodule distribution (constitution §11.4.106 Phase 6) operator-gated
OTA-015|Feature|Fixed (→ Fixed.md)|A/B slot switch via U-Boot BOOT_ORDER
OTA-016|Feature|Fixed (→ Fixed.md)|RAUC dd-apply to inactive slot with dm-verity
OTA-017|Feature|Fixed (→ Fixed.md)|U-Boot corrupt-slot auto-rollback via bootcount
OTA-018|Feature|Fixed (→ Fixed.md)|ApplyPort CLI + slot manager + Ed25519 verifier
OTA-019|Task|Fixed (→ Fixed.md)|Build resource stats tracker
OTA-020|Bug|Fixed (→ Fixed.md)|Database migration test
OTA-021|Task|In progress|HelixTrack bidirectional sync verification
```

## HelixTrack API Tickets (Post-Push)

```
Ticket count: 96
Unique OTA IDs: OTA-003 OTA-004 OTA-005 OTA-006 OTA-007 OTA-008 OTA-009 OTA-010 OTA-014 OTA-015 OTA-016 OTA-017 OTA-018 OTA-019 OTA-020 OTA-021 
```

## Pre/Post Counts

| Metric | Value |
|--------|-------|
| Pre-sync items | 16 |
| Post-sync items | 16 |
| HelixTrack total tickets | 96 |
| HelixTrack unique OTA IDs | OTA-003 OTA-004 OTA-005 OTA-006 OTA-007 OTA-008 OTA-009 OTA-010 OTA-014 OTA-015 OTA-016 OTA-017 OTA-018 OTA-019 OTA-020 OTA-021  |
| Pre-sync OTA IDs | OTA-003 OTA-004 OTA-005 OTA-006 OTA-007 OTA-008 OTA-009 OTA-010 OTA-014 OTA-015 OTA-016 OTA-017 OTA-018 OTA-019 OTA-020 OTA-021  |
| Post-sync OTA IDs | OTA-003 OTA-004 OTA-005 OTA-006 OTA-007 OTA-008 OTA-009 OTA-010 OTA-014 OTA-015 OTA-016 OTA-017 OTA-018 OTA-019 OTA-020 OTA-021  |

## Sync Output

### Push
```
[0;36m[helixtrack-push][0m Pushing workable items to HelixTrack...
[0;32m[helixtrack-push][0m Pushed [OTA-020] Database migration test
[0;32m[helixtrack-push][0m Pushed [OTA-019] Build resource stats tracker
[0;32m[helixtrack-push][0m Pushed [OTA-003] Emulator Tier-2 — real Android A/B update_engine/AVB/dm-verity auto-rollback is host-gated
[0;32m[helixtrack-push][0m Pushed [OTA-004] Emulator Tier-3 — real RK3588 / Orange Pi 5 Max vendor HAL, U-Boot slot-switch, dm-verity on real partitions
[0;32m[helixtrack-push][0m Pushed [OTA-014] Docs Chain submodule distribution (constitution §11.4.106 Phase 6) operator-gated
[0;32m[helixtrack-push][0m Pushed [OTA-005] Per-device telemetry filters (?event/?since/?until)
[0;32m[helixtrack-push][0m Pushed [OTA-006] Group + group-members cursor pagination
[0;32m[helixtrack-push][0m Pushed [OTA-007] Dashboard API client synced to new pagination/filter params
[0;32m[helixtrack-push][0m Pushed [OTA-008] Tier-1 Go OTA device-emulator
[0;32m[helixtrack-push][0m Pushed [OTA-009] Comprehensive dashboard UI testing system
[0;32m[helixtrack-push][0m Pushed [OTA-010] Autonomous e2e + security + HelixQA test suites
[0;32m[helixtrack-push][0m Pushed [OTA-015] A/B slot switch via U-Boot BOOT_ORDER
[0;32m[helixtrack-push][0m Pushed [OTA-016] RAUC dd-apply to inactive slot with dm-verity
[0;32m[helixtrack-push][0m Pushed [OTA-017] U-Boot corrupt-slot auto-rollback via bootcount
[0;32m[helixtrack-push][0m Pushed [OTA-018] ApplyPort CLI + slot manager + Ed25519 verifier
[0;32m[helixtrack-push][0m Pushed [OTA-021] HelixTrack bidirectional sync verification
[0;36m[helixtrack-push][0m Sync state written to docs/helixtrack_sync_state.md
[0;32m[helixtrack-push][0m Push complete — 16 items synced
```

### Pull
```
[0;36m[helixtrack-pull][0m Pulling updates from HelixTrack...
[0;32m[helixtrack-pull][0m Fetched 96 tickets from HelixTrack
[0;36m[helixtrack-pull][0m   UPDATE OTA-021 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-020 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-019 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-003 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-004 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-014 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-005 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-006 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-007 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-008 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-009 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-010 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-015 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-016 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-017 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-018 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-020 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-019 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-003 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-004 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-014 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-005 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-006 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-007 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-008 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-009 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-010 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-015 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-016 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-017 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-018 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-021 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-020 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-019 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-003 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-004 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-014 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-005 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-006 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-007 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-008 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-009 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-010 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-015 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-016 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-017 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-018 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-021 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-020 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-019 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-003 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-004 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-014 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-005 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-006 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-007 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-008 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-009 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-010 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-015 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-016 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-017 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-018 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-021 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-021 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-020 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-019 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-003 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-004 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-014 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-005 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-006 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-007 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-008 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-009 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-010 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-015 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-016 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-017 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-018 → Queued
[0;36m[helixtrack-pull][0m   UPDATE OTA-015 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-016 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-017 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-018 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-021 → In progress
[0;36m[helixtrack-pull][0m   UPDATE OTA-020 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-019 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-003 → Operator-blocked
[0;36m[helixtrack-pull][0m   UPDATE OTA-004 → Operator-blocked
[0;36m[helixtrack-pull][0m   UPDATE OTA-014 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-005 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-006 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-007 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-008 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-009 → Fixed (→ Fixed.md)
[0;36m[helixtrack-pull][0m   UPDATE OTA-010 → Fixed (→ Fixed.md)
[0;32m[helixtrack-pull][0m Pull sync complete: +0 added, 96 updated, 0 skipped
[0;32m[helixtrack-pull][0m Sync state written to docs/helixtrack_sync_state.md
```

---

*Generated by tests/helixqa/helix_e2e_bidir_sync_test.sh at 2026-06-21T05:59:09Z*
