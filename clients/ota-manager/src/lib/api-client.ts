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

export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  token_type: string;
}

// Devices
export interface DeviceRegistrationRequest {
  device_id: string;
  board: string;
  firmware_version: string;
  hardware_revision: string;
  serial_number: string;
}

export interface DeviceRegistered {
  id: string;
  device_id: string;
  board: string;
  status: string;
  created_at: string;
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
  items: DeviceRegistered[];
  next_cursor: string | null;
}

export interface DeviceStatus {
  device_id: string;
  online: boolean;
  current_release_id: string;
  last_seen: string;
  ip_address: string;
}

export interface DeviceHealth {
  device_id: string;
  online: boolean;
  battery_percent: number;
  storage_used_percent: number;
  temperature_celsius: number;
  uptime_seconds: number;
  last_reported: string;
}

// Telemetry
export interface TelemetryFilter {
  event?: string;
  since?: string;
  until?: string;
  cursor?: string;
  limit?: number;
}

// TelemetryHistory mirrors the REAL GET /devices/{id}/telemetry wire shape —
// server/internal/api/handlers_telemetry.go:30-38 (TelemetryHistory struct):
// `{device_id, items, next_cursor}`, cursor-paginated. NextCursor is a
// `*string` with NO `omitempty` tag, so the key is ALWAYS present — `null` on
// the last page (§11.4.6/§11.4.108: this previously declared a fabricated
// `{total, cursor}` shape the server never sends).
export interface TelemetryHistory {
  items: Record<string, unknown>[];
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

export interface ReleaseResponse {
  id: string;
  release_id?: string;
  project_id: string;
  version: string;
  file_url: string;
  file_hash: string;
  changelog: string;
  target_board: string;
  firmware_version: string;
  status: string;
  created_at: string;
  created_by: string;
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
export interface Deployment {
  deployment_id: string;
  release_id: string;
  group_ids: string[];
  strategy: string;
  rollout_percentage: number;
  staged: boolean;
  status: string;
  created_at: string;
  created_by: string;
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

export interface DeploymentStatus extends Deployment {
  rollout_state?: RolloutState;
  failed_devices?: number;
  completed_devices?: number;
  total_devices?: number;
}

export interface CreateRolloutRequest {
  deployment_id: string;
  groups: string[];
  rollout_percentage?: number;
  staged: boolean;
}

export interface RolloutState {
  deployment_id: string;
  status: string;
  progress: number;
  success_rate: number;
  error_rate: number;
  total_devices: number;
  completed_devices: number;
  failed_devices: number;
}

export interface RolloutDecision {
  approved: boolean;
  reason: string;
}

export interface RecallRequest {
  reason: string;
  force: boolean;
}

// RollbackList mirrors the REAL GET /deployments/{id}/rollbacks wire shape —
// server/internal/api/handlers_recall.go:36-39 (RollbackList struct):
// `{items}` ONLY — no cursor/pagination field of any kind on this endpoint
// (§11.4.6/§11.4.108: this previously declared a fabricated `total` field the
// server never sends).
export interface RollbackList {
  items: { id: string; deployment_id: string; reason: string; created_at: string }[];
}

export interface Project {
  project_id: string;
  name: string;
  description: string;
  os_types: string[];
  created_at: string;
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

export interface AddGroupMembersResult {
  added: number;
  failed: number;
}

export interface GroupMembers {
  group_id: string;
  devices: DeviceRegistered[];
}

export interface Group {
  id: string;
  group_id?: string;
  project_id: string;
  name: string;
  description: string;
  device_count: number;
  labels: Record<string, string>;
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
export interface AuditEntry {
  id: string;
  actor: string;
  action: string;
  resource_type: string;
  resource_id: string;
  details: Record<string, unknown>;
  ip_address: string;
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
