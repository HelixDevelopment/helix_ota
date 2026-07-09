import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";

// §11.4.170 host-render harness build config. Builds visual/harness.html to a
// static bundle in visual/.out — no dev server, no HMR, pure static output that
// Playwright renders to PNG on the host.
export default defineConfig({
  root: __dirname,
  base: "./",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "../src"),
    },
  },
  build: {
    outDir: path.resolve(__dirname, ".out"),
    emptyOutDir: true,
    rollupOptions: {
      input: path.resolve(__dirname, "harness.html"),
    },
  },
});
