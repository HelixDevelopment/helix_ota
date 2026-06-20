# Helix OTA — Fixed Summary (closed, short-form)

**Revision:** 3
**Last modified:** 2026-06-20T00:30:00Z

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
| OTA-015 | §11 | Implemented (→ Fixed.md) | Feature | A/B slot switch via U-Boot BOOT_ORDER (commit `18ed84a`, evidence `docs/qa/20260611T094958Z-ab-slot-switch/`) |
| OTA-016 | §12 | Implemented (→ Fixed.md) | Feature | RAUC dd-apply to inactive slot with dm-verity (evidence `docs/qa/20260620T051026Z-ab-rauc-verity/`) |
| OTA-017 | §13 | Implemented (→ Fixed.md) | Feature | U-Boot corrupt-slot auto-rollback via bootcount (commit `42be557`, evidence `docs/qa/20260611T095918Z-ab-rollback/`) |
| OTA-018 | §14 | Implemented (→ Fixed.md) | Feature | ApplyPort CLI + slot manager + Ed25519 verifier (58/58 tests, `server/internal/device/`) |
