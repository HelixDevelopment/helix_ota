import axios, { AxiosError, type InternalAxiosRequestConfig } from "axios";
import { useAuthStore } from "@/stores/auth-store";

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

// Releases
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

// Deployments
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

export interface Group {
  id: string;
  project_id: string;
  name: string;
  description: string;
  device_count: number;
  labels: Record<string, string>;
  created_at: string;
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
