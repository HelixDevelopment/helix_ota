# Stream D: Documentation Sync + CodeGraph Re-index — Report

**Date:** 2026-06-19T21:00Z  
**HEAD:** 9b8fb719

---

## 1. Feature Status Exports (Status.md + Status_Summary.md)

| File | HTML | PDF | DOCX |
|---|---|---|---|
| docs/features/Status.md | PASS (59,825 B) | PASS (81,621 B) | PASS (29,562 B) |
| docs/features/Status_Summary.md | PASS (12,652 B) | PASS (35,764 B) | PASS (14,927 B) |

## 2. Issues/Fixed Tracker Exports

| File | HTML | PDF |
|---|---|---|
| docs/Issues.md | PASS (8,053 B) | PASS (30,169 B) |
| docs/Issues_Summary.md | PASS (5,006 B) | PASS (21,567 B) |
| docs/Fixed.md | PASS (9,339 B) | PASS (40,307 B) |
| docs/Fixed_Summary.md | PASS (6,459 B) | PASS (25,560 B) |

## 3. Other Tracked Docs

| File | HTML | PDF |
|---|---|---|
| docs/research/main_specs/CONTINUATION.md | PASS (35,072 B) | PASS (126,360 B) |
| docs/emulator/rk3588_ab_virt/Status.md | PASS (19,712 B) | PASS (99,401 B) |
| docs/emulator/rk3588_ab_virt/Status_Summary.md | PASS (13,612 B) | PASS (44,691 B) |

## 4. docs_chain verify

| Context | Result | Detail |
|---|---|---|
| issues-status | PASS (synced) | 8 stale exports auto-applied |
| emulator-status | PASS (in-sync) | No changes needed |
| features-status | FAIL (config) | docs_chain v1 does not support `fingerprint` field; exported directly via pandoc |

## 5. CodeGraph Re-index

| Metric | Value |
|---|---|
| Files indexed | 1,969 |
| Total nodes | 39,230 |
| Total edges | 145,604 |
| DB size | 100.86 MB |
| Backend | node:sqlite (WAL) |
| Top languages | Go (1,611), YAML (156), TSX (67), Kotlin (49) |

## 6. README Doc-Links

- Features Status rows added (DOCX column)
- Emulator Status rows added
- Revision/date values refreshed
- DOCX column added to table

## 7. Verdict

All exports PASS. CodeGraph re-index PASS. README doc-links updated. docs_chain 2/3 PASS.
