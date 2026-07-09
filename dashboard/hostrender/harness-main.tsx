// Helix OTA dashboard — §11.4.170 HOST-RENDER harness entry.
//
// Mounts the REAL feature screens (the same components the operator uses) to
// host pixels in headless Chromium, WITHOUT a device, emulator, or running Go
// backend. The screens that the login harness cannot reach — because they sit
// behind ProtectedRoute and fetch /api/v1 data — are rendered here by:
//   1. installing a window.fetch stub that returns canned /api/v1 envelopes
//      (BEFORE React mounts, so the apiClient singleton's calls are captured);
//   2. wrapping the screen in the REAL <AuthProvider> + Router and driving it
//      to `authenticated` via a stubbed /auth/login (a crafted, UNSIGNED JWT —
//      the client decodes claims for UX only; the server is authoritative), so
//      RoleGate-gated actions render exactly as in production.
// This is a component-render stub (permitted §11.4.27) — no view logic is
// faked; every pixel is the real component under the real token layer. The
// screen is selected with
// ?screen=audit|releases|artifact-upload|appshell|deployments|fleet|groups|overview.
//
// STATE VARIATION (2026-07-10, §11.4.170 screen×STATE gap): an optional
// &state=empty|error query param swaps the canned /api response for the
// SPECIFIC endpoint the requested screen depends on, so the REAL empty-list /
// REAL error-panel branch in the component renders — no fabricated UI, no
// synthetic DOM injection. Only `releases` (GET /releases) and `fleet` (GET
// /telemetry/overview) declare a state variant below; every other screen
// param ignores `state` and behaves exactly as before (backward-compatible
// default `state=default`).
//
// Theme: seeded to light here; the Playwright spec stamps the authoritative
// data-theme (light/dark) on <html> before sampling — the same attribute
// theme.ts writes — so every var(--token) pixel flips deterministically.

import { StrictMode, useEffect, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import { MemoryRouter, Outlet, Route, Routes } from "react-router-dom";
import "../src/styles/tokens.css";
import { AuthProvider, useAuth } from "../src/auth/AuthContext";
import { AppShell } from "../src/components/AppShell";
import { Card } from "../src/components/ui";
import { AuditScreen } from "../src/screens/AuditScreen";
import { ReleaseList } from "../src/screens/ReleasesScreen";
import { ArtifactUploadScreen } from "../src/screens/ArtifactUploadScreen";
import { DeploymentList } from "../src/screens/DeploymentsScreen";
import { FleetHealth } from "../src/screens/FleetScreen";
import { GroupList } from "../src/screens/GroupsScreen";
import { DashboardOverview } from "../src/screens/OverviewScreen";
import type {
  AuditList,
  DeviceGroupList,
  HealthStatus,
  ReleaseList as ReleaseListT,
  TelemetryOverview,
  TokenResponse,
} from "../src/types/api";

// ── canned data ─────────────────────────────────────────────────────────────
// A crafted UNSIGNED JWT: header.payload.sig. AuthContext.decodeClaims reads
// only the payload (base64url) for roles/exp (UX-only, server authoritative).
function b64url(o: unknown): string {
  return btoa(JSON.stringify(o)).replace(/=+$/, "").replace(/\+/g, "-").replace(/\//g, "_");
}
const CLAIMS = { sub: "operator@helix.dev", roles: ["admin", "operator"], exp: 4102444800 };
const JWT = `e30.${b64url(CLAIMS)}.sig`;

const TOKENS: TokenResponse = {
  access_token: JWT,
  token_type: "Bearer",
  expires_in: 900,
  refresh_token: "refresh-stub",
  roles: ["admin", "operator"],
};

const AUDIT: AuditList = {
  items: [
    {
      id: "aud_1",
      actor: { subject: "alice", user_id: "usr_alice" },
      action: "RELEASE_CREATE",
      resource_type: "release",
      resource_id: "rel_42",
      ip_address: "10.0.0.7",
      created_at: "2026-06-10T09:00:00Z",
    },
    {
      id: "aud_2",
      actor: { subject: "system" },
      action: "DEPLOYMENT_HALT",
      resource_type: "deployment",
      resource_id: "dep_7",
      ip_address: "10.0.0.9",
      created_at: "2026-06-10T08:00:00Z",
    },
    {
      id: "aud_3",
      actor: { subject: "bob", user_id: "usr_bob" },
      action: "ARTIFACT_UPLOAD",
      resource_type: "artifact",
      created_at: "2026-06-10T07:30:00Z",
    },
  ],
  next_cursor: "aud_cursor_87",
};

const RELEASES: ReleaseListT = {
  items: [
    {
      release_id: "rel_42",
      artifact_id: "art_11",
      version: "1.4.0",
      os: "android",
      target_model: "rk3588-opi5max",
      status: "published",
      created_at: "2026-06-10T09:00:00Z",
    },
    {
      release_id: "rel_43",
      artifact_id: "art_12",
      version: "1.4.1-rc1",
      os: "android",
      target_model: "rk3588-opi5max",
      status: "draft",
      created_at: "2026-06-11T10:15:00Z",
    },
  ],
};

// GET /healthz — best-effort server-health badge on DashboardOverview.
const HEALTH: HealthStatus = { status: "ok" };

// GET /telemetry/overview — fleet-wide aggregates for FleetHealth.
const TELEMETRY_OVERVIEW: TelemetryOverview = {
  event_counts: { success: 12, failure: 2, installing: 3 },
  total: 17,
  failure_rate: 0.12,
  by_state: { success: 10, failure: 2, installing: 3, idle: 2 },
};

// state=empty variant: a genuine 200 response with zero rows in EVERY
// aggregate — exercises FleetHealth's "No device states reported yet." /
// "No telemetry events yet." EmptyState branches (real component code path,
// not a synthetic DOM injection).
const TELEMETRY_OVERVIEW_EMPTY: TelemetryOverview = {
  event_counts: {},
  total: 0,
  failure_rate: 0,
  by_state: {},
};

// state=empty variant: a genuine 200 response with zero releases — exercises
// ReleaseList's "No releases yet." EmptyState branch.
const RELEASES_EMPTY: ReleaseListT = { items: [] };

// GET /groups — device-group roster for GroupList.
const GROUPS: DeviceGroupList = {
  items: [
    {
      group_id: "grp_1",
      name: "canary-fleet",
      description: "Canary rollout devices",
      member_count: 12,
      created_at: "2026-06-01T00:00:00Z",
    },
    {
      group_id: "grp_2",
      name: "production-fleet",
      description: "Main production fleet",
      member_count: 340,
      created_at: "2026-06-02T00:00:00Z",
    },
  ],
};

// ── window.fetch stub (installed before mount) ──────────────────────────────
function jsonRes(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

// ?state=empty|error (default: "default") — read once at module load, same as
// `screen`. Only consulted for the endpoints declared below; every other
// endpoint is unaffected regardless of its value.
const harnessState = new URLSearchParams(window.location.search).get("state") ?? "default";

const realFetch = window.fetch.bind(window);
window.fetch = async (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
  const raw = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
  let path = raw;
  try {
    path = new URL(raw, window.location.origin).pathname;
  } catch {
    /* keep raw */
  }
  if (path.endsWith("/auth/login")) return jsonRes(TOKENS);
  if (path.endsWith("/audit")) return jsonRes(AUDIT);
  if (path.endsWith("/releases")) {
    if (harnessState === "empty") return jsonRes(RELEASES_EMPTY);
    if (harnessState === "error") {
      // A genuine non-404 failure (500) so ReleaseList's `error` branch
      // (ErrorPanel) renders — 404 has no special-cased meaning for this
      // screen, unlike FleetHealth's telemetry-overview 404 degrade path.
      return jsonRes(
        { error: { code: "INTERNAL", message: "Releases backend unavailable." } },
        500,
      );
    }
    return jsonRes(RELEASES);
  }
  if (path.endsWith("/healthz")) return jsonRes(HEALTH);
  if (path.endsWith("/telemetry/overview")) {
    if (harnessState === "empty") return jsonRes(TELEMETRY_OVERVIEW_EMPTY);
    if (harnessState === "error") {
      // A genuine non-404 failure (500) — FleetHealth treats 404 specially
      // (endpoint-not-available EmptyState per design), so the ERROR-state
      // proof MUST use a different status to reach the real ErrorPanel branch.
      return jsonRes(
        { error: { code: "INTERNAL", message: "Telemetry overview backend unavailable." } },
        500,
      );
    }
    return jsonRes(TELEMETRY_OVERVIEW);
  }
  if (path.endsWith("/groups")) return jsonRes(GROUPS);
  // Any other /api path is not needed by the harnessed screens.
  if (path.startsWith("/api/")) return jsonRes({ error: { code: "NOT_STUBBED", message: path } }, 404);
  return realFetch(input, init);
};

// ── bootstrap: drive the real AuthProvider to `authenticated` ───────────────
function Bootstrap({ children }: { children: ReactNode }) {
  const { status, login } = useAuth();
  useEffect(() => {
    if (status !== "authenticated") void login("operator@helix.dev", "harness");
  }, [status, login]);
  if (status !== "authenticated") return <div data-harness="auth-pending" />;
  return <div data-harness="ready">{children}</div>;
}

// AppShell renders an <Outlet/>; give it a minimal themed child so the frame we
// prove IS the real AppShell chrome (header brand-chrome + nav + theme toggle +
// token-driven shell body).
function AppShellFrameChild() {
  return (
    <div>
      <h1>Overview</h1>
      <Card title="Fleet summary">
        <p>
          AppShell brand-chrome frame — §11.4.170 host-render. The header is the
          fixed dark-navy brand bar; the body below follows var(--bg)/var(--fg).
        </p>
      </Card>
    </div>
  );
}

function ScreenHost({ screen }: { screen: string }) {
  if (screen === "appshell") {
    return (
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<AppShellFrameChild />} />
        </Route>
      </Routes>
    );
  }
  // Non-shell screens render standalone inside a themed body wrapper so the
  // token-driven page background is part of the captured surface.
  let el: ReactNode;
  switch (screen) {
    case "releases":
      el = <ReleaseList />;
      break;
    case "artifact-upload":
      el = <ArtifactUploadScreen />;
      break;
    case "deployments":
      el = <DeploymentList />;
      break;
    case "fleet":
      el = <FleetHealth />;
      break;
    case "groups":
      el = <GroupList />;
      break;
    case "overview":
      el = <DashboardOverview />;
      break;
    case "audit":
    default:
      el = <AuditScreen />;
      break;
  }
  return (
    <div style={{ minHeight: "100vh", background: "var(--bg)", color: "var(--fg)" }}>
      <main style={{ maxWidth: 1100, margin: "0 auto", padding: "24px 20px" }}>{el}</main>
    </div>
  );
}

const params = new URLSearchParams(window.location.search);
const screen = params.get("screen") ?? "audit";

// Seed an explicit theme so no un-themed frame paints before the spec stamps
// its authoritative light/dark (mirrors theme.ts initTheme's guarantee).
document.documentElement.setAttribute("data-theme", "light");

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("harness: #root not found");

createRoot(rootEl).render(
  <StrictMode>
    <AuthProvider>
      <MemoryRouter initialEntries={["/"]}>
        <Bootstrap>
          <ScreenHost screen={screen} />
        </Bootstrap>
      </MemoryRouter>
    </AuthProvider>
  </StrictMode>,
);
