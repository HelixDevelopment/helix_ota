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

// Rollout (design DeploymentsScreen rollout panel; staged rollout API is G7/1.0.1).
// The dashboard surfaces a read view + a guarded write call; the routes themselves
// are flagged UNVERIFIED (design §13) and degrade gracefully.
export interface RolloutState {
  deployment_id: DeploymentId;
  strategy: string;
  percentage: number; // 0..100
  paused: boolean;
  updated_at: string;
}

export interface RolloutCommand {
  percentage?: number;
  paused?: boolean;
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

export interface TelemetryEvent {
  device_id: DeviceId;
  event_type: TelemetryEventType;
  at: string;
  detail?: string;
}

// Telemetry read API is PARTIAL (gap G4); these shapes are best-effort and the
// screens render a graceful empty state when the endpoints are not yet built.
export interface TelemetryHistory {
  device_id: DeviceId;
  events: TelemetryEvent[];
}

export interface TelemetryOverview {
  total_devices: number;
  by_update_state: Partial<Record<UpdateState, number>>;
  by_version: Record<string, number>;
}

// --- groups (design DeploymentCreate group; group CRUD is G5/1.0.1) ---------

export interface DeviceGroup {
  group_id: string;
  name: string;
  member_count: number;
  created_at: string; // RFC 3339 UTC
}

export interface DeviceGroupCreate {
  name: string;
}

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
export interface DeviceGroupMembers {
  group_id: string;
  device_ids: DeviceId[];
}

// --- audit (design §6; audit viewer is G3/1.0.1, route deferred) ------------

export interface AuditActor {
  user_id?: string;
  subject: string;
}

export interface AuditEntry {
  id: string;
  actor: AuditActor;
  action: string;
  at: string;
  request_id?: string;
}

export interface AuditList {
  items: AuditEntry[];
  next_cursor?: string;
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
