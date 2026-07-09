/**
 * §11.4.170 device-independent HOST-side render harness for ota-manager —
 * Dashboard screen (`/dashboard`).
 *
 * Mounts the REAL, unmodified route tree (src/route-tree.gen.ts — the exact
 * routeTree shipped in src/main.tsx) with a memory history seeded at
 * "/dashboard", so the real MainLayout (Sidebar + Topbar) wraps the real
 * DashboardPage exactly as production does. Only the harness-level seams are
 * stubbed, exactly like harness.tsx does for LoginPage:
 *   - QueryClientProvider (DashboardPage -> useTelemetryOverview/useAuditLog)
 *   - ToastProvider (present in the real app tree; some layout children read it)
 *   - auth-store pre-seeded with a fake authenticated user (Topbar reads it)
 *   - ui-store theme applied via the `?theme=` param, same contract as harness.tsx
 *   - network responses for GET /telemetry/overview and GET /audit are
 *     intercepted at the Playwright layer (see lib-render.mjs) with canned,
 *     type-shape-accurate JSON — no product source is modified to make this
 *     possible.
 *
 * This file does NOT catch or suppress render errors — if the real
 * DashboardPage throws (e.g. a hook requiring a context the app never
 * provides), that exception propagates and is captured by the runner via
 * `page.on("pageerror")`, exactly as it would in the real shipped app (which
 * has no ErrorBoundary anywhere in its tree — confirmed by inspection).
 */
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { RouterProvider, createRouter, createMemoryHistory } from "@tanstack/react-router";
import { ToastProvider } from "@/components/ui/toast";
import { routeTree } from "@/route-tree.gen";
import { useAuthStore } from "@/stores/auth-store";
import "./harness.css";

// --- theme (matches src/index.css contract exactly, same as harness.tsx) ---
const params = new URLSearchParams(window.location.search);
const theme = params.get("theme") === "light" ? "light" : "dark";
document.documentElement.classList.remove("light", "dark");
document.documentElement.classList.add(theme);
document.documentElement.setAttribute("data-hostrender-theme", theme);

// --- auth-store: seed a fake authenticated user (Topbar renders it; the
// dashboard route is only reachable in the real app once authenticated) ---
useAuthStore.getState().setAuth({
  token: "hostrender-fake-token",
  refreshToken: "hostrender-fake-refresh",
  user: {
    id: "u-hostrender",
    email: "operator@example.com",
    display_name: "QA Operator",
    avatar_url: null,
    roles: ["admin"],
    permissions: [],
  },
});

// --- providers: the REAL route tree needs QueryClient + Router context,
// exactly as src/main.tsx wires them. ---
const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});

const router = createRouter({
  routeTree,
  history: createMemoryHistory({ initialEntries: ["/dashboard"] }),
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}

const rootEl = document.getElementById("root");
if (!rootEl) throw new Error("harness: #root not found");

ReactDOM.createRoot(rootEl).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <ToastProvider>
        <RouterProvider router={router} />
      </ToastProvider>
    </QueryClientProvider>
  </React.StrictMode>,
);
