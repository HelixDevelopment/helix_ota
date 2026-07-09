import { describe, it, expect } from "vitest";
import type {
  TokenResponse,
  DeviceRegistered,
  DeviceListItem,
  DeviceList,
  DeviceStatus,
  DeviceHealth,
  ReleaseResponse,
  Deployment,
  DeploymentStatus,
  RolloutState,
  RolloutDecision,
  Project,
  Group,
  GroupMembers,
  AuditEntry,
  TelemetryHistory,
} from "@/lib/api-client";

// §11.4.108/§11.4.115/§11.4.118 regression guard — client/server wire-shape
// drift on NON-LIST response interfaces (docs/qa/20260710-client-nonlist-
// wire-audit/EVIDENCE.md).
//
// Every interface asserted below was FABRICATED (or missing a real field)
// before this fix — verified field-by-field against the real Go structs
// (file:line citations in api-client.ts, next to each interface). This test
// function only TYPE-CHECKS (never executed at runtime — `tsc --noEmit` is
// the oracle), mirroring `wire-shape-pagination.test.ts`'s pattern for the
// list interfaces.
//
// RED (pre-fix, captured 2026-07-10 — see EVIDENCE.md for the full
// transcript): assigning any of the real-server-shaped fixtures below to the
// OLD client interfaces failed `tsc --noEmit` with `error TS2322` /
// `error TS2739` (missing required properties) / `error TS2353` (unknown
// properties), one error per interface. Concretely, reverting
// `TelemetryHistory` to omit `device_id` (the specific field the conductor
// briefing named) and running `npx tsc --noEmit` reproduced:
//   error TS2741: Property 'device_id' is missing in type
//   '{ items: never[]; next_cursor: null; }' but required in type
//   'TelemetryHistory'.
// (the mirror-image of the GREEN assertion at the bottom of this file).
//
// GREEN (post-fix): every fixture below — built strictly from the REAL Go
// wire structs — type-checks against the corrected client interfaces, and
// the enumerated OLD fabricated shapes are permanent compile errors via
// `// @ts-expect-error`.
function assertRealServerShapesTypeCheck(): void {
  // TokenResponse — server/internal/api/wire.go:29-36.
  const tokenResponse: TokenResponse = {
    access_token: "a",
    refresh_token: "r",
    expires_in: 3600,
    token_type: "Bearer",
    roles: ["admin"],
  };
  void tokenResponse;

  // DeviceRegistered — server/internal/api/wire.go:52-59.
  const deviceRegistered: DeviceRegistered = {
    device_id: "dev-1",
    hardware_id: "hw-1",
    device_token: "tok",
    token_type: "Bearer",
    expires_in: 3600,
    registered_at: "2026-07-10T00:00:00Z",
  };
  void deviceRegistered;

  // DeviceListItem — server/internal/api/wire.go:81-95 — and DeviceList's
  // real item type (previously typed as DeviceRegistered).
  const deviceListItem: DeviceListItem = {
    device_id: "dev-1",
    hardware_id: "hw-1",
    model: "Orange Pi 5 Max",
    os: "android",
    target_version: null,
    update_state: "idle",
    health_ok: true,
    registered_at: "2026-07-10T00:00:00Z",
  };
  const deviceList: DeviceList = { items: [deviceListItem], next_cursor: null };
  void deviceList;

  // DeviceHealth (nested) + DeviceStatus — server/internal/api/wire.go:62-78.
  const deviceHealth: DeviceHealth = { ok: true, last_error_code: null };
  const deviceStatus: DeviceStatus = {
    device_id: "dev-1",
    hardware_id: "hw-1",
    target_version: null,
    update_state: "idle",
    health: deviceHealth,
  };
  void deviceStatus;

  // ReleaseResponse — server/internal/api/wire.go:147-156.
  const release: ReleaseResponse = {
    release_id: "rel-1",
    artifact_id: "art-1",
    version: "1.0.0",
    os: "android",
    target_model: "Orange Pi 5 Max",
    status: "published",
    created_at: "2026-07-10T00:00:00Z",
  };
  void release;

  // Deployment + DeploymentStatus — server/internal/api/wire.go:174-205.
  const deployment: Deployment = {
    deployment_id: "dep-1",
    release_id: "rel-1",
    strategy: "all-targets",
    status: "active",
    target_count: 10,
    created_at: "2026-07-10T00:00:00Z",
  };
  const deploymentStatus: DeploymentStatus = {
    ...deployment,
    progress: { pending: 1, downloading: 2, installed: 3, succeeded: 4, failed: 0 },
  };
  void deploymentStatus;

  // RolloutState + RolloutDecision — server/internal/api/handlers_rollout.go:16-51.
  const rolloutState: RolloutState = {
    deployment_id: "dep-1",
    status: "in_progress",
    current_phase: 0,
    phases: [
      { percentage: 10, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 300, auto_progress: true },
    ],
    updated_at: "2026-07-10T00:00:00Z",
  };
  const rolloutDecision: RolloutDecision = { action: "advance", reason: "healthy", state: rolloutState };
  void rolloutDecision;

  // Project — server/internal/api/wire.go:260-266.
  const project: Project = {
    project_id: "proj-1",
    name: "Default",
    created_at: "2026-07-10T00:00:00Z",
    updated_at: "2026-07-10T00:00:00Z",
  };
  void project;

  // Group + GroupMembers — server/internal/api/handlers_group.go:54-95.
  const group: Group = {
    group_id: "grp-1",
    name: "Production",
    member_count: 3,
    created_at: "2026-07-10T00:00:00Z",
  };
  const groupMembers: GroupMembers = {
    group_id: "grp-1",
    items: [{ device_id: "dev-1", added_at: "2026-07-10T00:00:00Z" }],
    next_cursor: null,
  };
  void group;
  void groupMembers;

  // AuditEntry — server/internal/api/audit_wire.go:9-31.
  const auditEntry: AuditEntry = {
    id: "audit-1",
    actor: { subject: "user-1" },
    action: "DEVICE_CREATE",
    resource_type: "device",
    created_at: "2026-07-10T00:00:00Z",
  };
  void auditEntry;

  // TelemetryHistory — server/internal/api/handlers_telemetry.go:15-38. The
  // conductor-flagged specific omission: `device_id` is a REQUIRED top-level
  // field (§11.4.6/§11.4.108).
  const telemetryHistory: TelemetryHistory = {
    device_id: "dev-1",
    items: [{ event: "success", timestamp: "2026-07-10T00:00:00Z", received_at: "2026-07-10T00:00:01Z" }],
    next_cursor: null,
  };
  void telemetryHistory;

  // @ts-expect-error — `device_id` is REQUIRED on the real TelemetryHistory
  // wire shape; if this stops erroring, the field has been dropped again.
  const telemetryHistoryMissingDeviceId: TelemetryHistory = { items: [], next_cursor: null };
  void telemetryHistoryMissingDeviceId;

  // @ts-expect-error — the OLD fabricated DeviceRegistered shape ({id, board,
  // status, created_at}); none of these fields exist on the real wire.
  const deviceRegisteredFabricated: DeviceRegistered = { id: "x", device_id: "dev-1", board: "orangepi", status: "active", created_at: "2026-07-10T00:00:00Z" };
  void deviceRegisteredFabricated;

  // @ts-expect-error — the OLD fabricated DeviceStatus shape ({online,
  // current_release_id, ip_address}); none of these fields exist on the real
  // wire, and the real required `health` block is missing here.
  const deviceStatusFabricated: DeviceStatus = { device_id: "dev-1", online: true, current_release_id: "rel-1", last_seen: "2026-07-10T00:00:00Z", ip_address: "10.0.0.1" };
  void deviceStatusFabricated;

  // @ts-expect-error — the OLD fabricated Release shape ({project_id,
  // file_url, file_hash, changelog, target_board, firmware_version,
  // created_by}); none of these fields exist on the real wire, and the real
  // required `artifact_id`/`target_model` are missing here.
  const releaseFabricated: ReleaseResponse = { id: "rel-1", project_id: "proj-1", version: "1.0.0", file_url: "https://example/x.zip", file_hash: "abc", changelog: "notes", target_board: "orangepi", firmware_version: "android14", status: "active", created_at: "2026-07-10T00:00:00Z", created_by: "user-1" };
  void releaseFabricated;

  // @ts-expect-error — the OLD fabricated Deployment shape ({group_ids,
  // rollout_percentage, staged, created_by}); the real wire has a single
  // optional `group` string and a `target_count`, never these fields.
  const deploymentFabricated: Deployment = { deployment_id: "dep-1", release_id: "rel-1", group_ids: ["grp-1"], strategy: "rolling", rollout_percentage: 100, staged: false, status: "active", created_at: "2026-07-10T00:00:00Z", created_by: "user-1" };
  void deploymentFabricated;

  // @ts-expect-error — the OLD fabricated RolloutDecision shape
  // ({approved: boolean}); the real wire sends `action`/`reason`/`state`.
  const rolloutDecisionFabricated: RolloutDecision = { approved: true, reason: "ok" };
  void rolloutDecisionFabricated;

  // @ts-expect-error — the OLD fabricated Group shape ({id, project_id,
  // device_count, labels}); the real primary key is `group_id`, the real
  // count field is `member_count`, and groups are not project-scoped.
  const groupFabricated: Group = { id: "grp-1", project_id: "proj-1", name: "Production", description: "", device_count: 3, labels: {}, created_at: "2026-07-10T00:00:00Z" };
  void groupFabricated;

  // @ts-expect-error — the OLD fabricated AuditEntry shape (`actor` as a bare
  // string); the real wire nests `{user_id?, subject}`.
  const auditEntryFabricated: AuditEntry = { id: "audit-1", actor: "user-1", action: "DEVICE_CREATE", resource_type: "device", resource_id: "dev-1", details: {}, ip_address: "10.0.0.1", created_at: "2026-07-10T00:00:00Z" };
  void auditEntryFabricated;
}
void assertRealServerShapesTypeCheck;

describe("non-list response wire-shape reconciliation (client interfaces match the real Go server)", () => {
  it("is a compile-time-only guard — tsc --noEmit is the oracle (see file header for RED/GREEN evidence)", () => {
    // This test body is intentionally a no-op at runtime: the assertion this
    // file makes is entirely in the type-level function above, checked by
    // `npx tsc --noEmit`. A single trivial runtime assertion keeps vitest's
    // "no tests in file" failure mode from masking a deleted guard.
    expect(true).toBe(true);
  });
});
