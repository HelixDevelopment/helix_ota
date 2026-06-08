// Helix OTA — typed fetch client for the /api/v1 control plane (design §8).
// The ONLY thing in the SPA that talks to /api/v1. It:
//  - attaches Authorization: Bearer <access> + Accept-Encoding: br, gzip (design §7.2),
//  - maps the server error envelope (design §6) into a typed ApiError,
//  - performs a single transparent 401 -> refresh -> retry, sharing one in-flight refresh,
//  - carries NO view logic (decoupling, design §11.4.28).
//
// Token state + refresh policy live in auth/AuthContext.tsx; this module is handed a
// minimal TokenBridge so it can read the access token and request a rotation on 401.

import type {
  ApiErrorEnvelope,
  Artifact,
  ArtifactUploadMetadata,
  AuditList,
  Deployment,
  DeploymentCreate,
  DeploymentStatus,
  DeviceGroup,
  DeviceGroupCreate,
  DeviceGroupList,
  DeviceStatus,
  HealthStatus,
  LoginRequest,
  Release,
  ReleaseCreate,
  ReleaseList,
  RolloutCommand,
  RolloutState,
  TelemetryHistory,
  TelemetryOverview,
  TokenResponse,
} from "../types/api";

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly requestId?: string;
  readonly details: { field?: string; message: string }[];

  constructor(
    status: number,
    code: string,
    message: string,
    requestId?: string,
    details?: { field?: string; message: string }[],
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
    this.details = details ?? [];
  }
}

// Bridge to the auth layer. Implemented by AuthContext (design §7.2).
export interface TokenBridge {
  getAccessToken(): string | null;
  // Returns the new access token on success, or null if the session is dead.
  refresh(): Promise<string | null>;
  onSessionExpired(): void;
}

function resolveBaseUrl(): string {
  // Runtime config injection (design §3.2). Falls back to same-origin /api/v1.
  const injected = (globalThis as { __HELIX_CONFIG__?: { apiBaseUrl?: string } })
    .__HELIX_CONFIG__?.apiBaseUrl;
  return (injected ?? "/api/v1").replace(/\/+$/, "");
}

interface RequestOptions {
  method?: string;
  // JSON body; omitted for multipart (use `form`).
  body?: unknown;
  form?: FormData;
  // When true, do not attach the bearer token (used for /auth/login).
  anonymous?: boolean;
  // When true, do not attempt a 401 -> refresh -> retry (used for /auth/refresh itself).
  noRefresh?: boolean;
  query?: Record<string, string | number | undefined>;
}

export class ApiClient {
  private readonly baseUrl: string;
  private bridge: TokenBridge | null = null;

  constructor(baseUrl: string = resolveBaseUrl()) {
    this.baseUrl = baseUrl;
  }

  // Wired by AuthProvider once the session layer is constructed (design §4).
  attachTokenBridge(bridge: TokenBridge): void {
    this.bridge = bridge;
  }

  private buildUrl(path: string, query?: RequestOptions["query"]): string {
    const url = new URL(
      this.baseUrl + path,
      // base needed for relative same-origin baseUrl in browsers/node
      typeof window !== "undefined" ? window.location.origin : "http://localhost",
    );
    if (query) {
      for (const [k, v] of Object.entries(query)) {
        if (v !== undefined) url.searchParams.set(k, String(v));
      }
    }
    return url.toString();
  }

  private async parseError(res: Response): Promise<ApiError> {
    let code = "UNKNOWN";
    let message = res.statusText || "request failed";
    let requestId: string | undefined = res.headers.get("X-Request-Id") ?? undefined;
    let details: { field?: string; message: string }[] | undefined;
    try {
      const env = (await res.json()) as ApiErrorEnvelope;
      if (env?.error) {
        code = env.error.code ?? code;
        message = env.error.message ?? message;
        requestId = env.error.request_id ?? requestId;
        details = env.error.details;
      }
    } catch {
      // Non-JSON error body — keep the status-derived defaults.
    }
    return new ApiError(res.status, code, message, requestId, details);
  }

  private async raw(path: string, opts: RequestOptions, retrying = false): Promise<Response> {
    const headers: Record<string, string> = {
      Accept: "application/json",
      "Accept-Encoding": "br, gzip",
    };
    let body: BodyInit | undefined;
    if (opts.form) {
      body = opts.form; // browser sets the multipart boundary Content-Type
    } else if (opts.body !== undefined) {
      headers["Content-Type"] = "application/json";
      body = JSON.stringify(opts.body);
    }
    if (!opts.anonymous) {
      const token = this.bridge?.getAccessToken();
      if (token) headers["Authorization"] = `Bearer ${token}`;
    }

    const res = await fetch(this.buildUrl(path, opts.query), {
      method: opts.method ?? "GET",
      headers,
      body,
    });

    // Transparent single 401 -> refresh -> retry with rotation (design §7.2).
    if (res.status === 401 && !opts.noRefresh && !opts.anonymous && !retrying && this.bridge) {
      const newToken = await this.bridge.refresh();
      if (newToken) {
        return this.raw(path, opts, true);
      }
      this.bridge.onSessionExpired();
    }
    return res;
  }

  private async json<T>(path: string, opts: RequestOptions = {}): Promise<T> {
    const res = await this.raw(path, opts);
    if (!res.ok) throw await this.parseError(res);
    if (res.status === 204) return undefined as T;
    return (await res.json()) as T;
  }

  // --- auth (design §7) -----------------------------------------------------

  login(req: LoginRequest): Promise<TokenResponse> {
    return this.json<TokenResponse>("/auth/login", {
      method: "POST",
      body: req,
      anonymous: true,
    });
  }

  // refresh is called by the AuthContext directly; noRefresh prevents recursion.
  refresh(refreshToken: string): Promise<TokenResponse> {
    return this.json<TokenResponse>("/auth/refresh", {
      method: "POST",
      body: { refresh_token: refreshToken },
      anonymous: true,
      noRefresh: true,
    });
  }

  // --- artifacts (design §9.2) ---------------------------------------------

  uploadArtifact(file: File, metadata: ArtifactUploadMetadata): Promise<Artifact> {
    const form = new FormData();
    form.append("file", file);
    form.append("metadata", JSON.stringify(metadata));
    return this.json<Artifact>("/artifacts/upload", { method: "POST", form });
  }

  getArtifact(id: string): Promise<Artifact> {
    return this.json<Artifact>(`/artifacts/${encodeURIComponent(id)}`);
  }

  // --- releases (design §9.3) ----------------------------------------------

  listReleases(query?: {
    os?: string;
    target_model?: string;
    status?: string;
    limit?: number;
    cursor?: string;
  }): Promise<ReleaseList> {
    return this.json<ReleaseList>("/releases", { query });
  }

  createRelease(req: ReleaseCreate): Promise<Release> {
    return this.json<Release>("/releases", { method: "POST", body: req });
  }

  getRelease(id: string): Promise<Release> {
    return this.json<Release>(`/releases/${encodeURIComponent(id)}`);
  }

  // --- deployments (design §9.3) -------------------------------------------

  createDeployment(req: DeploymentCreate): Promise<Deployment> {
    return this.json<Deployment>("/deployments", { method: "POST", body: req });
  }

  getDeployment(id: string): Promise<DeploymentStatus> {
    return this.json<DeploymentStatus>(`/deployments/${encodeURIComponent(id)}`);
  }

  // Rollout panel (staged rollout API is G7/1.0.1, routes flagged UNVERIFIED — design §13).
  getRollout(deploymentId: string): Promise<RolloutState> {
    return this.json<RolloutState>(
      `/deployments/${encodeURIComponent(deploymentId)}/rollout`,
    );
  }

  postRollout(deploymentId: string, cmd: RolloutCommand): Promise<RolloutState> {
    return this.json<RolloutState>(
      `/deployments/${encodeURIComponent(deploymentId)}/rollout`,
      { method: "POST", body: cmd },
    );
  }

  // --- devices + telemetry (design §9.4) -----------------------------------

  getDeviceStatus(deviceId: string): Promise<DeviceStatus> {
    return this.json<DeviceStatus>(`/devices/${encodeURIComponent(deviceId)}/status`);
  }

  getDeviceTelemetry(deviceId: string): Promise<TelemetryHistory> {
    return this.json<TelemetryHistory>(
      `/devices/${encodeURIComponent(deviceId)}/telemetry`,
    );
  }

  getTelemetryOverview(): Promise<TelemetryOverview> {
    return this.json<TelemetryOverview>("/telemetry/overview");
  }

  // --- groups (design DeploymentCreate group; G5/1.0.1) --------------------

  listGroups(query?: { limit?: number; cursor?: string }): Promise<DeviceGroupList> {
    return this.json<DeviceGroupList>("/groups", { query });
  }

  createGroup(req: DeviceGroupCreate): Promise<DeviceGroup> {
    return this.json<DeviceGroup>("/groups", { method: "POST", body: req });
  }

  // --- audit (design §6; G3/1.0.1) -----------------------------------------

  listAudit(query?: { limit?: number; cursor?: string }): Promise<AuditList> {
    return this.json<AuditList>("/audit", { query });
  }

  // --- health (best-effort; not a defined MVP route — design §8) ------------

  health(): Promise<HealthStatus> {
    return this.json<HealthStatus>("/healthz", { anonymous: true });
  }
}

// Shared singleton instance for the app (design §4: single typed client).
export const apiClient = new ApiClient();
