/**
 * §11.4.170 device-independent HOST-side render harness for ota-manager.
 *
 * Mounts the REAL LoginPage component (src/features/auth/login-page.tsx) — not a
 * copy — with the exact providers it depends on:
 *   - QueryClientProvider (LoginPage -> useLogin -> useMutation)
 *   - RouterProvider with a memory history (LoginPage -> useLogin -> useNavigate)
 *
 * Theme is driven by the `?theme=light|dark` URL param, applying the SAME class
 * contract the app's src/index.css declares:
 *   - light  => documentElement gets class `light` (light HSL token overrides)
 *   - dark   => documentElement gets class `dark`  (tokens fall back to :root = dark palette)
 *
 * No dev server, no emulator, no running app — this file is Vite-built to a static
 * bundle and rendered to PNG by Playwright on the host.
 */
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from "@tanstack/react-router";
import LoginPage from "@/features/auth/login-page";
import "./harness.css";

// --- theme (matches src/index.css contract exactly) ---
const params = new URLSearchParams(window.location.search);
const theme = params.get("theme") === "light" ? "light" : "dark";
document.documentElement.classList.remove("light", "dark");
document.documentElement.classList.add(theme);
document.documentElement.setAttribute("data-hostrender-theme", theme);

// --- providers: real component needs QueryClient + Router context ---
const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
});

const rootRoute = createRootRoute({ component: LoginPage });
const router = createRouter({
  routeTree: rootRoute,
  history: createMemoryHistory({ initialEntries: ["/"] }),
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
      <RouterProvider router={router} />
    </QueryClientProvider>
  </React.StrictMode>,
);
