// Helix OTA dashboard — §11.4.170 HOST-RENDER visual-proof config.
//
// Renders real component pixels ON THE HOST (headless Chromium against the
// vite-served SPA) with NO device, NO emulator, NO running Go backend. The
// /login screen renders fully client-side (it only calls /api on submit), so
// this harness needs ONLY the vite dev server — making the proof genuinely
// device- AND backend-independent per §11.4.170.
//
// Distinct from playwright.config.ts (the full e2e harness that boots the Go
// control plane in globalSetup). Run:
//   npx playwright test --config=playwright.hostrender.config.ts
//
// The golden baselines live under hostrender/*-snapshots/ (committed in-repo,
// dashboard scope). Deterministic framing: fixed 1280x800 viewport, dsf=1,
// animations disabled.

import { defineConfig, devices } from "@playwright/test";

const PORT = 4318; // distinct from the full-e2e config's 4317

export default defineConfig({
  testDir: "./hostrender",
  timeout: 60_000,
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [["list"]],
  // NO globalSetup: the login screen renders without the backend (§11.4.170
  // device/backend-independent host pixels).
  use: {
    baseURL: `http://localhost:${PORT}`,
    viewport: { width: 1280, height: 800 },
    deviceScaleFactor: 1,
    screenshot: "only-on-failure",
  },
  expect: {
    // Small AA/subpixel tolerance for the idiomatic toHaveScreenshot golden-good
    // (same-host renders); the explicit pixelmatch self-validation test uses its
    // own calibrated thresholds.
    toHaveScreenshot: { maxDiffPixelRatio: 0.01, animations: "disabled" },
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: {
    command: `npx vite --port ${PORT} --strictPort`,
    url: `http://localhost:${PORT}`,
    timeout: 60_000,
    reuseExistingServer: false,
    env: {
      // Login render never hits /api; a dummy proxy target keeps vite quiet.
      HELIX_API_TARGET: "http://localhost:9",
    },
  },
});
