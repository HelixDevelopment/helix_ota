# Helix OTA — Fixed Summary (closed, short-form)

**Revision:** 2
**Last modified:** 2026-06-10T18:30:00Z

Short-form companion of [`Fixed.md`](Fixed.md) (§11.4.53 parity). Closed
items only; open items appear in [`Issues_Summary.md`](Issues_Summary.md).
Sorted closure-date DESC. All commit hashes read from `git log`.

| OTA ID | # | Status | Type | One-line description |
|---|---|---|---|---|
| OTA-001 | §1 | Fixed (→ Fixed.md) | Bug | `UpdateAvailable` payload now carries `deployment_id` so devices can echo it in telemetry (commit `3c57867`, `TestEmulatorSelfServesDeploymentID`) |
| OTA-002 | §2 | Implemented (→ Fixed.md) | Feature | New `GET /deployments` list endpoint enumerates deployments with cursor pagination (commit `3c57867`, `TestDeploymentListReturnsActive`) |
| OTA-005 | §5 | Implemented (→ Fixed.md) | Feature | Per-device telemetry filters `?event`/`?since`/`?until` (commit `50ef5c6`) |
| OTA-006 | §6 | Implemented (→ Fixed.md) | Feature | Group + group-members cursor pagination (commit `50ef5c6`) |
| OTA-007 | §7 | Completed (→ Fixed.md) | Task | Dashboard API client synced to new pagination/filter params (commit `b0b8ee2`) |
| OTA-008 | §8 | Implemented (→ Fixed.md) | Feature | Tier-1 Go OTA device-emulator `server/internal/deviceemu` + `cmd/ota-device-emu` (commit `7dc3334`) |
| OTA-009 | §9 | Completed (→ Fixed.md) | Task | Comprehensive dashboard UI testing system for all panels (commit `fa571b8`) |
| OTA-010 | §10 | Completed (→ Fixed.md) | Task | Autonomous e2e + security + HelixQA for telemetry filters & pagination (commit `a839220`) |
