// Helix OTA dashboard — Playwright global setup.
// Builds + boots the REAL Go control plane on HELIX_PORT=8095 (in-memory, admin
// seeded), waits for /healthz, and stashes the child PID for teardown. No mocks:
// the smoke test exercises the real SPA against the real API (Constitution §11.4).

import { spawn, spawnSync } from "node:child_process";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import { setTimeout as sleep } from "node:timers/promises";

const API_PORT = 8095;
const ADMIN_USER = "admin@helix.example";
const ADMIN_PASS = "e2e-smoke-pass-1234";
const SERVER_DIR = join(process.cwd(), "..", "server");
const BIN_PATH = join(process.cwd(), ".e2e-ota-server"); // gitignored build output
const PID_FILE = join(process.cwd(), ".e2e-server.pid");

async function waitForHealth(url: string, attempts = 60): Promise<boolean> {
  for (let i = 0; i < attempts; i++) {
    try {
      const res = await fetch(url);
      if (res.ok) return true;
    } catch {
      // not up yet
    }
    await sleep(300);
  }
  return false;
}

export default async function globalSetup(): Promise<void> {
  // Build the Go server binary from the server module.
  const build = spawnSync("go", ["build", "-o", BIN_PATH, "./cmd/ota-server"], {
    cwd: SERVER_DIR,
    stdio: "inherit",
  });
  if (build.status !== 0) {
    throw new Error(`go build failed with status ${build.status}`);
  }

  // Boot the control plane (in-memory; admin login seeded via env).
  const child = spawn(BIN_PATH, [], {
    env: {
      ...process.env,
      HELIX_PORT: String(API_PORT),
      HELIX_ADMIN_USERNAME: ADMIN_USER,
      HELIX_ADMIN_PASSWORD: ADMIN_PASS,
    },
    stdio: "ignore",
    detached: true,
  });
  child.unref();
  if (child.pid) writeFileSync(PID_FILE, String(child.pid), "utf8");

  // Expose the test credentials to the spec via env.
  process.env.HELIX_E2E_USER = ADMIN_USER;
  process.env.HELIX_E2E_PASS = ADMIN_PASS;

  const healthy = await waitForHealth(`http://localhost:${API_PORT}/healthz`);
  if (!healthy) {
    throw new Error(`control plane did not become healthy on :${API_PORT}`);
  }
}
