// Helix OTA dashboard — Playwright smoke-test config (anti-bluff browser proof).
//
// Boots the REAL Go control plane (HELIX_PORT=8095, in-memory, admin seeded via
// HELIX_ADMIN_PASSWORD) in globalSetup, and runs the Vite dev server (which
// proxies /api -> HELIX_API_TARGET=http://localhost:8095) as the page server.
// The smoke test then drives a real Chromium against the real SPA + real API.
//
// Run: npx playwright test  (chromium must be installed: npx playwright install chromium)

import { defineConfig, devices } from "@playwright/test";

const PREVIEW_PORT = 4317; // dashboard dev server for the test (avoids 5173 collisions)
const API_PORT = 8095; // Go control plane (operator-mandated test port)

export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [["list"]],
  globalSetup: "./e2e/global-setup.ts",
  globalTeardown: "./e2e/global-teardown.ts",
  use: {
    baseURL: `http://localhost:${PREVIEW_PORT}`,
    trace: "on-first-retry",
    screenshot: "only-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    // Vite dev server with the /api proxy pointed at the test control plane.
    command: `npx vite --port ${PREVIEW_PORT} --strictPort`,
    url: `http://localhost:${PREVIEW_PORT}`,
    timeout: 60_000,
    reuseExistingServer: false,
    env: {
      HELIX_API_TARGET: `http://localhost:${API_PORT}`,
    },
  },
});
