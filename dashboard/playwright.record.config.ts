// Playwright config for recording only — uses already-running server on :8080.
import { defineConfig, devices } from "@playwright/test";

const PREVIEW_PORT = 5173; // Already-running Vite dev server

export default defineConfig({
  testDir: "./e2e",
  timeout: 120_000,
  fullyParallel: false,
  retries: 0,
  workers: 1,
  reporter: [["list"]],
  use: {
    baseURL: `http://localhost:${PREVIEW_PORT}`,
    screenshot: "on",
    video: "on", // Record viewport video for the full session
    viewport: { width: 1280, height: 900 },
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
});
