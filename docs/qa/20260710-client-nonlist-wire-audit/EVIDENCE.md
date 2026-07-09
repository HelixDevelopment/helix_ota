# Client non-list wire-shape audit — ota-manager

**Revision:** 1
**Last modified:** 2026-07-10T04:10:00Z

Anti-bluff discovery-pressure sweep (§11.4.6 no-guessing, §11.4.108
source-truth, §11.4.118 discovery-pressure) over every NON-LIST response
interface in `clients/ota-manager/src/lib/api-client.ts`, following the two
confirmed shape-drift bugs already fixed in this client (BUG-1
`TelemetryOverview` fabrication; the six list-interface `{items,total,cursor}`
vs real `{items,next_cursor}` drift, commit `f711b1f9`).

## Headline finding

The completeness sweep this task was designed to run **did** turn up the
predicted class of defect, at a scale much larger than the conductor's
working hypothesis ("one confirmed omission — `TelemetryHistory.device_id`
— plus a spot-check of the rest"). Every real, returned-by-a-handler
non-list interface checked against its Go struct — 15 of them — was found to
have genuine field-level drift, ranging from a single missing optional field
(`TokenResponse.roles`) to entire fabricated shapes with almost no field
overlap (`DeviceRegistered`, `DeviceStatus`, `DeviceHealth`,
`ReleaseResponse`, `Deployment`, `RolloutState`, `RolloutDecision`, `Group`).
Two of the fabricated shapes (`Deployment`, `AuditEntry`) were ALSO the
per-item type nested inside an already-"fixed" list interface
(`DeploymentList.items`, `AuditLogList.items`), so the earlier
list-pagination fix (commit `f711b1f9`) corrected the outer envelope while
the item contents underneath stayed wrong.

## Per-interface audit table

| Interface | Verdict | Real server citation |
|---|---|---|
| `TokenResponse` | DRIFT — missing `roles?: string[]` | `server/internal/api/wire.go:29-36` |
| `DeviceRegistered` | DRIFT — fabricated `{id, board, status, created_at}`; real shape `{device_id, hardware_id, device_token, token_type, expires_in, registered_at}` | `server/internal/api/wire.go:52-59` via `handlers_device.go:192-202` |
| `DeviceList.items` (new `DeviceListItem`) | DRIFT — typed as `DeviceRegistered[]` (the one-time registration-response shape); real per-item shape is `DeviceListItem` (`device_id, hardware_id, model, os, os_version?, current_version?, target_version, group?, update_state, active_slot?, health_ok, last_seen?, registered_at`) | `server/internal/api/wire.go:81-95` via `handlers_device.go:167-190` |
| `DeviceStatus` | DRIFT — fabricated `{device_id, online, current_release_id, last_seen, ip_address}`; real shape `{device_id, hardware_id, current_version?, target_version, last_seen?, update_state, active_slot?, health}` | `server/internal/api/wire.go:69-78` via `handlers_device.go:204-230` |
| `DeviceHealth` | DRIFT — fabricated top-level `{device_id, online, battery_percent, storage_used_percent, temperature_celsius, uptime_seconds, last_reported}`; real shape is the NESTED `{ok, last_error_code}` block of `DeviceStatus.health` — no such standalone endpoint exists | `server/internal/api/wire.go:62-65` |
| `Release`/`ReleaseResponse` | DRIFT — fabricated `{id, project_id, file_url, file_hash, changelog, target_board, firmware_version, created_by}`; real shape `{release_id, artifact_id, version, os, target_model, status, notes?, created_at}` | `server/internal/api/wire.go:147-156` via `handlers_release.go:126-138` |
| `Deployment` | DRIFT — fabricated `{group_ids: string[], rollout_percentage, staged, created_by}`; real shape `{deployment_id, release_id, strategy, group?, status, target_count, created_at}` | `server/internal/api/wire.go:174-182` via `handlers_deployment.go:226-237` |
| `DeploymentStatus` (new `DeploymentProgress`) | DRIFT — fabricated `{rollout_state?, failed_devices?, completed_devices?, total_devices?}`; real shape is `Deployment & {progress: {pending, downloading, installed, succeeded, failed}}` | `server/internal/api/wire.go:193-205` via `handlers_deployment.go:121-132,190-224` |
| `RolloutState` (new `RolloutPhaseSpec`) | DRIFT — fabricated `{progress, success_rate, error_rate, total_devices, completed_devices, failed_devices}`; real shape `{deployment_id, status, current_phase, phases[], updated_at}` | `server/internal/api/handlers_rollout.go:16-44,67-75` |
| `RolloutDecision` | DRIFT — fabricated `{approved: boolean, reason}`; real shape `{action, reason, state: RolloutState}` | `server/internal/api/handlers_rollout.go:47-51,129-160` |
| `Project` | DRIFT — fabricated `os_types: string[]` field; missing real `updated_at` | `server/internal/api/wire.go:260-266` via `handlers_project.go:95-103` |
| `Group` (list item of `GroupList`) | DRIFT — fabricated `{id, project_id, device_count, labels}`; real shape `{group_id, name, description?, member_count, created_at}` (groups are NOT project-scoped) | `server/internal/api/handlers_group.go:54-60,97-115` |
| `GroupMembers` (new `GroupMemberView`) | DRIFT — fabricated `{group_id, devices: DeviceRegistered[]}`; real shape `{group_id, items: {device_id, added_at}[], next_cursor}` (cursor-paginated) | `server/internal/api/handlers_group.go:84-95` |
| `AddGroupMembersResult` | DRIFT — fabricated `{added: number, failed: number}`; real shape `{added: string[], already_member: string[], not_found: string[]}` | `server/internal/api/handlers_group.go:77-81` |
| `Artifact` | **VERIFIED CORRECT** — no drift | `server/internal/api/wire.go:122-132` |
| `ArtifactUploadMetadata` | **VERIFIED CORRECT** — no drift | `server/internal/api/wire.go:107-119` |
| `DeltaView` | **VERIFIED CORRECT** — no drift | `server/internal/api/handlers_delta.go:24-32` |
| `AuditEntry` (new `AuditActor`, list item of `AuditLogList`) | DRIFT — `actor` was a bare `string`; real shape nests `{user_id?, subject}`; `user_agent` entirely missing | `server/internal/api/audit_wire.go:9-31` via `:39-55` |
| `TelemetryHistory` (new `TelemetryEventView`) | DRIFT — missing required `device_id` (the conductor-flagged omission); `items` was untyped `Record<string, unknown>[]` instead of the real `TelemetryEventView` shape | `server/internal/api/handlers_telemetry.go:15-38,66-78` |
| `TelemetryOverview` | **VERIFIED CORRECT** (already fixed under BUG-1) — no further drift | `server/internal/api/handlers_telemetry.go:56-64` |
| `RollbackList.items` (new `RollbackView`) | DRIFT — inline item shape `{id, deployment_id, reason, created_at}` dropped `kind`, `from_release_id`, `to_release_id`, `recall_deployment_id`, `triggered_by`, `details` | `server/internal/api/handlers_recall.go:23-39` via `:41-48` |

**Result: 15 of 19 checked interfaces had real field-level drift; 4
(`Artifact`, `ArtifactUploadMetadata`, `DeltaView`, `TelemetryOverview`)
were already correct.**

## Out-of-scope discoveries (flagged, not fixed)

This task's charter is RESPONSE-shape interfaces. While reading every real
handler to verify each response, the following REQUEST-body interfaces were
also found drifted from the real Go request structs. They are flagged here
as follow-up work items — fixing them was judged out of scope (tight-scope
discipline, §11.4.20) because correcting a request body also requires
correcting the create-wizard/dialog code that builds it, a materially larger
change surface than this response-shape audit:

- `DeviceRegistrationRequest` vs real `DeviceRegistration`
  (`server/internal/api/wire.go:41-49`) — no field overlap at all.
- `CreateReleaseRequest` vs real `ReleaseCreate`
  (`server/internal/api/wire.go:136-144`) — no field overlap; the
  create-release wizard's own code comment already flags this submission
  path as non-functional.
- `RecallRequest` vs real `RecallRequest`
  (`server/internal/api/handlers_recall.go:17-20`) — sends `{reason, force}`,
  server requires `{to_release_id, reason?}`; the real endpoint would reject
  every recall request this client currently sends.
- `CreateDeploymentRequest`/`CreateRolloutRequest` — not cross-checked in
  detail (out of scope), but given the pattern above are very likely
  similarly drifted; noted for the same follow-up item.

Each of these has an inline comment added at its declaration site in
`api-client.ts` (except the two Create* request types, noted here only, to
avoid ballooning the diff) citing the real struct and file:line.

## Files changed

- `clients/ota-manager/src/lib/api-client.ts` — all interface fixes listed
  in the table above; added `DeviceListItem`, `DeploymentProgress`,
  `RolloutPhaseSpec`, `GroupMemberView`, `AuditActor`, `RollbackView`,
  `TelemetryEventView`.
- `clients/ota-manager/src/types/api.ts` — re-export the new interfaces.
- `clients/ota-manager/src/hooks/useDevices.ts` — device-list adapter:
  `DeviceListItem` field remap (`device_id/hardware_id/os/current_version`),
  new `deriveViewStatus` honestly derived from `health_ok`/`update_state`
  (previously matched a `status` field that never existed on the wire, so
  every device rendered "unknown"), `targetModel` now wired from the real
  `model` field (previously never populated).
- `clients/ota-manager/src/features/devices/device-detail-page.tsx` —
  `DeviceStatus` field remap (`hardware_id/current_version/health.ok/
  update_state/active_slot`); `currentSlot`/`healthStatus` now wired from
  real fields (previously permanent placeholders); dropped the never-real
  `ipAddress`.
- `clients/ota-manager/src/hooks/useDeployments.ts` — `d.group` (real
  singular field) instead of `d.group_ids[0]` (never existed).
- `clients/ota-manager/src/hooks/useDeployment.ts` — dropped
  `created_by`/`rollout_percentage` sourcing (no real field), made both
  optional on the shim's own view-model interface; `targetGroupName` now
  from `d.group`.
- `clients/ota-manager/src/hooks/use-groups.ts` — `data.group_id` (real
  primary key) instead of `data.id` in `useCreateGroup`/`useUpdateGroup`.
- `clients/ota-manager/src/hooks/useGroups.ts` — `g.group_id`/`g.member_count`
  instead of `g.id`/`g.device_count`.
- `clients/ota-manager/src/hooks/use-releases.ts` — `data.release_id`
  instead of `data.id` in `useCreateRelease`.
- `clients/ota-manager/src/hooks/useReleases.ts` — `r.release_id/r.os/
  r.target_model` instead of `r.id/r.firmware_version/r.target_board`.
- `clients/ota-manager/src/hooks/useAuditLog.ts` — `e.actor.subject` instead
  of `e.actor` (now a nested object).
- `clients/ota-manager/src/hooks/use-projects.ts` — placeholder `Project`
  fixture: dropped fabricated `os_types`, added real `updated_at`.
- `clients/ota-manager/src/__tests__/wire-shape-pagination.test.ts` — added
  the now-required `device_id` to its pre-existing `TelemetryHistory`
  fixture (this file's own scope is the list/pagination shape; it needed a
  one-field update to keep compiling under the corrected interface).
- `clients/ota-manager/src/__tests__/use-telemetry-overview.test.tsx` — its
  `deploymentsFixture` used the old fabricated `Deployment` fields
  (`group_ids`/`rollout_percentage`/`staged`); replaced with the real
  `target_count` field (the fixture's assertions only ever depended on
  `.items.length`, unaffected).
- `clients/ota-manager/src/__tests__/wire-shape-nonlist.test.ts` (NEW) —
  the guard test for this audit; compile-time-only, `tsc --noEmit` is the
  oracle, same pattern as `wire-shape-pagination.test.ts`.

No caller needed changes for: `Artifact`/`ArtifactUploadMetadata`/`DeltaView`
(unchanged, verified correct), `TelemetryOverview` (unchanged, already
fixed), `features/audit/audit-page.tsx` and
`features/dashboard/dashboard-page.tsx` (consume the already-mapped
`AuditView`/`TelemetryOverviewView` shim types, which absorbed the
`AuditEntry`/nothing-changed-here field changes at the adapter boundary),
`features/deployments/deployments-page.tsx`, `features/deployments/
deployment-detail-page.tsx` (consume the local shim `Deployment` view
types, whose fields stayed the same — only the shim's internal mapping
changed), `features/releases/releases-page.tsx`, `features/groups/
groups-page.tsx` (same pattern — shim absorbs the wire change).

## RED evidence (captured, §11.4.115)

Temporarily reverted `TelemetryHistory` in `api-client.ts` to drop the
required `device_id` field, then ran `npx tsc --noEmit`:

```
src/__tests__/wire-shape-nonlist.test.ts(174,5): error TS2353: Object literal may only specify known properties, and 'device_id' does not exist in type 'TelemetryHistory'.
src/__tests__/wire-shape-nonlist.test.ts(180,3): error TS2578: Unused '@ts-expect-error' directive.
src/__tests__/wire-shape-pagination.test.ts(60,48): error TS2353: Object literal may only specify known properties, and 'device_id' does not exist in type 'TelemetryHistory'.
```

Restored the fix; `npx tsc --noEmit` exit 0 (GREEN, reconfirmed below).

Additionally, during authoring, the FULL set of `@ts-expect-error` "old
fabricated shape" assertions in `wire-shape-nonlist.test.ts` were verified to
actually fire (not silently pass) by the intermediate `tsc` run that
surfaced `TS2578: Unused '@ts-expect-error' directive` for six of them
(before the object literals were reformatted to single-line so the directive
aligned with the reported error line) — i.e. every "this OLD shape should
be a compile error" assertion was proven to genuinely reproduce a TS error
against the corrected interfaces, not merely assumed.

## Verification

- `npx tsc --noEmit` — **exit 0** (GREEN, full project, all interfaces +
  all callers + all test fixtures).
- `npx vitest run` — **13 test files passed (13), 51 tests passed (51)**,
  9.26s.

## Constraints honored

- No `server/*.go` files modified (read-only, source of truth).
- No submodule, `dist/`, or other `docs/qa/` directories modified.
- No git commands run by this agent; conductor commits.
