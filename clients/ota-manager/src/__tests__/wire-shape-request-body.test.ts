import { describe, it, expect } from "vitest";
import type {
  LoginRequest,
  DeviceRegistrationRequest,
  CreateReleaseRequest,
  CreateRolloutRequest,
  RolloutPhaseSpec,
  RecallRequest,
  CreateDeploymentRequest,
  CreateGroupRequest,
  UpdateGroupRequest,
  AddGroupMembersRequest,
  ArtifactUploadMetadata,
  DeltaRegisterRequest,
} from "@/lib/api-client";

// §11.4.108/§11.4.115/§11.4.118 regression guard — client/server wire-shape
// drift on REQUEST-BODY interfaces (docs/qa/20260710-client-request-body-
// audit/EVIDENCE.md).
//
// Every interface asserted below was FABRICATED (or missing a real required
// field) before this fix — verified field-by-field against the real Go
// request structs the corresponding handler decodes (file:line citations in
// api-client.ts, next to each interface). This test function only
// TYPE-CHECKS (never executed at runtime — `tsc --noEmit` is the oracle),
// mirroring `wire-shape-pagination.test.ts` / `wire-shape-nonlist.test.ts`'s
// pattern for the response interfaces.
//
// RED (pre-fix, captured 2026-07-10 — see EVIDENCE.md for the full
// transcript): temporarily reverting `RecallRequest` to the OLD fabricated
// `{ reason: string; force: boolean }` shape (while `useRecall.ts` built the
// corrected `{ to_release_id, reason }` body) and running `npx tsc --noEmit`
// reproduced:
//   src/hooks/useRecall.ts(26,9): error TS2322: Type 'string | undefined' is
//   not assignable to type 'string'.
//     Type 'undefined' is not assignable to type 'string'.
// (the mirror-image of the GREEN assertion below, where the real
// `to_release_id`-shaped fixture type-checks and the OLD `{reason, force}`
// shape is a permanent compile error via `// @ts-expect-error`.)
//
// GREEN (post-fix): every fixture below — built strictly from the REAL Go
// wire request structs — type-checks against the corrected client
// interfaces, and the enumerated OLD fabricated shapes are permanent compile
// errors via `// @ts-expect-error`.
function assertRealServerRequestShapesTypeCheck(): void {
  // LoginRequest — server/internal/api/wire.go:19-22.
  const loginRequest: LoginRequest = { username: "operator", password: "hunter2" };
  void loginRequest;

  // @ts-expect-error — the OLD fabricated LoginRequest shape ({email,
  // password}); the server has no `email` field, only `username`.
  const loginRequestFabricated: LoginRequest = { email: "operator@example.com", password: "hunter2" };
  void loginRequestFabricated;

  // DeviceRegistrationRequest — server/internal/api/wire.go:41-49
  // (DeviceRegistration struct).
  const deviceRegistrationRequest: DeviceRegistrationRequest = {
    hardware_id: "hw-1",
    model: "Orange Pi 5 Max",
    os: "android",
  };
  void deviceRegistrationRequest;

  // @ts-expect-error — the OLD fabricated DeviceRegistrationRequest shape
  // ({device_id, board, firmware_version, hardware_revision, serial_number});
  // none of these fields exist on the real request, and the real required
  // `hardware_id`/`model`/`os` are missing here.
  const deviceRegistrationRequestFabricated: DeviceRegistrationRequest = { device_id: "dev-1", board: "orangepi", firmware_version: "1.0.0", hardware_revision: "rev-a", serial_number: "sn-1" };
  void deviceRegistrationRequestFabricated;

  // CreateReleaseRequest — server/internal/api/wire.go:137-144 (ReleaseCreate
  // struct).
  const createReleaseRequest: CreateReleaseRequest = {
    artifact_id: "art-1",
    version: "1.0.0",
    os: "android",
    target_model: "Orange Pi 5 Max",
  };
  void createReleaseRequest;

  // @ts-expect-error — the OLD fabricated CreateReleaseRequest shape
  // ({project_id, file_url, file_hash, changelog, target_board,
  // firmware_version}); none of these fields exist on the real request, and
  // the real required `artifact_id`/`target_model` are missing here.
  const createReleaseRequestFabricated: CreateReleaseRequest = { project_id: "proj-1", version: "1.0.0", file_url: "https://example/x.zip", file_hash: "abc", changelog: "notes", target_board: "orangepi", firmware_version: "android14" };
  void createReleaseRequestFabricated;

  // CreateRolloutRequest — server/internal/api/handlers_rollout.go:24-27
  // (RolloutCreate struct): `{phases}` ONLY (deployment id is a URL param).
  const rolloutPhase: RolloutPhaseSpec = {
    percentage: 50,
    success_threshold: 0.95,
    error_threshold: 0.05,
    duration_seconds: 300,
    auto_progress: false,
  };
  const createRolloutRequest: CreateRolloutRequest = { phases: [rolloutPhase] };
  void createRolloutRequest;

  // @ts-expect-error — the OLD fabricated CreateRolloutRequest shape
  // ({deployment_id, groups, rollout_percentage, staged}); none of these
  // fields exist on the real request, and the real required `phases` array
  // is missing here.
  const createRolloutRequestFabricated: CreateRolloutRequest = { deployment_id: "dep-1", groups: ["grp-1"], rollout_percentage: 50, staged: false };
  void createRolloutRequestFabricated;

  // RecallRequest — server/internal/api/handlers_recall.go:17-20.
  const recallRequest: RecallRequest = { to_release_id: "rel-1" };
  void recallRequest;

  // @ts-expect-error — the OLD fabricated RecallRequest shape ({reason,
  // force}); the server has no `force` field, and the real required
  // `to_release_id` is missing here.
  const recallRequestFabricated: RecallRequest = { reason: "bug", force: true };
  void recallRequestFabricated;

  // CreateDeploymentRequest — server/internal/api/wire.go:167-171
  // (DeploymentCreate struct): `{release_id, strategy, group?}` — a SINGLE
  // optional group name, never a `group_ids` array.
  const createDeploymentRequest: CreateDeploymentRequest = {
    release_id: "rel-1",
    strategy: "all-targets",
    group: "grp-1",
  };
  void createDeploymentRequest;

  // @ts-expect-error — the OLD fabricated CreateDeploymentRequest shape
  // ({group_ids, rollout_percentage, staged}); the real wire has a single
  // optional `group` string, never these fields.
  const createDeploymentRequestFabricated: CreateDeploymentRequest = { release_id: "rel-1", group_ids: ["grp-1"], strategy: "rolling", rollout_percentage: 100, staged: false };
  void createDeploymentRequestFabricated;

  // CreateGroupRequest — server/internal/api/handlers_group.go:42-45
  // (GroupCreate struct): `{name, description?}` ONLY.
  const createGroupRequest: CreateGroupRequest = { name: "Production" };
  void createGroupRequest;

  // @ts-expect-error — the OLD fabricated CreateGroupRequest shape
  // ({device_ids, labels}); groups have no membership-at-creation or
  // `labels` concept on the real request.
  const createGroupRequestFabricated: CreateGroupRequest = { name: "Production", description: "", device_ids: ["dev-1"], labels: { env: "prod" } };
  void createGroupRequestFabricated;

  // UpdateGroupRequest — server/internal/api/handlers_group.go:48-51
  // (GroupUpdate struct): `{name, description?}` ONLY — no `labels`.
  const updateGroupRequest: UpdateGroupRequest = { name: "Renamed" };
  void updateGroupRequest;

  // @ts-expect-error — the OLD fabricated UpdateGroupRequest shape carrying
  // `labels`; groups have no `labels` concept anywhere on the real server.
  const updateGroupRequestFabricated: UpdateGroupRequest = { name: "Renamed", labels: { env: "prod" } };
  void updateGroupRequestFabricated;

  // AddGroupMembersRequest — VERIFIED CORRECT, server/internal/api/
  // handlers_group.go:71-73 (MemberAdd struct).
  const addGroupMembersRequest: AddGroupMembersRequest = { device_ids: ["dev-1", "dev-2"] };
  void addGroupMembersRequest;

  // ArtifactUploadMetadata — VERIFIED CORRECT, server/internal/api/
  // wire.go:107-119.
  const artifactUploadMetadata: ArtifactUploadMetadata = {
    sha256: "abc123",
    signature: "sig",
    version: "1.0.0",
    os: "android",
    target_model: "Orange Pi 5 Max",
  };
  void artifactUploadMetadata;

  // DeltaRegisterRequest — VERIFIED CORRECT, server/internal/api/
  // handlers_delta.go:15-21.
  const deltaRegisterRequest: DeltaRegisterRequest = {
    base_artifact_id: "art-1",
    target_artifact_id: "art-2",
  };
  void deltaRegisterRequest;
}
void assertRealServerRequestShapesTypeCheck;

describe("request-body wire-shape reconciliation (client interfaces match the real Go server)", () => {
  it("is a compile-time-only guard — tsc --noEmit is the oracle (see file header for RED/GREEN evidence)", () => {
    // This test body is intentionally a no-op at runtime: the assertion this
    // file makes is entirely in the type-level function above, checked by
    // `npx tsc --noEmit`. A single trivial runtime assertion keeps vitest's
    // "no tests in file" failure mode from masking a deleted guard.
    expect(true).toBe(true);
  });
});
