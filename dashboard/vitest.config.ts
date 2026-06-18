// Helix OTA dashboard — Vitest unit/component test config (jsdom + RTL).
// Component layer complements the Playwright e2e suite: fast, in-process React
// rendering and pure-logic unit tests. Anti-bluff: every test asserts real
// user-visible behaviour / real client logic (mocks are unit-test-only per §11.4.27).

import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    // Vitest must not pick up the Playwright e2e specs (they need a real browser).
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    exclude: ["e2e/**", "node_modules/**", "dist/**"],
    css: false,
  },
});
