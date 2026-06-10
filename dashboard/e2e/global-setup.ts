// Helix OTA dashboard — Playwright global setup.
// Builds + boots the REAL Go control plane on HELIX_PORT=8095 (in-memory, admin
// seeded), waits for /healthz, and stashes the child PID for teardown. No mocks:
// the smoke test exercises the real SPA against the real API (Constitution §11.4).

import { spawn, spawnSync } from "node:child_process";
import { generateKeyPairSync } from "node:crypto";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import { setTimeout as sleep } from "node:timers/promises";

const API_PORT = 8095;
const ADMIN_USER = "admin@helix.example";
const ADMIN_PASS = "e2e-smoke-pass-1234";
const SERVER_DIR = join(process.cwd(), "..", "server");
const BIN_PATH = join(process.cwd(), ".e2e-ota-server"); // gitignored build output
const PID_FILE = join(process.cwd(), ".e2e-server.pid");
// Ephemeral artifact-signing key (regenerated every run, gitignored, §11.4.10).
const KEY_FILE = join(process.cwd(), ".e2e-artifact-key.json");

// Mint an EPHEMERAL ed25519 keypair so the e2e harness can produce GENUINELY
// signed artifacts the real server verifies (the artifact-pipeline signing
// contract: raw 32-byte SPKI-tail pubkey configured via HELIX_ARTIFACT_PUBKEY;
// the detached signature is ed25519 over the raw 32 SHA-256 digest bytes — see
// tests/e2e/pipeline_signed.sh + ota-artifact-validator stages.go). The private
// key is written to a gitignored file and consumed by e2e/helpers.ts seedArtifact.
function mintArtifactSigningKey(): string {
  const { publicKey, privateKey } = generateKeyPairSync("ed25519");
  // The trusted public key the server expects = the last 32 bytes of the DER
  // SubjectPublicKeyInfo (the raw ed25519 point), base64-encoded.
  const rawPub = publicKey.export({ type: "spki", format: "der" }).subarray(-32);
  const pubKeyB64 = Buffer.from(rawPub).toString("base64");
  const privKeyPem = privateKey.export({ type: "pkcs8", format: "pem" }).toString();
  writeFileSync(KEY_FILE, JSON.stringify({ privateKeyPem: privKeyPem, publicKeyB64: pubKeyB64 }), {
    encoding: "utf8",
    mode: 0o600,
  });
  return pubKeyB64;
}

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

  // Mint the ephemeral signing key BEFORE boot so the server trusts it.
  const artifactPubKeyB64 = mintArtifactSigningKey();

  // Boot the control plane (in-memory; admin login seeded via env).
  const child = spawn(BIN_PATH, [], {
    env: {
      ...process.env,
      HELIX_PORT: String(API_PORT),
      HELIX_ADMIN_USERNAME: ADMIN_USER,
      HELIX_ADMIN_PASSWORD: ADMIN_PASS,
      // Trust the harness's ephemeral key so seedArtifact's signed uploads verify.
      HELIX_ARTIFACT_PUBKEY: artifactPubKeyB64,
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
