// Helix OTA dashboard — Playwright global teardown.
// Stops the Go control plane started in global-setup and removes the pid file.

import { existsSync, readFileSync, rmSync } from "node:fs";
import { join } from "node:path";

const PID_FILE = join(process.cwd(), ".e2e-server.pid");
const KEY_FILE = join(process.cwd(), ".e2e-artifact-key.json");

export default async function globalTeardown(): Promise<void> {
  // Always remove the ephemeral signing key (§11.4.10 — never lingers/committed).
  rmSync(KEY_FILE, { force: true });
  if (!existsSync(PID_FILE)) return;
  try {
    const pid = Number(readFileSync(PID_FILE, "utf8").trim());
    if (pid > 0) {
      try {
        process.kill(pid, "SIGTERM");
      } catch {
        // already gone
      }
    }
  } finally {
    rmSync(PID_FILE, { force: true });
  }
}
