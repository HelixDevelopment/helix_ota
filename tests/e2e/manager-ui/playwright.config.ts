// tests/e2e/manager-ui/playwright.config.ts
//
// Playwright configuration for the OTA Manager end-to-end tests.
//
// These tests expect the ota-server to be running locally on port 8080
// (started via scripts/dev.sh or manually).  The SPA is either served by
// the Vite dev server (port 5173) or the embedded path at /manager on the
// ota-server (port 8080).
//
// Usage:
//   npx playwright test --config tests/e2e/manager-ui/playwright.config.ts
//
// Or with the project convenience script:
//   cd clients/ota-manager && pnpm test:e2e
//
// Environment:
//   BASE_URL — overrides the SPA URL (default http://localhost:5173)
//   API_URL  — overrides the API base URL (default http://localhost:8080/api/v1)
//   CI       — when set, uses headless mode with reduced timeouts

import { defineConfig, devices } from "@playwright/test";

const BASE_URL = process.env.BASE_URL || "http://localhost:5173";
const API_URL = process.env.API_URL || "http://localhost:8080/api/v1";
const isCI = !!process.env.CI;

export default defineConfig({
  // Directory containing the test files.
  testDir: "./e2e",

  // Fail the build on CI if you have no tests.
  forbidOnly: isCI,

  // Retry once on CI to handle flaky navigations.
  retries: isCI ? 1 : 0,

  // Limit parallel workers to 1 for E2E (shared server state).
  workers: 1,

  // Reporter configuration.
  reporter: [
    ["list"],
    ["html", { outputFolder: "playwright-report" }],
    ["json", { outputFile: "playwright-results.json" }],
  ],

  // Shared timeout for each test.
  timeout: isCI ? 60_000 : 30_000,

  use: {
    // Base URL for navigation — all test URLs are relative to this.
    baseURL: BASE_URL,

    // Capture trace on first retry for debugging.
    trace: isCI ? "on-first-retry" : "on",

    // Capture screenshot on failure.
    screenshot: "only-on-failure",

    // Video recording on failure.
    video: "retain-on-failure",
  },

  // Web server configuration: when CI is NOT set, expect the developer to
  // have the dev server running externally.  On CI we start both the Vite
  // dev server and the ota-server.
  webServer: isCI
    ? [
        {
          command: "cd clients/ota-manager && pnpm dev --port 5173",
          port: 5173,
          timeout: 30_000,
          reuseExistingServer: false,
        },
        {
          command:
            "cd server && go run ./cmd/ota-server -port 8080",
          port: 8080,
          timeout: 60_000,
          reuseExistingServer: false,
        },
      ]
    : [],

  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
    {
      name: "firefox",
      use: { ...devices["Desktop Firefox"] },
    },
    {
      name: "webkit",
      use: { ...devices["Desktop Safari"] },
    },
  ],
});
