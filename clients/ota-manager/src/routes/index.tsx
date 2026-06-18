import { createFileRoute, redirect } from "@tanstack/react-router";
import LoginPage from "@/features/auth/login-page";
import DashboardPage from "@/features/dashboard/dashboard-page";
import DevicesPage from "@/features/devices/devices-page";
import DeviceDetailPage from "@/features/devices/device-detail-page";
import ReleasesPage from "@/features/releases/releases-page";
import DeploymentsPage from "@/features/deployments/deployments-page";
import DeploymentDetailPage from "@/features/deployments/deployment-detail-page";
import GroupsPage from "@/features/groups/groups-page";
import AuditPage from "@/features/audit/audit-page";

function isAuthenticated(): boolean {
  return !!localStorage.getItem("auth_token");
}

function AuthGuard({ children }: { children: React.ReactNode }) {
  if (!isAuthenticated()) {
    throw redirect({ to: "/login" });
  }
  return <>{children}</>;
}

// ── Root redirect ──────────────────────────────────────────────────────
export const Route = createFileRoute("/")({
  component: () => {
    throw redirect({ to: "/dashboard" });
  },
});

// ── Login (no auth required) ───────────────────────────────────────────
export const LoginRoute = createFileRoute("/login")({
  component: LoginPage,
});

// ── Dashboard (auth required) ──────────────────────────────────────────
export const DashboardRoute = createFileRoute("/dashboard")({
  component: () => (
    <AuthGuard>
      <DashboardPage />
    </AuthGuard>
  ),
});

// ── Devices (auth required) ────────────────────────────────────────────
export const DevicesRoute = createFileRoute("/devices")({
  component: () => (
    <AuthGuard>
      <DevicesPage />
    </AuthGuard>
  ),
});

// ── Device detail (auth required) ──────────────────────────────────────
export const DeviceDetailRoute = createFileRoute("/devices/$deviceId")({
  component: () => (
    <AuthGuard>
      <DeviceDetailPage />
    </AuthGuard>
  ),
});

// ── Releases (auth required) ───────────────────────────────────────────
export const ReleasesRoute = createFileRoute("/releases")({
  component: () => (
    <AuthGuard>
      <ReleasesPage />
    </AuthGuard>
  ),
});

// ── Deployments (auth required) ────────────────────────────────────────
export const DeploymentsRoute = createFileRoute("/deployments")({
  component: () => (
    <AuthGuard>
      <DeploymentsPage />
    </AuthGuard>
  ),
});

// ── Deployment detail (auth required) ──────────────────────────────────
export const DeploymentDetailRoute = createFileRoute("/deployments/$deploymentId")({
  component: () => (
    <AuthGuard>
      <DeploymentDetailPage />
    </AuthGuard>
  ),
});

// ── Groups (auth required) ─────────────────────────────────────────────
export const GroupsRoute = createFileRoute("/groups")({
  component: () => (
    <AuthGuard>
      <GroupsPage />
    </AuthGuard>
  ),
});

// ── Audit (auth required) ──────────────────────────────────────────────
export const AuditRoute = createFileRoute("/audit")({
  component: () => (
    <AuthGuard>
      <AuditPage />
    </AuthGuard>
  ),
});
