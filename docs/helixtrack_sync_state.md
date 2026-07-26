# HelixTrack Sync State

**Last synced:** 2026-06-24T06:06:08Z
**Last verified:** 2026-07-26T07:33:00Z
**Direction:** push (workable_items.db → HelixTrack API)
**Pushed:** 16 (at last sync)
**Current DB items:** 68
**Not yet synced:** 52 (OTA-022 through OTA-073)

## Status: OPERATOR-BLOCKED

HelixTrack API requires onboarding/setup before REST API calls (push/pull)
can proceed. The `htCore` binary starts and serves on :8080, but routes return
302 redirects to the HTTPS web login form. The space databases
(`helix_track/spaces/_default/data/helixtrack.db` and
`helix_track/spaces/helix_ota/data/helixtrack.db`) are not yet initialized.

### Sync scripts available and verified

| Script | Path | Status |
|--------|------|--------|
| Push sync | `scripts/sync_helixtrack_push.sh` | Ready — blocked on API auth |
| Pull sync | `scripts/sync_helixtrack_pull.sh` | Ready — blocked on API auth |
| E2E bidir test | `tests/helixqa/helix_e2e_bidir_sync_test.sh` | Ready — blocked on API auth |

### What's needed to unblock

1. Complete Space onboarding via the web UI (admin user creation)
   - Start `htCore` with `helix_track/spaces/helix_ota/` space
   - Navigate to `http://localhost:8080` and follow setup
   - Alternatively, set `onboarding_complete: true` in space config if DB
     is pre-seeded with admin credentials
2. Retrieve a valid JWT from `/api/auth/login` with the onboarded credentials
3. Run `HELIXTRACK_JWT="<token>" bash scripts/sync_helixtrack_push.sh`
4. Run `HELIXTRACK_JWT="<token>" bash scripts/sync_helixtrack_pull.sh`

### Items pending sync (since 2026-06-24)

| OTA ID | Status |
|--------|--------|
| OTA-022 | Completed (→ Fixed.md) |
| OTA-023 | Completed (→ Fixed.md) |
| OTA-024 | Fixed (→ Fixed.md) |
| OTA-025 | Fixed (→ Fixed.md) |
| OTA-026 | Completed (→ Fixed.md) |
| OTA-027 | Completed (→ Fixed.md) |
| OTA-028 | Queued |
| OTA-029 | Queued |
| OTA-030 | Queued |
| OTA-031 | Queued |
| OTA-032 | Completed (→ Fixed.md) |
| OTA-033 | Fixed (→ Fixed.md) |
| OTA-034 | Implemented (→ Fixed.md) |
| OTA-035 | Implemented (→ Fixed.md) |
| OTA-036 | Implemented (→ Fixed.md) |
| OTA-037 | Implemented (→ Fixed.md) |
| OTA-038 through OTA-073 | (various — 34 items) |

| Status | Count |
|--------|-------|
| Synced (June 2024) | 16 |
| Pending sync | 52 |
| **Total in DB** | **68** |

---

*Last updated: 2026-07-26 during OTA-021 verification*
