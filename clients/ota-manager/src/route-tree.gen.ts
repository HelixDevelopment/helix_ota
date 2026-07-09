import { createRoute } from "@tanstack/react-router";
import { Route as rootRoute } from "@/routes/__root";
import { MainLayout } from "@/features/layout/main-layout";
import LoginPage from "@/features/auth/login-page";
import DashboardPage from "@/features/dashboard/dashboard-page";
import DevicesPage from "@/features/devices/devices-page";
import DeviceDetailPage from "@/features/devices/device-detail-page";
import ReleasesPage from "@/features/releases/releases-page";
import DeploymentsPage from "@/features/deployments/deployments-page";
import DeploymentDetailPage from "@/features/deployments/deployment-detail-page";
import GroupsPage from "@/features/groups/groups-page";
import AuditPage from "@/features/audit/audit-page";

// Login page — no sidebar layout.
const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  component: LoginPage,
});

// Layout route — sidebar + topbar wrapper hosting the authenticated pages.
const layoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "layout",
  component: MainLayout,
});

const dashboardRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/dashboard",
  component: DashboardPage,
});

const devicesRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/devices",
  component: DevicesPage,
});

const deviceDetailRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/devices/$deviceId",
  component: DeviceDetailPage,
});

const releasesRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/releases",
  component: ReleasesPage,
});

const deploymentsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/deployments",
  component: DeploymentsPage,
});

const deploymentDetailRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/deployments/$deploymentId",
  component: DeploymentDetailPage,
});

const groupsRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/groups",
  component: GroupsPage,
});

const auditRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/audit",
  component: AuditPage,
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  layoutRoute.addChildren([
    dashboardRoute,
    devicesRoute,
    deviceDetailRoute,
    releasesRoute,
    deploymentsRoute,
    deploymentDetailRoute,
    groupsRoute,
    auditRoute,
  ]),
]);

export { routeTree };
