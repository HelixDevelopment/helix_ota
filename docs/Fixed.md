# Helix OTA — Fixed (closed workable items)

**Revision:** 3
**Last modified:** 2026-06-20T00:30:00Z

This is the canonical closed-archive tracker (§11.4.19 column alignment,
§11.4.33 type-aware closure vocabulary, §11.4.54 OTA-NNN). Open items
live in [`Issues.md`](Issues.md); the short-form companion is
[`Fixed_Summary.md`](Fixed_Summary.md). Sorted closure-date DESC. All
commit hashes below were read from `git log`, not invented.

---

## §1. [OTA-001] Telemetry `deployment_id` not derivable from `otaprotocol.UpdateAvailable`

**Status:** Fixed (→ Fixed.md)
**Type:** Bug

Closed 2026-06-10 — commit `3c57867` `feat(api): close 2 protocol gaps —
UpdateAvailable.deployment_id + GET /deployments`. The `ota-protocol`
`UpdateAvailable` payload now carries the `deployment_id` field, wired
through the server's update-check response, so a real device can obtain
and echo back the `deployment_id` its telemetry must supply (previously
the server required it but the protocol never told the device). Proven
by `TestEmulatorSelfServesDeploymentID` — the Tier-1 emulator
round-trips register → update-check → telemetry self-serving the
`deployment_id` end-to-end.

---

## §2. [OTA-002] No `GET /deployments` list endpoint to enumerate deployments

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-10 — commit `3c57867` `feat(api): close 2 protocol gaps —
UpdateAvailable.deployment_id + GET /deployments`. A `GET /deployments`
list endpoint now enumerates existing deployments with the same
cursor-pagination convention as the group/members endpoints (`50ef5c6`),
so the dashboard and operator/automation clients can discover deployment
IDs without already knowing them. Proven by
`TestDeploymentListReturnsActive` — the list endpoint returns the active
deployments.

---

## §5. [OTA-005] Per-device telemetry filters (`?event` / `?since` / `?until`)

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-10 — commit `50ef5c6` `feat(api): per-device telemetry
filters + group/members pagination`. Per-device telemetry endpoint now
accepts `?event`, `?since`, `?until` query filters; OpenAPI synced
(redocly-clean).

---

## §6. [OTA-006] Group + group-members cursor pagination

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-10 — commit `50ef5c6` `feat(api): per-device telemetry
filters + group/members pagination`. Group and group-members listing
endpoints gained cursor-based pagination.

---

## §7. [OTA-007] Dashboard API client synced to new pagination/filter params

**Status:** Completed (→ Fixed.md)
**Type:** Task

Closed 2026-06-10 — commit `b0b8ee2` `chore(dashboard): sync API client
to new pagination/filter params`. Dashboard API client brought into
lockstep with the new telemetry-filter + pagination parameters
(OTA-005/OTA-006).

---

## §8. [OTA-008] Tier-1 Go OTA device-emulator

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-10 — commit `7dc3334` `feat(emulator): Tier-1 Go OTA
device-emulator + resilience for new handlers`. A Go device-emulator
(`server/internal/deviceemu` + `cmd/ota-device-emu`) that speaks the
real `ota-protocol` to the control plane; surfaced the telemetry
`deployment_id` protocol gap now tracked as OTA-001.

---

## §9. [OTA-009] Comprehensive dashboard UI testing system

**Status:** Completed (→ Fixed.md)
**Type:** Task

Closed 2026-06-10 — commit `fa571b8` `test(dashboard): comprehensive UI
testing system for all panels`. UI test suite covering all dashboard
panels (Vitest + Playwright + a11y).

---

## §10. [OTA-010] Autonomous e2e + security + HelixQA for telemetry filters & pagination

**Status:** Completed (→ Fixed.md)
**Type:** Task

Closed 2026-06-10 — commit `a839220` `test(qa): autonomous e2e +
security + HelixQA for telemetry filters & pagination`. Autonomous
end-to-end + security suites + HelixQA bank covering the OTA-005/OTA-006
telemetry-filter and pagination work.

---

## §11. [OTA-015] A/B slot switch via U-Boot BOOT_ORDER

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-11 — commit `18ed84a` `feat(AB): PWU-AB-1 A/B slot switch proven`. Full A/B slot switch via U-Boot BOOT_ORDER env var against real U-Boot 2024.01 + QEMU `virt` + HVF on the emulator T1 ladder. Boots A → reboots to B via fw_setenv → verifies HELIX_SLOTID=B, HELIX_ROOTDEV=/dev/vda3. Proven 3/3 deterministic. Evidence: `docs/qa/20260611T094958Z-ab-slot-switch/`. Runtime signature registered in `docs/design/rk3588_ab_virt/runtime-signatures.yaml` (PWU-AB-1-SLOT-SWITCH).

---

## §12. [OTA-016] RAUC dd-apply to inactive slot with dm-verity

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-20 — PWU-AB-2 RAUC reconciliation: dd rootfs to inactive slot + fw_setenv BOOT_ORDER switch so guest boots the new slot. RAUC slot-class scheme reconciled with `uboot_ab/boot.cmd` BOOT_ORDER env scheme. Proven 3/3 deterministic. Evidence: `docs/qa/20260620T051026Z-ab-rauc-verity/`. Runtime signature: PWU-AB-2-RAUC-DD.

---

## §13. [OTA-017] U-Boot corrupt-slot auto-rollback via bootcount

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-11 — commit `42be557` `feat(AB): PWU-AB-3 corrupt-slot AUTO-ROLLBACK proven`. bootcount > bootlimit triggers altbootcmd which swaps to the good slot. ROLLBACK run booted from bad slot B and rolled back to good slot A (HELIX_SLOTID=A, HELIX_ROOTDEV=/dev/vda2). Proven 2/2 deterministic. Evidence: `docs/qa/20260611T095918Z-ab-rollback/`. Runtime signature: PWU-AB-3-ROLLBACK.

---

## §14. [OTA-018] ApplyPort CLI + slot manager + Ed25519 verifier

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-20 — PWU-AB-4 ApplyPort (`server/cmd/applyport/`). Slot detection from `/proc/cmdline` (`helix_slot=A|B`), Ed25519 artifact signature verification via `crypto/ed25519` (proven real, not a stub — §1.1 mutation `TestMutationSignatureUsesRealEd25519`), write-and-arm with health marker, and device client (login, check-for-update, apply). 58/58 tests passing including §1.1 paired-mutation suite. Evidence: `server/internal/device/`. Runtime signatures: PWU-AB-4-APPLYPORT-BUILD, PWU-AB-4-APPLYPORT-TESTS, PWU-AB-4-SLOT-DETECTION, PWU-AB-4-SIGNATURE-VERIFIER.
