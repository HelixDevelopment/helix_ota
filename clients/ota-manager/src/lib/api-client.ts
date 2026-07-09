import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios";
import { useAuthStore } from "@/stores/auth-store";

declare global {
  interface ImportMeta {
    readonly env: ImportMetaEnv;
  }
}

interface ImportMetaEnv {
  readonly VITE_API_BASE_URL?: string;
}

// Same-origin by default: the SPA is served by the control plane itself, and the
// API lives under the server's APIBasePath (/api/v1). A RELATIVE base keeps every
// request same-origin so the Tier-C CSP `connect-src 'self'` (server security
// headers) does not block it — an absolute default (e.g. http://localhost:8080)
// would be off-origin at any real deployment and be CSP-blocked. Override with
// VITE_API_BASE_URL only for a split-origin dev setup (must also be allowed by CSP).
const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "/api/v1";

export const apiClient = axios.create({
  baseURL: BASE_URL,
  timeout: 30_000,
  headers: { "Content-Type": "application/json" },
});

function authRequestInterceptor(
  config: InternalAxiosRequestConfig,
): InternalAxiosRequestConfig {
  const token = useAuthStore.getState().token;
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
}

function authResponseErrorInterceptor(error: AxiosError<ApiErrorResponse>) {
  if (error.response?.status === 401) {
    useAuthStore.getState().logout();
    window.location.href = "/login";
  }
  return Promise.reject(error);
}

apiClient.interceptors.request.use(authRequestInterceptor);
apiClient.interceptors.response.use(undefined, authResponseErrorInterceptor);

// -- Convenience wrappers used by hooks --

export async function apiGet<T>(url: string, config?: Record<string, unknown>): Promise<T> {
  const { data } = await apiClient.get<T>(url, config);
  return data;
}

export async function apiPost<T>(url: string, body?: unknown, config?: Record<string, unknown>): Promise<T> {
  const { data } = await apiClient.post<T>(url, body, config);
  return data;
}

export async function apiPatch<T>(url: string, body?: unknown, config?: Record<string, unknown>): Promise<T> {
  const { data } = await apiClient.patch<T>(url, body, config);
  return data;
}

export async function apiDelete<T>(url: string, config?: Record<string, unknown>): Promise<T> {
  const { data } = await apiClient.delete<T>(url, config);
  return data;
}

// Multipart POST — used by the artifact upload endpoint (multipart/form-data
// body: file + metadata + optional sha256/signature parts, per
// server/internal/api/handlers_artifact.go handleUploadArtifact).
export async function apiMultipartPost<T>(
  url: string,
  formData: FormData,
  config?: Record<string, unknown>,
): Promise<T> {
  const { headers, ...rest } = config ?? {};
  const { data } = await apiClient.post<T>(url, formData, {
    ...rest,
    headers: { "Content-Type": "multipart/form-data", ...(headers as Record<string, string> | undefined) },
  });
  return data;
}

// -- Types shared between this client and the Go server wire protocol --

export interface ApiErrorResponse {
  code: string;
  message: string;
  request_id: string;
  details?: Record<string, unknown>;
}

export interface PaginatedResponse<T> {
  data: T[];
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

// Auth
export interface LoginRequest {
  email: string;
  password: string;
}

// TokenResponse mirrors the REAL POST /auth/login + /auth/refresh wire shape
// — server/internal/api/wire.go:29-36 (TokenResponse struct): `{access_token,
// token_type, expires_in, refresh_token, roles}`. Roles is `[]string` with
// `omitempty` — the key is entirely ABSENT when the server issues no roles
// claim, never `null` (§11.4.6/§11.4.108: this previously omitted the
// `roles` field the server actually sends).
export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  token_type: string;
  roles?: string[];
}

// Devices
//
// NOTE (§11.4.6 — out-of-scope discovery, not fixed in this pass): the REAL
// POST /devices/register request body — server/internal/api/wire.go:41-49
// (DeviceRegistration struct) — is `{hardware_id, model, os, os_version?,
// current_version?, group?, metadata?}`. This client-declared
// `DeviceRegistrationRequest` shape (`{device_id, board, firmware_version,
// hardware_revision, serial_number}`) matches none of the real request
// fields. REQUEST-body interfaces are out of scope for this response-shape
// audit (see EVIDENCE.md) — flagged here as a discovered, unfixed defect for
// a follow-up work item.
export interface DeviceRegistrationRequest {
  device_id: string;
  board: string;
  firmware_version: string;
  hardware_revision: string;
  serial_number: string;
}

// DeviceRegistered mirrors the REAL POST /devices/register (+ idempotent
// replay) wire shape — server/internal/api/wire.go:52-59 (DeviceRegistered
// struct) via toDeviceRegistered (handlers_device.go:192-202): `{device_id,
// hardware_id, device_token, token_type, expires_in, registered_at}`.
// (§11.4.6/§11.4.108: the previous `{id, device_id, board, status,
// created_at}` shape was fabricated — the server never sends `id`, `board`,
// or `status` on this response; `device_token`/`token_type`/`expires_in` —
// the freshly-minted device bearer credential — were entirely missing from
// the client type.)
export interface DeviceRegistered {
  device_id: string;
  hardware_id: string;
  device_token: string;
  token_type: string;
  expires_in: number;
  registered_at: string;
}

// DeviceListItem is the REAL per-item shape of GET /devices and GET
// /devices/by-hardware/{hardwareId} — server/internal/api/wire.go:81-95
// (DeviceListItem struct) via toDeviceListItem (handlers_device.go:167-190).
// TargetVersion is `*string` with NO `omitempty` — the key is always
// present, `null` when unset. LastSeen is `*time.Time` WITH `omitempty` —
// the key is entirely absent until the device has reported once.
// OSVersion/CurrentVersion/Group/ActiveSlot are plain `string,omitempty` —
// absent when empty. (§11.4.6/§11.4.108: DeviceList.items previously typed
// its entries as `DeviceRegistered` — the one-time registration-response
// shape — instead of this real per-item list shape; none of
// DeviceRegistered's fields exist on this response.)
export interface DeviceListItem {
  device_id: string;
  hardware_id: string;
  model: string;
  os: string;
  os_version?: string;
  current_version?: string;
  target_version: string | null;
  group?: string;
  update_state: string;
  active_slot?: string;
  health_ok: boolean;
  last_seen?: string;
  registered_at: string;
}

export interface DeviceRegistration extends DeviceRegistrationRequest {
  id?: string;
}

export interface DeviceListFilter {
  group?: string;
  os?: string;
  model?: string;
  status?: string;
  cursor?: string;
  limit?: number;
}

// DeviceList mirrors the REAL GET /devices wire shape — server/internal/api/
// wire.go:97-100 (DeviceList struct): `{items, next_cursor}`, cursor-paginated.
// NextCursor is a `*string` with NO `omitempty` tag, so the key is ALWAYS
// present on the wire — `null` on the last page, a string cursor otherwise
// (§11.4.6/§11.4.108: this previously declared a fabricated `{total, cursor}`
// shape the server never sends — both fields were always `undefined` at
// runtime).
export interface DeviceList {
  items: DeviceListItem[];
  next_cursor: string | null;
}

// DeviceHealth is the REAL nested health block of DeviceStatus —
// server/internal/api/wire.go:62-65 (DeviceHealth struct): `{ok,
// last_error_code}`. LastErrorCode is `*string` with NO `omitempty` — the
// key is always present, `null` when the device has no recorded error.
// (§11.4.6/§11.4.108: the previous top-level `{device_id, online,
// battery_percent, storage_used_percent, temperature_celsius,
// uptime_seconds, last_reported}` shape was entirely fabricated — the server
// has no such standalone health endpoint or field set; this block only ever
// nests inside DeviceStatus.)
export interface DeviceHealth {
  ok: boolean;
  last_error_code: string | null;
}

// DeviceStatus mirrors the REAL GET /devices/{id}/status wire shape —
// server/internal/api/wire.go:69-78 (DeviceStatus struct) via toDeviceStatus
// (handlers_device.go:204-230). TargetVersion is `*string` with NO
// `omitempty` — always present, `null` when unset. LastSeen is
// `*time.Time` WITH `omitempty` — absent until the device has reported once.
// CurrentVersion/ActiveSlot are `string,omitempty` — absent when empty.
// (§11.4.6/§11.4.108: the previous `{device_id, online, current_release_id,
// last_seen, ip_address}` shape was fabricated — none of `online`,
// `current_release_id`, or `ip_address` exist on the real wire.)
export interface DeviceStatus {
  device_id: string;
  hardware_id: string;
  current_version?: string;
  target_version: string | null;
  last_seen?: string;
  update_state: string;
  active_slot?: string;
  health: DeviceHealth;
}

// Telemetry
export interface TelemetryFilter {
  event?: string;
  since?: string;
  until?: string;
  cursor?: string;
  limit?: number;
}

// TelemetryEventView is the REAL per-item shape of TelemetryHistory.items —
// server/internal/api/handlers_telemetry.go:15-28 (TelemetryEventView
// struct) via toTelemetryView (:66-78). Version/DeploymentID/ErrorCode/Detail
// are `string,omitempty` — absent when empty; DurationMS/BytesTransferred
// are `*int64,omitempty` — absent when the device never reported them.
export interface TelemetryEventView {
  event: string;
  version?: string;
  deployment_id?: string;
  error_code?: string;
  detail?: string;
  timestamp: string;
  received_at: string;
  duration_ms?: number;
  bytes_transferred?: number;
}

// TelemetryHistory mirrors the REAL GET /devices/{id}/telemetry wire shape —
// server/internal/api/handlers_telemetry.go:30-38 (TelemetryHistory struct):
// `{device_id, items, next_cursor}`, cursor-paginated. NextCursor is a
// `*string` with NO `omitempty` tag, so the key is ALWAYS present — `null` on
// the last page (§11.4.6/§11.4.108: this previously declared a fabricated
// `{total, cursor}` shape the server never sends AND omitted the real
// `device_id` field + typed `items` as an opaque `Record<string, unknown>[]`
// instead of the real TelemetryEventView[] shape).
export interface TelemetryHistory {
  device_id: string;
  items: TelemetryEventView[];
  next_cursor: string | null;
}

// TelemetryOverview mirrors the REAL GET /telemetry/overview wire shape —
// server/internal/api/handlers_telemetry.go:56-64 (TelemetryOverview struct):
// fleet-wide event counts by type, the running total, the terminal
// failure rate, and the fleet device count keyed by last-known update state.
// (§11.4.6 / BUG-1 fix: this previously declared a fabricated camelCase shape
// — {totalDevices, activeDeployments, pendingUpdates, failedDevices} — that
// the real server never sends; every dashboard stat card silently read
// `undefined` at runtime. The dashboard-facing camelCase view model now lives
// in `useTelemetryOverview.ts` as `TelemetryOverviewView`, mapped from this
// real shape.)
export interface TelemetryOverview {
  event_counts: Record<string, number>;
  total: number;
  failure_rate: number;
  by_state: Record<string, number>;
}

// Releases
export type Role = 'viewer' | 'operator' | 'admin' | 'device' | 'super_admin';

export interface ProjectAccess {
  project_id: string;
  role: Role;
}

export interface CreateReleaseRequest {
  project_id: string;
  version: string;
  file_url: string;
  file_hash: string;
  changelog: string;
  target_board: string;
  firmware_version: string;
}

// ReleaseResponse mirrors the REAL release wire shape — server/internal/api/
// wire.go:147-156 (Release struct) via toRelease (handlers_release.go:126-138),
// returned by POST /releases, GET /releases/{id}, and (as list items) GET
// /releases. Notes is `string,omitempty` — absent when empty.
// (§11.4.6/§11.4.108: the previous `{id, release_id?, project_id, version,
// file_url, file_hash, changelog, target_board, firmware_version, status,
// created_at, created_by}` shape was almost entirely fabricated — the server
// has no `project_id`/`file_url`/`file_hash`/`changelog`/`target_board`/
// `firmware_version`/`created_by` field on this response, and the real
// primary key is `release_id`, never `id`. NOTE (out-of-scope discovery):
// `CreateReleaseRequest` — the POST /releases request body this client sends
// — is a SEPARATELY drifted interface (real ReleaseCreate is
// `{artifact_id, version, os, target_model, notes?, min_current_version?}`);
// REQUEST-body interfaces are out of scope for this response-shape audit —
// see EVIDENCE.md.)
export interface ReleaseResponse {
  release_id: string;
  artifact_id: string;
  version: string;
  os: string;
  target_model: string;
  status: string;
  notes?: string;
  created_at: string;
}

export type Release = ReleaseResponse;

export interface ReleaseFilter {
  os?: string;
  target_model?: string;
  status?: string;
  cursor?: string;
  limit?: number;
}

// ReleaseList mirrors the REAL GET /releases wire shape — server/internal/api/
// wire.go:159-162 (ReleaseList struct): `{items, next_cursor}`, cursor-
// paginated. NextCursor is a `*string` with NO `omitempty` tag, so the key is
// ALWAYS present — `null` on the last page (§11.4.6/§11.4.108: this previously
// declared a fabricated `{total, cursor}` shape the server never sends).
export interface ReleaseList {
  items: Release[];
  next_cursor: string | null;
}

// Deployments
//
// Deployment mirrors the REAL deployment wire shape — server/internal/api/
// wire.go:174-182 (Deployment struct) via toDeployment
// (handlers_deployment.go:226-237), returned by POST /deployments and (as
// list items) GET /deployments. Group is `string,omitempty` (a single
// optional target-group name — the MVP has no multi-group `group_ids`
// array). (§11.4.6/§11.4.108: the previous `{..., group_ids: string[],
// rollout_percentage, staged, ..., created_by}` shape was fabricated — the
// server has no `group_ids` array (only a single optional `group` string),
// no `rollout_percentage`/`staged` on Deployment itself (those concepts live
// on RolloutState for a staged rollout), no `created_by`; `target_count` —
// the real device-count field — was entirely missing from the client type.)
export interface Deployment {
  deployment_id: string;
  release_id: string;
  strategy: string;
  group?: string;
  status: string;
  target_count: number;
  created_at: string;
}

// DeploymentList mirrors the REAL GET /deployments wire shape —
// server/internal/api/wire.go:184-190 (DeploymentList struct): `{items,
// next_cursor}`. CORRECTION vs the original conductor briefing for this task:
// DeploymentList DOES carry a `next_cursor` field on the wire (a `*string`
// with NO `omitempty` tag, so the key is ALWAYS present) — it is NOT
// items-only. handleListDeployments (handlers_deployment.go:106-117) never
// sets it, so it is always `null` today (wire.go's own comment: "NextCursor is
// reserved for future pagination parity with ReleaseList; the MVP returns all
// active deployments in one page") — but the JSON key is still emitted as
// `"next_cursor":null`, not omitted. Typed here as the real wire shape rather
// than dropping the field, so a future paginated /deployments needs no client
// type change. (§11.4.6/§11.4.108: this previously declared a fabricated
// `{total, cursor}` shape the server never sends.)
export interface DeploymentList {
  items: Deployment[];
  next_cursor: string | null;
}

// DeploymentProgress is the REAL aggregate progress block of
// DeploymentStatus — server/internal/api/wire.go:193-199 (DeploymentProgress
// struct), derived from telemetry by deriveProgress
// (handlers_deployment.go:190-224).
export interface DeploymentProgress {
  pending: number;
  downloading: number;
  installed: number;
  succeeded: number;
  failed: number;
}

// DeploymentStatus mirrors the REAL GET /deployments/{id} wire shape —
// server/internal/api/wire.go:202-205 (DeploymentStatus struct: an embedded
// Deployment + a `progress` block), returned by handleGetDeployment
// (handlers_deployment.go:121-132). (§11.4.6/§11.4.108: the previous
// `Deployment & {rollout_state?, failed_devices?, completed_devices?,
// total_devices?}` shape was fabricated — the server nests a `progress:
// DeploymentProgress` object, not a flat `rollout_state`/`failed_devices`/
// `completed_devices`/`total_devices` set.)
export interface DeploymentStatus extends Deployment {
  progress: DeploymentProgress;
}

export interface CreateRolloutRequest {
  deployment_id: string;
  groups: string[];
  rollout_percentage?: number;
  staged: boolean;
}

// RolloutPhaseSpec is the REAL per-phase shape inside RolloutState.phases —
// server/internal/api/handlers_rollout.go:16-22 (RolloutPhaseSpec struct).
export interface RolloutPhaseSpec {
  percentage: number;
  success_threshold: number;
  error_threshold: number;
  duration_seconds: number;
  auto_progress: boolean;
}

// RolloutState mirrors the REAL rollout-state wire shape —
// server/internal/api/handlers_rollout.go:38-44 (RolloutState struct) via
// toRolloutState (:67-75), returned by POST/GET
// /deployments/{id}/rollout. (§11.4.6/§11.4.108: the previous `{deployment_id,
// status, progress, success_rate, error_rate, total_devices,
// completed_devices, failed_devices}` shape was fabricated — the server
// sends `current_phase` + the full `phases` plan + `updated_at`, none of
// `progress`/`success_rate`/`error_rate`/`total_devices`/`completed_devices`/
// `failed_devices` exist on this response.)
export interface RolloutState {
  deployment_id: string;
  status: string;
  current_phase: number;
  phases: RolloutPhaseSpec[];
  updated_at: string;
}

// RolloutDecision mirrors the REAL POST /deployments/{id}/rollout/evaluate
// response — server/internal/api/handlers_rollout.go:47-51 (RolloutDecision
// struct) via handleEvaluateRollout (:129-160): `{action, reason, state}`.
// (§11.4.6/§11.4.108: the previous `{approved: boolean, reason: string}`
// shape was fabricated — the server sends an `action` string
// (advance/hold/halt/complete) + the full post-evaluation `state`, never a
// boolean `approved`.)
export interface RolloutDecision {
  action: string;
  reason: string;
  state: RolloutState;
}

// NOTE (§11.4.6 — out-of-scope discovery, not fixed in this pass): the REAL
// POST /deployments/{id}/recall request body — server/internal/api/
// handlers_recall.go:17-20 (RecallRequest struct) — is `{to_release_id,
// reason?}`. This client-declared `{reason, force}` shape sends neither the
// field the server requires (`to_release_id` is mandatory) nor matches its
// optional `reason` semantics; the real endpoint would reject this body.
// REQUEST-body interfaces are out of scope for this response-shape audit
// (see EVIDENCE.md) — flagged here as a discovered, unfixed defect for a
// follow-up work item.
export interface RecallRequest {
  reason: string;
  force: boolean;
}

// RollbackView mirrors the REAL rollback/abort history row —
// server/internal/api/handlers_recall.go:23-34 (RollbackView struct) via
// toRollbackView (:41-48), returned by POST /deployments/{id}/recall (single
// object) and as items of GET /deployments/{id}/rollbacks. Kind/
// FromReleaseID/ToReleaseID/RecallDeploymentID/Reason/TriggeredBy are
// `,omitempty` — absent when empty; Details is `map[string]string,omitempty`
// — absent when nil. (§11.4.6/§11.4.108: the previous inline `{id,
// deployment_id, reason, created_at}` item shape dropped `kind`,
// `from_release_id`, `to_release_id`, `recall_deployment_id`,
// `triggered_by`, and `details` — all real fields the server sends.)
export interface RollbackView {
  id: string;
  deployment_id: string;
  kind: string;
  from_release_id?: string;
  to_release_id?: string;
  recall_deployment_id?: string;
  reason?: string;
  triggered_by?: string;
  details?: Record<string, string>;
  created_at: string;
}

// RollbackList mirrors the REAL GET /deployments/{id}/rollbacks wire shape —
// server/internal/api/handlers_recall.go:36-39 (RollbackList struct):
// `{items}` ONLY — no cursor/pagination field of any kind on this endpoint
// (§11.4.6/§11.4.108: this previously declared a fabricated `total` field the
// server never sends).
export interface RollbackList {
  items: RollbackView[];
}

// Project mirrors the REAL project wire shape — server/internal/api/
// wire.go:260-266 (ProjectResponse struct) via toProjectResponse
// (handlers_project.go:95-103), returned by POST/GET/PATCH
// /projects(/{id}). Description is `string,omitempty` — absent when empty.
// (§11.4.6/§11.4.108: the previous shape had a fabricated `os_types:
// string[]` field the server never sends, and was missing the real
// `updated_at` field.)
export interface Project {
  project_id: string;
  name: string;
  description?: string;
  created_at: string;
  updated_at: string;
}
export interface CreateDeploymentRequest {
  release_id: string;
  group_ids: string[];
  strategy: "rolling" | "canary" | "blue_green";
  rollout_percentage?: number;
  staged: boolean;
}

export interface DeploymentResponse {
  id: string;
  release_id: string;
  group_ids: string[];
  strategy: string;
  rollout_percentage: number;
  staged: boolean;
  status: string;
  created_at: string;
  created_by: string;
}

// Groups
export interface CreateGroupRequest {
  name: string;
  description: string;
  device_ids: string[];
  labels: Record<string, string>;
}

export interface UpdateGroupRequest {
  name?: string;
  description?: string;
  labels?: Record<string, string>;
}

export interface AddGroupMembersRequest {
  device_ids: string[];
}

// AddGroupMembersResult mirrors the REAL POST /groups/{id}/members response —
// server/internal/api/handlers_group.go:77-81 (MemberAddResult struct):
// which submitted device ids were newly added, already members, or unknown.
// (§11.4.6/§11.4.108: the previous `{added: number, failed: number}` shape
// was fabricated — the server returns three ID arrays, never aggregate
// counts.)
export interface AddGroupMembersResult {
  added: string[];
  already_member: string[];
  not_found: string[];
}

// GroupMemberView is the REAL per-item shape of GroupMembers.items —
// server/internal/api/handlers_group.go:84-87 (GroupMemberView struct).
export interface GroupMemberView {
  device_id: string;
  added_at: string;
}

// GroupMembers mirrors the REAL GET /groups/{id}/members wire shape —
// server/internal/api/handlers_group.go:89-95 (GroupMembers struct):
// `{group_id, items, next_cursor}`, cursor-paginated (oldest-first).
// (§11.4.6/§11.4.108: the previous `{group_id, devices: DeviceRegistered[]}`
// shape was fabricated — the field is `items`, never `devices`; each item is
// `{device_id, added_at}`, never a full device-registration object; the
// response is cursor-paginated and the client type carried no `next_cursor`
// at all.)
export interface GroupMembers {
  group_id: string;
  items: GroupMemberView[];
  next_cursor: string | null;
}

// Group mirrors the REAL device-group wire shape — server/internal/api/
// handlers_group.go:54-60 (GroupView struct) via toGroupView/
// toGroupViewWithCount (:97-115), returned by POST/GET/PATCH
// /groups(/{id}) and (as list items) GET /groups. Description is
// `string,omitempty` — absent when empty.
// (§11.4.6/§11.4.108: the previous `{id, group_id?, project_id, name,
// description, device_count, labels, created_at}` shape was fabricated —
// groups are NOT project-scoped on the server (no `project_id`), there is no
// `labels` field, the real primary key is `group_id` (never bare `id`), and
// the real member-count field is `member_count`, never `device_count`.)
export interface Group {
  group_id: string;
  name: string;
  description?: string;
  member_count: number;
  created_at: string;
}

// GroupList mirrors the REAL GET /groups wire shape — server/internal/api/
// handlers_group.go:62-67 (GroupList struct): `{items, next_cursor}`, cursor-
// paginated. NextCursor is a `*string` with NO `omitempty` tag, so the key is
// ALWAYS present — `null` on the last page (§11.4.6/§11.4.108: this previously
// declared a fabricated `{total, cursor}` shape the server never sends).
export interface GroupList {
  items: Group[];
  next_cursor: string | null;
}

// Artifacts (server/internal/api/wire.go: ArtifactUploadMetadata, Artifact)
export interface ArtifactUploadMetadata {
  sha256: string;
  signature: string;
  version: string;
  os: string;
  target_model: string;
  file_hash?: string;
  file_size?: number;
  metadata_hash?: string;
  metadata_size?: number;
  payload_offset?: number;
  payload_size?: number;
}

export interface Artifact {
  artifact_id: string;
  sha256: string;
  size: number;
  os: string;
  target_model: string;
  version: string;
  storage_ref?: string;
  verified: boolean;
  uploaded_at: string;
}

// Deltas (server/internal/api/handlers_delta.go: DeltaRegister, DeltaView)
export interface DeltaRegisterRequest {
  base_artifact_id: string;
  target_artifact_id: string;
  sha256?: string;
  size?: number;
  storage_ref?: string;
}

export interface DeltaView {
  id: string;
  base_artifact_id: string;
  target_artifact_id: string;
  sha256?: string;
  size?: number;
  storage_ref?: string;
  created_at: string;
}

// GET /deltas?base=<id>&target=<id> (server/internal/api/handlers_delta.go handleFindDelta)
export interface DeltaFindParams {
  base: string;
  target: string;
}

// Audit
// AuditActor identifies who performed an audited action — server/internal/
// api/audit_wire.go:28-31 (AuditActor struct): the durable users-row key
// (UserID, nullable) plus the token Subject (always set).
export interface AuditActor {
  user_id?: string;
  subject: string;
}

// AuditEntry mirrors the REAL audit_logs response projection —
// server/internal/api/audit_wire.go:9-22 (AuditLogEntry struct) via
// toAuditLogEntry (:39-55), returned as items of GET /audit.
// ResourceID/Details/IPAddress/UserAgent are `,omitempty` — absent when
// empty/nil. (§11.4.6/§11.4.108: the previous `actor: string` field was
// fabricated — the server nests `{user_id?, subject}`, never a bare string;
// `user_agent` was entirely missing from the client type.)
export interface AuditEntry {
  id: string;
  actor: AuditActor;
  action: string;
  resource_type: string;
  resource_id?: string;
  details?: Record<string, unknown>;
  ip_address?: string;
  user_agent?: string;
  created_at: string;
}

// GET /audit query filters (server/internal/api/handlers_audit.go handleListAudit)
export interface AuditFilter {
  action?: string;
  resource_type?: string;
  since?: string;
  until?: string;
  cursor?: string;
  limit?: number;
}

// GET /audit paged body (server/internal/api/audit_wire.go AuditLogList)
export interface AuditLogList {
  items: AuditEntry[];
  next_cursor?: string;
}
