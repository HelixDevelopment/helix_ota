# Client REQUEST-body wire-shape audit — `clients/ota-manager`

**Revision:** 1
**Last modified:** 2026-07-10T00:00:00Z

## Scope

Follow-up to the prior response-shape reconciliation pass. This pass audits
every REQUEST-body interface (`*Request` / `*Payload` / create-update-recall
input shapes) declared in `clients/ota-manager/src/lib/api-client.ts` against
the real Go request struct the corresponding `server/internal/api/*.go`
handler decodes, and fixes every caller (adapter-shim hooks, dialogs/forms)
that builds a request body from a drifted interface.

Method per §11.4.6 (no-guessing) / §11.4.108 (runtime-signature) / §11.4.115
(RED-baseline-on-broken-artifact): every claim below cites the exact server
`file:line` for the real Go struct. Nothing is asserted without a citation.

## Per-interface drift table

| Client interface | Verdict | Drift | Server citation |
|---|---|---|---|
| `LoginRequest` | **FIXED** | Client sent `{email, password}`; server requires `{username, password}`. Every login would 400. | `server/internal/api/wire.go:19-22` (`LoginRequest`), `handlers_auth.go:56-67` (`handleLogin`) |
| `DeviceRegistrationRequest` | **FIXED** (unused) | Client shape `{device_id, board, firmware_version, hardware_revision, serial_number}` matched none of the real fields. Discovered **unused** — `use-devices.ts`'s `useRegisterDevice` already used the correct `DeviceRegistration` type, so this had zero runtime impact; fixed for wire-shape correctness of the exported type. | `server/internal/api/wire.go:41-49` (`DeviceRegistration`) |
| `CreateReleaseRequest` | **FIXED** | Client sent `{project_id, version, file_url, file_hash, changelog, target_board, firmware_version}`; server requires `{artifact_id, version, os, target_model, notes?, min_current_version?}`. Real required `artifact_id`/`os`/`target_model` were entirely absent. | `server/internal/api/wire.go:137-144` (`ReleaseCreate`) |
| `CreateRolloutRequest` | **FIXED** | Client sent `{deployment_id, groups, rollout_percentage?, staged}`; server requires `{phases: RolloutPhaseSpec[]}` only (deployment id is a URL param). Real required `phases` was entirely absent — every create-rollout submission would 400 (`len(req.Phases) == 0`). | `server/internal/api/handlers_rollout.go:24-27` (`RolloutCreate`), `:93-97` (`handleCreateRollout`) |
| `RecallRequest` | **FIXED** (RED→GREEN proof below) | Client sent `{reason, force}`; server requires `{to_release_id, reason?}`. `force` does not exist server-side; `to_release_id` (mandatory) was entirely absent — the recall endpoint rejected EVERY request this client sent. | `server/internal/api/handlers_recall.go:17-20` (`RecallRequest`), `:63-72` (`handleRecall`) |
| `CreateDeploymentRequest` | **FIXED** | Client sent `{release_id, group_ids: string[], strategy, rollout_percentage?, staged}`; server requires `{release_id, strategy, group?}` — a single optional group name, never an array, and no `rollout_percentage`/`staged` field on this request at all. | `server/internal/api/wire.go:167-171` (`DeploymentCreate`) |
| `CreateGroupRequest` | **FIXED** | Client sent `{name, description, device_ids: string[], labels: Record<string,string>}`; server requires `{name, description?}` only. `device_ids`/`labels` do not exist on group creation — Go's JSON unmarshal silently ignored them (**silent-data-loss bug**, not a 400: the operator believed a new group started with the given members/labels; the server always created an empty group). | `server/internal/api/handlers_group.go:42-45` (`GroupCreate`) |
| `UpdateGroupRequest` | **FIXED** | Client carried an extra `labels?` field; groups have no `labels` concept anywhere on the server. Silently ignored (same silent-drop class as above). | `server/internal/api/handlers_group.go:48-51` (`GroupUpdate`) |
| `AddGroupMembersRequest` | **VERIFIED CORRECT** | `{device_ids: string[]}` — exact match, no drift. | `server/internal/api/handlers_group.go:71-73` (`MemberAdd`) |
| `ArtifactUploadMetadata` | **VERIFIED CORRECT** | Field-for-field match (`sha256, signature, version, os, target_model, file_hash?, file_size?, metadata_hash?, metadata_size?, payload_offset?, payload_size?`), no drift. | `server/internal/api/wire.go:107-119` (`ArtifactUploadMetadata`) |
| `DeltaRegisterRequest` | **VERIFIED CORRECT** | `{base_artifact_id, target_artifact_id, sha256?, size?, storage_ref?}` — exact match, no drift. | `server/internal/api/handlers_delta.go:15-21` (`DeltaRegister`) |
| `DeltaFindParams` | **VERIFIED CORRECT (out of scope)** | Query-string params (`?base=&target=`), not a JSON body — outside this audit's strict scope, but verified correct against `c.Query("base")`/`c.Query("target")` while auditing its sibling. | `server/internal/api/handlers_delta.go:89-97` (`handleFindDelta`) |
| `RefreshRequest` (inline, no named client interface) | **VERIFIED CORRECT** | `use-auth.ts`'s `useRefresh` already sends `{refresh_token}` inline, matching the server exactly. No interface to fix. | `server/internal/api/wire.go:25-27` (`RefreshRequest`) |

### Discovered but explicitly NOT fixed (out of strict wire-shape scope)

- **`CreateProjectRequest`/`UpdateProjectRequest`** (`server/internal/api/wire.go:248-257`) have no corresponding client interface in `api-client.ts` at all, and no page/hook in `clients/ota-manager` currently builds a project create/update request — project management is unwired in this SPA. Adding brand-new, never-exercised interfaces was judged out of scope for a drift-reconciliation pass (would be fabricating untested surface, not fixing existing drift).
- **`CreateDeploymentRequest.strategy` value domain**: `handleCreateDeployment` (`server/internal/api/handlers_deployment.go:39-42`) accepts ONLY the literal `"all-targets"` for this MVP. The `useCreateDeployment.ts` dialog's strategy picker (`rolling`/`blue-green`/`canary`/`full`) maps to `rolling`/`blue_green`/`canary` — all of which are still rejected server-side with a 400. This is a **business-rule/value-domain** mismatch (the Go field is untyped `string`), not a JSON structural drift, so the wire *shape* is now correct but the deployment-creation flow will still 400 until the strategy picker is redesigned for the MVP's real capability. Documented in-code (`useCreateDeployment.ts`).
- **`CreateRolloutRequest` phase-plan validation**: the rollout-engine brick validates phase plans as strictly-increasing percentages ending at 100 (`handlers_rollout.go:80-81` comment). `useEvaluateRollout.ts`'s single-percentage panel now builds a technically-correct `{phases: [...]}` wire shape, but a submission below 100% may still be rejected as an invalid phase plan — a real phase-plan builder UI is a separate, out-of-scope work item. Documented in-code.

## Component/hook changes required by the corrected interfaces

| File | Change |
|---|---|
| `src/hooks/use-auth.ts` | `useLogin` now maps its UI-facing `LoginCredentials.email` onto the wire `LoginRequest.username` field at the request boundary (login form UX unchanged). |
| `src/hooks/useCreateDeployment.ts` | Drops fabricated `group_ids`/`rollout_percentage`/`staged`; builds `{release_id, strategy, group?}`. |
| `src/hooks/useCreateRelease.ts` | Drops fabricated `project_id`/`file_url`/`file_hash`/`changelog`/`target_board`/`firmware_version`; builds `{artifact_id, version, os, target_model, notes?, min_current_version?}`. |
| `src/hooks/useEvaluateRollout.ts` | Drops fabricated `deployment_id`/`groups`/`rollout_percentage`/`staged`; builds `{phases: [RolloutPhaseSpec]}` from the panel's single percentage value. |
| `src/hooks/useRecall.ts` | `RecallInput` gains a required `toReleaseId` field (was `force?: boolean`, dropped); builds `{to_release_id, reason?}`. |
| `src/hooks/useCreateGroup.ts` | Drops fabricated `device_ids: []`/`labels: {}`; builds `{name, description?}`. |
| `src/features/deployments/deployment-detail-page.tsx` | `RecallSection` gains a **new required "Target release ID" input** (`recallToReleaseId` state, threaded via new props) — previously there was no way for the UI to supply the server's mandatory `to_release_id`; the Recall button is now gated on this field being non-empty (previously gated only on the reason field). |
| `src/lib/api-client.ts` | 8 request-body interfaces fixed (`LoginRequest`, `DeviceRegistrationRequest`, `CreateReleaseRequest`, `CreateRolloutRequest`, `RecallRequest`, `CreateDeploymentRequest`, `CreateGroupRequest`, `UpdateGroupRequest`); 3 verified correct with citation comments added (`AddGroupMembersRequest`, `ArtifactUploadMetadata`, `DeltaRegisterRequest`); 1 verified-correct-but-out-of-scope noted (`DeltaFindParams`). |
| `src/__tests__/wire-shape-request-body.test.ts` | **New** compile-time-only guard test (`tsc --noEmit` oracle), mirroring the existing `wire-shape-pagination.test.ts`/`wire-shape-nonlist.test.ts` pattern — asserts every corrected interface against a real-server-shaped fixture and pins every enumerated OLD fabricated shape as a permanent `@ts-expect-error`. |

No response interfaces were touched (`ReleaseResponse`, `Deployment`, `DeploymentStatus`, `RolloutState`, `RolloutDecision`, `RollbackView`, `Group`, `DeploymentResponse`, etc. — untouched, per strict scope). `DeploymentResponse` (a fabricated response shape: `{id, group_ids, rollout_percentage, staged, created_by}`) is confirmed unused by any live caller (grep: only declared + re-exported) — left as-is, out of scope.

## §11.4.115 RED → GREEN proof (RecallRequest)

### RED (drift reintroduced, real Go struct unchanged)

`RecallRequest` in `src/lib/api-client.ts` was temporarily reverted to the OLD fabricated shape while `src/hooks/useRecall.ts` kept the corrected `{to_release_id, reason}` body construction:

```ts
// TEMPORARILY REINTRODUCED for RED capture:
export interface RecallRequest {
  reason: string;
  force: boolean;
}
```

Command: `npx tsc --noEmit`

Real captured output:

```
src/hooks/useRecall.ts(26,9): error TS2322: Type 'string | undefined' is not assignable to type 'string'.
  Type 'undefined' is not assignable to type 'string'.
```

This proves the fix is load-bearing: the corrected `useRecall.ts` body (`to_release_id`, `reason`) is structurally incompatible with the old drifted interface — exactly the defect class this audit closes (the real server would have rejected every recall the OLD client shape sent, per `handleRecall`'s `req.ToReleaseID == ""` check, `server/internal/api/handlers_recall.go:68-72`).

### GREEN (fix restored)

`RecallRequest` restored to:

```ts
export interface RecallRequest {
  to_release_id: string;
  reason?: string;
}
```

Command: `npx tsc --noEmit`

Real captured output: **(empty — exit 0)**

## Final verification (post-fix, full suite)

```
$ npx tsc --noEmit
(no output — exit 0)

$ npx vitest run
 RUN  v4.1.9 /home/milos/Factory/projects/tools_and_research/helix_ota/clients/ota-manager

 Test Files  14 passed (14)
      Tests  52 passed (52)
   Start at  04:36:47
   Duration  9.23s (transform 273ms, setup 417ms, import 1.31s, tests 943ms, environment 5.38s)
```

## Files changed

- `clients/ota-manager/src/lib/api-client.ts`
- `clients/ota-manager/src/hooks/use-auth.ts`
- `clients/ota-manager/src/hooks/useCreateDeployment.ts`
- `clients/ota-manager/src/hooks/useCreateRelease.ts`
- `clients/ota-manager/src/hooks/useEvaluateRollout.ts`
- `clients/ota-manager/src/hooks/useRecall.ts`
- `clients/ota-manager/src/hooks/useCreateGroup.ts`
- `clients/ota-manager/src/features/deployments/deployment-detail-page.tsx`
- `clients/ota-manager/src/__tests__/wire-shape-request-body.test.ts` (new)
