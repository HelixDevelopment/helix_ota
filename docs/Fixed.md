# Helix OTA — Fixed (closed workable items)

**Revision:** 2
**Last modified:** 2026-06-10T18:30:00Z

This is the canonical closed-archive tracker (§11.4.19 column alignment,
§11.4.33 type-aware closure vocabulary, §11.4.54 ATM-NNN). Open items
live in [`Issues.md`](Issues.md); the short-form companion is
[`Fixed_Summary.md`](Fixed_Summary.md). Sorted closure-date DESC. All
commit hashes below were read from `git log`, not invented.

---

## §1. [ATM-001] Telemetry `deployment_id` not derivable from `otaprotocol.UpdateAvailable`

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

## §2. [ATM-002] No `GET /deployments` list endpoint to enumerate deployments

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

## §5. [ATM-005] Per-device telemetry filters (`?event` / `?since` / `?until`)

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-10 — commit `50ef5c6` `feat(api): per-device telemetry
filters + group/members pagination`. Per-device telemetry endpoint now
accepts `?event`, `?since`, `?until` query filters; OpenAPI synced
(redocly-clean).

---

## §6. [ATM-006] Group + group-members cursor pagination

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-10 — commit `50ef5c6` `feat(api): per-device telemetry
filters + group/members pagination`. Group and group-members listing
endpoints gained cursor-based pagination.

---

## §7. [ATM-007] Dashboard API client synced to new pagination/filter params

**Status:** Completed (→ Fixed.md)
**Type:** Task

Closed 2026-06-10 — commit `b0b8ee2` `chore(dashboard): sync API client
to new pagination/filter params`. Dashboard API client brought into
lockstep with the new telemetry-filter + pagination parameters
(ATM-005/ATM-006).

---

## §8. [ATM-008] Tier-1 Go OTA device-emulator

**Status:** Implemented (→ Fixed.md)
**Type:** Feature

Closed 2026-06-10 — commit `7dc3334` `feat(emulator): Tier-1 Go OTA
device-emulator + resilience for new handlers`. A Go device-emulator
(`server/internal/deviceemu` + `cmd/ota-device-emu`) that speaks the
real `ota-protocol` to the control plane; surfaced the telemetry
`deployment_id` protocol gap now tracked as ATM-001.

---

## §9. [ATM-009] Comprehensive dashboard UI testing system

**Status:** Completed (→ Fixed.md)
**Type:** Task

Closed 2026-06-10 — commit `fa571b8` `test(dashboard): comprehensive UI
testing system for all panels`. UI test suite covering all dashboard
panels (Vitest + Playwright + a11y).

---

## §10. [ATM-010] Autonomous e2e + security + HelixQA for telemetry filters & pagination

**Status:** Completed (→ Fixed.md)
**Type:** Task

Closed 2026-06-10 — commit `a839220` `test(qa): autonomous e2e +
security + HelixQA for telemetry filters & pagination`. Autonomous
end-to-end + security suites + HelixQA bank covering the ATM-005/ATM-006
telemetry-filter and pagination work.
