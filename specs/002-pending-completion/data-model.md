# Data Model: Pending Work Completion

No new entities. This phase updates existing records only.

## Workable Items DB Update Plan

### Items to mark Closed (code evidence exists, committed and pushed)

| ota_id | New Status | Evidence |
|--------|-----------|----------|
| OTA-028 | Completed (→ Fixed.md) | ota-rollout-engine SuccessThreshold==0 fix, tests PASS |
| OTA-029 | Completed (→ Fixed.md) | config/multitrack/the-factory.yaml logic groups |
| OTA-030 | Completed (→ Fixed.md) | config/multitrack/aliases.yaml.example |
| OTA-031 | Completed (→ Fixed.md) | OTA-003 migrated Issues→Fixed, RESUMPTION updated |
| OTA-039 | Completed (→ Fixed.md) | Verified: applyport CLI has zero placeholders |
| OTA-040 | Completed (→ Fixed.md) | DDoS/memory/race test stubs, F86 6/12→9/12 |
| OTA-044 | Completed (→ Fixed.md) | QA handoff checklist (53 steps), gate wired |
| OTA-058 | Completed (→ Fixed.md) | Containers ~80 commits merged, pointer bumped |
| OTA-061 | Completed (→ Fixed.md) | Feature evidence script (624 lines, 20+ APIs) |
| OTA-067 | Fixed (→ Fixed.md) | Telemetry schema_version + validation, 23 packages PASS |
| OTA-069 | Fixed (→ Fixed.md) | OverlayNetwork Connect() cross-host enumeration |
| OTA-070 | Fixed (→ Fixed.md) | Tunnel AutoReconnect with exponential backoff, 76 tests PASS |
| OTA-072 | Completed (→ Fixed.md) | 3/3 CLI binaries verified to have tests |

### Items to leave Queued (hardware-gated)

| ota_id | Status | Unblock Condition |
|--------|--------|-------------------|
| OTA-038 | Queued | RK3588 Orange Pi 5 Max with Linux/U-Boot |
| OTA-041 | Queued | Research-gated — needs U-Boot fw_setenv hardware test |
| OTA-042 | Queued | RK3588 Tier-3 on-silicon A/B test |
| OTA-043 | Queued | Android bricks on real RK3588 device |

### Items to leave as-is

| ota_id | Current Status | Reason |
|--------|---------------|--------|
| OTA-021 | In progress | Operator-blocked — HelixTrack needs admin onboarding |
| OTA-051/052/053 | Queued | Large scope — Accounts M6-M8 needs full §11.4.167 workstream |
| OTA-059 | Queued | VM/emu hardening backlog — software-actionable but large scope |
| OTA-071 | Queued | QEMU e2e integration — software-actionable but large scope |

## DB Update SQL

```sql
-- Pattern for each item:
UPDATE items SET status = 'Completed (→ Fixed.md)', modified_at = datetime('now') 
WHERE ota_id = 'OTA-028';
-- (repeat for 13 items with appropriate status based on type)
```
