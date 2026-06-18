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

const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080";

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

export interface DeviceList {
  items: DeviceRegistered[];
  total: number;
  cursor?: string;
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

export interface TelemetryHistory {
  items: Record<string, unknown>[];
  total: number;
  cursor?: string;
}

export interface TelemetryOverview {
  totalDevices: number;
  activeDeployments: number;
  pendingUpdates: number;
  failedDevices: number;
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

export interface ReleaseList {
  items: Release[];
  total: number;
  cursor?: string;
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

export interface DeploymentList {
  items: Deployment[];
  total: number;
  cursor?: string;
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

export interface RollbackList {
  items: { id: string; deployment_id: string; reason: string; created_at: string }[];
  total: number;
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

export interface GroupList {
  items: Group[];
  total: number;
  cursor?: string;
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
