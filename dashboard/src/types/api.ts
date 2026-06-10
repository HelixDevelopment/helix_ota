// Helix OTA — hand-kept TypeScript wire types mirroring the /api/v1 protocol DTOs.
// Source of truth is docs/research/main_specs/1.0.0-mvp/api/openapi.yaml (design §3.1).
// Identifiers are opaque server-issued strings — never parsed or minted client-side (design §2).

export type ReleaseId = string;
export type DeploymentId = string;
export type DeviceId = string;
export type ArtifactId = string;

// --- auth (design §7.1, endpoints §7) ---------------------------------------

export type Role = "admin" | "operator" | "viewer" | "device";

export interface LoginRequest {
  username: string;
  password: string;
}

export interface TokenResponse {
  access_token: string;
  token_type: "Bearer";
  expires_in: number; // seconds, ~900
  refresh_token: string;
  roles: Role[];
}

export interface RefreshRequest {
  refresh_token: string;
}

// Decoded JWT access-token claims the dashboard reads (design §7.1).
export interface AccessClaims {
  sub: string;
  roles: Role[];
  exp: number; // epoch seconds
}

// --- artifacts (design §9.2, endpoints §9.1) --------------------------------

export interface ArtifactUploadMetadata {
  sha256: string; // lowercase hex
  signature: string; // base64 detached
  version: string;
  os: string;
  target_model: string;
  // AOSP A/B streaming fields.
  file_hash: string;
  file_size: number;
  metadata_hash: string;
  metadata_size: number;
  payload_offset: number;
  payload_size: number;
}

export interface Artifact {
  artifact_id: ArtifactId;
  sha256: string;
  version: string;
  os: string;
  target_model: string;
  verified: boolean;
  storage_ref: string;
  created_at: string; // RFC 3339 UTC
}

// --- releases (design §9.3, endpoints §10) ----------------------------------

export interface ReleaseCreate {
  artifact_id: ArtifactId;
  version: string;
  os: string;
  target_model: string;
  notes?: string;
  min_current_version?: string;
}

export interface Release {
  release_id: ReleaseId;
  artifact_id: ArtifactId;
  version: string;
  os: string;
  target_model: string;
  status: string;
  notes?: string;
  min_current_version?: string;
  created_at: string;
}

export interface ReleaseList {
  items: Release[];
  next_cursor?: string;
}

// --- deployments (design §9.3, endpoints §11) -------------------------------

export type DeploymentStrategy = "all-targets";

export interface DeploymentCreate {
  release_id: ReleaseId;
  strategy: DeploymentStrategy; // locked to all-targets for MVP
  group?: string;
}

export interface Deployment {
  deployment_id: DeploymentId;
  release_id: ReleaseId;
  strategy: DeploymentStrategy;
  group?: string;
  created_at: string;
}

export interface DeploymentProgress {
  pending: number;
  downloading: number;
  installed: number;
  succeeded: number;
  failed: number;
}

export interface DeploymentStatus {
  deployment_id: DeploymentId;
  release_id: ReleaseId;
  strategy: DeploymentStrategy;
  status: string;
  progress: DeploymentProgress;
  created_at: string;
}

// Rollout (staged-rollout API, 1.0.1). Phase-based model mirroring the server
// wire (handlers_rollout.go): a rollout is a strictly-increasing list of phases
// (percentage ending at 100, thresholds in [0,1]) advanced by health verdicts.
// GET returns the current state; POST creates+starts; POST /evaluate applies a
// health verdict and returns the engine decision. Routes degrade gracefully
// when the deployment has no rollout yet (404).
export interface RolloutPhaseSpec {
  percentage: number; // 1..100, strictly increasing, last == 100
  success_threshold: number; // [0,1]
  error_threshold: number; // [0,1]
  duration_seconds: number;
  auto_progress: boolean;
}

export interface RolloutCreate {
  phases: RolloutPhaseSpec[];
}

export interface RolloutState {
  deployment_id: DeploymentId;
  status: string; // e.g. running, halted, completed, rolled_back
  current_phase: number; // 0-based index into phases
  phases: RolloutPhaseSpec[];
  updated_at: string;
}

// POST /deployments/{id}/rollout/evaluate body — telemetry-derived health
// summary for the current phase cohort.
export interface RolloutVerdict {
  success_rate: number; // [0,1]
  error_rate: number; // [0,1]
  post_boot_health_failed: boolean;
}

// Response to an evaluation: the engine decision + the resulting state.
export interface RolloutDecision {
  action: string; // advance | hold | halt | complete (engine vocabulary)
  reason: string;
  state: RolloutState;
}

// --- recall / rollback (rollback_ux.md §7) ----------------------------------

// POST /deployments/{id}/recall body — server-driven rollback of the current
// release back to a previous-good release (forward-fix supersede semantics).
export interface RecallRequest {
  to_release_id: ReleaseId;
  reason?: string;
}

// A rollback_history row (response to recall + GET /deployments/{id}/rollbacks).
export interface RollbackRecord {
  id: string;
  deployment_id: DeploymentId;
  kind: string; // rollback | abort
  from_release_id?: string;
  to_release_id?: string;
  recall_deployment_id?: string;
  reason?: string;
  triggered_by?: string;
  details?: Record<string, string>;
  created_at: string;
}

export interface RollbackList {
  items: RollbackRecord[];
}

// --- devices + telemetry (design §9.4, endpoints §8 / §12) ------------------

export type UpdateState =
  | "idle"
  | "download_started"
  | "installing"
  | "installed"
  | "verifying"
  | "success"
  | "failure";

export interface DeviceStatus {
  device_id: DeviceId;
  current_version: string;
  target_version?: string;
  last_seen: string;
  update_state: UpdateState;
  active_slot?: string;
  health?: string;
}

export type TelemetryEventType = Exclude<UpdateState, "idle">;

// One telemetry-history row (handlers_telemetry.go TelemetryEventView). `event`
// is the wire event-type string; the extra projection fields are optional.
export interface TelemetryEventView {
  event: string;
  version?: string;
  deployment_id?: string;
  error_code?: string;
  detail?: string;
  timestamp: string; // device-reported time, RFC 3339
  received_at: string; // server ingest time, RFC 3339
}

// GET /devices/{id}/telemetry — newest-first, cursor-paginated. next_cursor is
// null on the last page.
export interface TelemetryHistory {
  device_id: DeviceId;
  items: TelemetryEventView[];
  next_cursor: string | null;
}

// GET /telemetry/overview — fleet-wide aggregates (handlers_telemetry.go).
//  - event_counts: count keyed by telemetry event type
//  - failure_rate: failure / (success + failure) terminal outcomes, 0..1
//  - by_state: fleet device count keyed by last-known update state
export interface TelemetryOverview {
  event_counts: Record<string, number>;
  total: number;
  failure_rate: number;
  by_state: Record<string, number>;
}

// --- groups (design DeploymentCreate group; group CRUD is G5/1.0.1) ---------

export interface DeviceGroup {
  group_id: string;
  name: string;
  description?: string;
  member_count: number;
  created_at: string; // RFC 3339 UTC
}

export interface DeviceGroupCreate {
  name: string;
  description?: string;
}

// GET /groups returns { items } (no cursor pagination in MVP).
export interface DeviceGroupList {
  items: DeviceGroup[];
  next_cursor?: string;
}

// Batch member-add request/response for POST /groups/{id}/members.
// The wire moved from a single-device 204 to a batch 200 with a per-id
// disposition breakdown so the operator UI can report partial results.
export interface DeviceGroupMembersAdd {
  device_ids: DeviceId[];
}

export interface DeviceGroupMembersAddResult {
  added: DeviceId[];
  already_member: DeviceId[];
  not_found: DeviceId[];
}

// GET /groups/{id}/members — current membership snapshot.
// Wire change: the flat `device_ids: DeviceId[]` was replaced by an `items`
// array of per-member records carrying the membership timestamp.
export interface DeviceGroupMember {
  device_id: DeviceId;
  added_at: string;
}

// GET /groups/{id}/members is oldest-first and cursor-paginated; next_cursor is
// absent/null on the last page (server: handlers_group.go:handleListGroupMembers).
export interface DeviceGroupMembers {
  group_id: string;
  items: DeviceGroupMember[];
  next_cursor?: string | null;
}

// --- audit (operational_endpoints.md §4; admin-only GET /audit) -------------

// Who performed an audited action. `subject` is always set; `user_id` is empty
// when the actor did not resolve to a durable users row.
export interface AuditActor {
  user_id?: string;
  subject: string;
}

// One audit_logs row (audit_wire.go AuditLogEntry).
export interface AuditEntry {
  id: string;
  actor: AuditActor;
  action: string; // SCREAMING_SNAKE verb, e.g. RELEASE_CREATE
  resource_type: string;
  resource_id?: string;
  details?: Record<string, string>;
  ip_address?: string;
  user_agent?: string;
  created_at: string; // RFC 3339 UTC
}

export interface AuditList {
  items: AuditEntry[];
  next_cursor: string | null;
}

// Optional ?since/?until RFC3339 time bounds + ?action/?resource_type filters.
export interface AuditQuery {
  action?: string;
  resource_type?: string;
  since?: string; // RFC 3339
  until?: string; // RFC 3339
  limit?: number; // [1,200]
  cursor?: string;
}

// --- health (endpoints does not define a public /health in MVP; best-effort) -

export interface HealthStatus {
  status: string;
}

// --- error envelope (design §8, endpoints §6) -------------------------------

export interface ApiErrorDetail {
  field?: string;
  message: string;
}

export interface ApiErrorEnvelope {
  error: {
    code: string;
    message: string;
    request_id?: string;
    details?: ApiErrorDetail[];
  };
}
