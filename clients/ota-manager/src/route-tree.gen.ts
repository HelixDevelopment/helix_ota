import { createRoute } from "@tanstack/react-router";
import { Route as rootRoute } from "@/routes/__root";
import { DashboardPage } from "@/routes/dashboard";
import { MainLayout } from "@/features/layout/main-layout";
import { LoginPage } from "@/features/auth/login-page";

// Login page — no sidebar layout
const loginRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: "/login",
  component: LoginPage,
});

// Layout route — sidebar + topbar wrapper
const layoutRoute = createRoute({
  getParentRoute: () => rootRoute,
  id: "layout",
  component: MainLayout,
});

// Dashboard lives inside the layout
const indexedDashboardRoute = createRoute({
  getParentRoute: () => layoutRoute,
  path: "/dashboard",
  component: DashboardPage,
});

const routeTree = rootRoute.addChildren([
  loginRoute,
  layoutRoute.addChildren([indexedDashboardRoute]),
]);

export { routeTree };
