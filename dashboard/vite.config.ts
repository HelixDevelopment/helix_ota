import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Helix OTA dashboard — Vite SPA build (design §3.2: static, CDN-deployable;
// runtime API base URL injected at deploy time, never hard-coded).
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    // Dev-only proxy so the SPA talks to a locally-running control plane at /api/v1.
    proxy: {
      "/api": {
        target: process.env.HELIX_API_TARGET ?? "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    sourcemap: true,
  },
});
