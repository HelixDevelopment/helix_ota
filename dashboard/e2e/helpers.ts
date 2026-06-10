// Helix OTA dashboard — shared e2e helpers.
// Login flow + API-driven seeding against the REAL in-memory control plane booted
// in global-setup. Seeding uses the same /api/v1 endpoints the SPA uses (admin token),
// so the UI assertions run against genuine server state — no mocks (Constitution §11.4).

import { createHash, createPrivateKey, sign as cryptoSign } from "node:crypto";
import { readFileSync } from "node:fs";
import { join } from "node:path";
import { expect, type APIRequestContext, type Page } from "@playwright/test";

// Same constants as e2e/global-setup.ts (test-only, non-secret).
export const USER = "admin@helix.example";
export const PASS = "e2e-smoke-pass-1234";

// The control plane the Vite dev server proxies /api to (global-setup boots it here).
export const API_BASE = "http://localhost:8095/api/v1";

export async function login(page: Page): Promise<void> {
  await page.goto("/login");
  await expect(page.getByText("Helix OTA — operator login")).toBeVisible();
  await page.getByPlaceholder("operator@example.com").fill(USER);
  await page.locator('input[type="password"]').fill(PASS);
  await page.getByRole("button", { name: "Sign in" }).click();
  // Landed on Overview -> the shell is up.
  await expect(page.getByRole("heading", { name: "Overview", level: 1 })).toBeVisible();
}

// Obtain an admin access token directly from the API for seeding.
export async function adminToken(request: APIRequestContext): Promise<string> {
  const res = await request.post(`${API_BASE}/auth/login`, {
    data: { username: USER, password: PASS },
  });
  expect(res.ok(), `login for seeding failed: ${res.status()}`).toBeTruthy();
  const body = (await res.json()) as { access_token: string };
  expect(body.access_token, "no access_token in login response").toBeTruthy();
  return body.access_token;
}

// Create a device group via the real API; returns its server-issued group_id.
export async function seedGroup(
  request: APIRequestContext,
  token: string,
  name: string,
  description?: string,
): Promise<string> {
  const res = await request.post(`${API_BASE}/groups`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { name, ...(description ? { description } : {}) },
  });
  expect(res.ok(), `group create failed: ${res.status()} ${await res.text()}`).toBeTruthy();
  const body = (await res.json()) as { group_id: string };
  expect(body.group_id).toBeTruthy();
  return body.group_id;
}

// --- signed-artifact seeding (closes the populated-detail e2e gap) -----------
// The real server verifies every artifact: ed25519 over the raw 32 SHA-256 digest
// bytes, under the trusted key configured via HELIX_ARTIFACT_PUBKEY. global-setup
// mints that ephemeral keypair and stashes the private key here; these helpers mint
// a genuinely-signed artifact so the e2e can reach a POPULATED release/deployment
// detail against real server state — no mocks (Constitution §11.4 / §11.4.27).

const KEY_FILE = join(process.cwd(), ".e2e-artifact-key.json");

interface SignedArtifactSeed {
  artifactId: string;
  sha256: string;
  version: string;
  os: string;
  targetModel: string;
}

// CRC-32 (IEEE 802.3) — needed for the ZIP_STORED local/central directory entries.
function crc32(buf: Buffer): number {
  let crc = 0xffffffff;
  for (let i = 0; i < buf.length; i++) {
    crc ^= buf[i];
    for (let b = 0; b < 8; b++) {
      crc = crc & 1 ? (crc >>> 1) ^ 0xedb88320 : crc >>> 1;
    }
  }
  return (crc ^ 0xffffffff) >>> 0;
}

// Build a minimal, valid ZIP archive with a single STORED (uncompressed) entry.
// The server's S1 structure check requires a readable ZIP whose payload.bin (if
// present) is ZIP_STORED (handlers_artifact.go validateStructure).
function buildZipStored(entryName: string, payload: Buffer): Buffer {
  const nameBuf = Buffer.from(entryName, "utf8");
  const crc = crc32(payload);
  const size = payload.length;

  const local = Buffer.alloc(30);
  local.writeUInt32LE(0x04034b50, 0); // local file header signature
  local.writeUInt16LE(20, 4); // version needed
  local.writeUInt16LE(0, 6); // flags
  local.writeUInt16LE(0, 8); // method 0 = STORED
  local.writeUInt16LE(0, 10); // mod time
  local.writeUInt16LE(0, 12); // mod date
  local.writeUInt32LE(crc, 14);
  local.writeUInt32LE(size, 18); // compressed size
  local.writeUInt32LE(size, 22); // uncompressed size
  local.writeUInt16LE(nameBuf.length, 26);
  local.writeUInt16LE(0, 28); // extra len
  const localHeader = Buffer.concat([local, nameBuf, payload]);

  const central = Buffer.alloc(46);
  central.writeUInt32LE(0x02014b50, 0); // central dir header signature
  central.writeUInt16LE(20, 4); // version made by
  central.writeUInt16LE(20, 6); // version needed
  central.writeUInt16LE(0, 8); // flags
  central.writeUInt16LE(0, 10); // method STORED
  central.writeUInt16LE(0, 12);
  central.writeUInt16LE(0, 14);
  central.writeUInt32LE(crc, 16);
  central.writeUInt32LE(size, 20);
  central.writeUInt32LE(size, 24);
  central.writeUInt16LE(nameBuf.length, 28);
  central.writeUInt16LE(0, 30); // extra len
  central.writeUInt16LE(0, 32); // comment len
  central.writeUInt16LE(0, 34); // disk number
  central.writeUInt16LE(0, 36); // internal attrs
  central.writeUInt32LE(0, 38); // external attrs
  central.writeUInt32LE(0, 42); // local header offset
  const centralHeader = Buffer.concat([central, nameBuf]);

  const eocd = Buffer.alloc(22);
  eocd.writeUInt32LE(0x06054b50, 0); // EOCD signature
  eocd.writeUInt16LE(0, 4);
  eocd.writeUInt16LE(0, 6);
  eocd.writeUInt16LE(1, 8); // entries on this disk
  eocd.writeUInt16LE(1, 10); // total entries
  eocd.writeUInt32LE(centralHeader.length, 12); // central dir size
  eocd.writeUInt32LE(localHeader.length, 16); // central dir offset
  eocd.writeUInt16LE(0, 20); // comment len

  return Buffer.concat([localHeader, centralHeader, eocd]);
}

// Mint + upload a GENUINELY signed artifact through POST /artifacts/upload and
// assert the real server verified it (.verified == true). Returns the seed.
export async function seedArtifact(
  request: APIRequestContext,
  token: string,
  opts: { version: string; os?: string; targetModel?: string; payload?: string },
): Promise<SignedArtifactSeed> {
  const os = opts.os ?? "android";
  const targetModel = opts.targetModel ?? "OrangePi5Max";

  // Load the ephemeral private key the harness minted in global-setup.
  const keyJson = JSON.parse(readFileSync(KEY_FILE, "utf8")) as { privateKeyPem: string };
  const privateKey = createPrivateKey(keyJson.privateKeyPem);

  const payload = Buffer.from(
    opts.payload ?? `helix-ota e2e payload ${opts.version} ${Date.now()}`,
    "utf8",
  );
  const zip = buildZipStored("payload.bin", payload);

  // S2: lowercase-hex SHA-256 over the WHOLE zip bytes.
  const sha256hex = createHash("sha256").update(zip).digest("hex");
  // S3: ed25519 detached signature over the RAW 32 digest bytes (NOT the file).
  const digestBytes = Buffer.from(sha256hex, "hex");
  const signatureB64 = cryptoSign(null, digestBytes, privateKey).toString("base64");

  const metadata = {
    sha256: sha256hex,
    signature: signatureB64,
    version: opts.version,
    os,
    target_model: targetModel,
    file_hash: Buffer.from("file-hash").toString("base64"),
    file_size: zip.length,
    metadata_hash: Buffer.from("meta-hash").toString("base64"),
    metadata_size: 64,
    payload_offset: 0,
    payload_size: payload.length,
  };

  const res = await request.post(`${API_BASE}/artifacts/upload`, {
    headers: { Authorization: `Bearer ${token}` },
    multipart: {
      file: { name: "ota.zip", mimeType: "application/zip", buffer: zip },
      metadata: JSON.stringify(metadata),
    },
  });
  expect(
    res.status(),
    `signed artifact upload failed: ${res.status()} ${await res.text()}`,
  ).toBe(201);
  const body = (await res.json()) as { artifact_id: string; sha256: string; verified: boolean };
  expect(body.verified, "server did not report the seeded artifact as verified").toBe(true);
  expect(body.artifact_id).toBeTruthy();
  return {
    artifactId: body.artifact_id,
    sha256: body.sha256,
    version: opts.version,
    os,
    targetModel,
  };
}

// Create a release from a (signed, verified) artifact; returns its release_id.
export async function seedRelease(
  request: APIRequestContext,
  token: string,
  seed: SignedArtifactSeed,
  notes = "e2e seeded release",
): Promise<string> {
  const res = await request.post(`${API_BASE}/releases`, {
    headers: { Authorization: `Bearer ${token}` },
    data: {
      artifact_id: seed.artifactId,
      version: seed.version,
      os: seed.os,
      target_model: seed.targetModel,
      notes,
    },
  });
  expect(res.status(), `release create failed: ${res.status()} ${await res.text()}`).toBe(201);
  const body = (await res.json()) as { release_id: string };
  expect(body.release_id).toBeTruthy();
  return body.release_id;
}

// Create an all-targets deployment for a release; returns its deployment_id.
export async function seedDeployment(
  request: APIRequestContext,
  token: string,
  releaseId: string,
): Promise<string> {
  const res = await request.post(`${API_BASE}/deployments`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { release_id: releaseId, strategy: "all-targets" },
  });
  expect(res.status(), `deployment create failed: ${res.status()} ${await res.text()}`).toBe(201);
  const body = (await res.json()) as { deployment_id: string };
  expect(body.deployment_id).toBeTruthy();
  return body.deployment_id;
}

// Create + start a staged rollout for a deployment so the rollout panel shows real
// phase data; returns nothing (the e2e asserts the rendered table).
export async function seedRollout(
  request: APIRequestContext,
  token: string,
  deploymentId: string,
): Promise<void> {
  const res = await request.post(
    `${API_BASE}/deployments/${encodeURIComponent(deploymentId)}/rollout`,
    {
      headers: { Authorization: `Bearer ${token}` },
      data: {
        phases: [
          { percentage: 10, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 3600, auto_progress: false },
          { percentage: 50, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 3600, auto_progress: false },
          { percentage: 100, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 0, auto_progress: false },
        ],
      },
    },
  );
  expect(res.status(), `rollout create failed: ${res.status()} ${await res.text()}`).toBe(201);
}

// --- device + telemetry seeding (closes the Fleet device-detail e2e gap) -----
// Register a real device (admin path) and ingest a few telemetry events for it
// through the SAME device-token endpoints a real device uses, so the Fleet
// device-detail screen (DeviceDetail) renders a POPULATED status card + a
// newest-first telemetry history table against genuine server state — no mocks
// (Constitution §11.4 / §11.4.27 / §11.4.69).
//
// Contract replicated from the server:
//  - POST /devices/register (admin token) -> { device_id, device_token } (201).
//  - POST /client/telemetry (the device_token) ingests a batch; the schema
//    validator REQUIRES a non-empty deployment_id (ota-protocol
//    ValidateTelemetryReport), but the telemetry path does NOT FK-check it, so a
//    placeholder is accepted and the events are stored (202 Accepted).
//  - GET /devices/{id}/telemetry is newest-first (insertion order reversed): the
//    LAST event in `events` renders at the TOP of the rendered table.

export interface SeededDevice {
  deviceId: string;
  deviceToken: string;
  hardwareId: string;
  model: string;
  // Events as posted (chronological). The history table renders these reversed
  // (newest-first), so events[events.length - 1] is the top row.
  events: { event: string; version?: string; detail?: string }[];
}

// Register a device via the admin REST API; returns its id + device-scoped token.
export async function seedDevice(
  request: APIRequestContext,
  token: string,
  opts: { hardwareId: string; model?: string; os?: string; currentVersion?: string },
): Promise<{ deviceId: string; deviceToken: string; hardwareId: string; model: string }> {
  const model = opts.model ?? "OrangePi5Max";
  const os = opts.os ?? "android";
  const res = await request.post(`${API_BASE}/devices/register`, {
    headers: { Authorization: `Bearer ${token}` },
    data: {
      hardware_id: opts.hardwareId,
      model,
      os,
      ...(opts.currentVersion ? { current_version: opts.currentVersion } : {}),
    },
  });
  expect(
    res.status(),
    `device register failed: ${res.status()} ${await res.text()}`,
  ).toBe(201);
  const body = (await res.json()) as { device_id: string; device_token: string };
  expect(body.device_id, "no device_id in register response").toBeTruthy();
  expect(body.device_token, "no device_token in register response").toBeTruthy();
  return { deviceId: body.device_id, deviceToken: body.device_token, hardwareId: opts.hardwareId, model };
}

// Ingest a batch of telemetry events for a device through its device token (the
// real /client/telemetry endpoint). Asserts the server accepted them all (202,
// accepted == events.length). `events` are posted in array order (chronological);
// the history endpoint renders them newest-first.
export async function ingestTelemetry(
  request: APIRequestContext,
  deviceToken: string,
  events: { event: string; version?: string; detail?: string; errorCode?: string; timestamp?: string }[],
  opts?: { deploymentId?: string },
): Promise<void> {
  const deploymentId = opts?.deploymentId ?? "dep-e2e-seed";
  const base = Date.now();
  const wireEvents = events.map((e, i) => ({
    event: e.event,
    ...(e.version ? { version: e.version } : {}),
    ...(e.detail ? { detail: e.detail } : {}),
    ...(e.errorCode ? { error_code: e.errorCode } : {}),
    // Distinct, monotonically-increasing device-reported timestamps so the
    // newest-first ordering is unambiguous.
    timestamp: e.timestamp ?? new Date(base + i * 1000).toISOString(),
  }));
  const res = await request.post(`${API_BASE}/client/telemetry`, {
    headers: { Authorization: `Bearer ${deviceToken}` },
    data: { deployment_id: deploymentId, events: wireEvents },
  });
  expect(
    res.status(),
    `telemetry ingest failed: ${res.status()} ${await res.text()}`,
  ).toBe(202);
  const ack = (await res.json()) as { accepted: number; rejected: number };
  expect(ack.rejected, `server rejected ${ack.rejected} seeded telemetry events`).toBe(0);
  expect(ack.accepted, "server accepted no telemetry events").toBe(events.length);
}

// One-shot: register a device and ingest a few telemetry events, returning enough
// for the e2e to assert the populated Fleet device-detail render. The last event
// is a `success` so the device's last-known update_state + current_version reflect
// it (the server's applyDeviceRuntime promotes the latest event).
export async function seedDeviceWithTelemetry(
  request: APIRequestContext,
  token: string,
  opts: { hardwareId: string; model?: string; version?: string },
): Promise<SeededDevice> {
  const version = opts.version ?? "1.4.0";
  const dev = await seedDevice(request, token, {
    hardwareId: opts.hardwareId,
    model: opts.model,
    currentVersion: "1.3.0",
  });
  const events = [
    { event: "download_started", version, detail: "e2e seeded download" },
    { event: "installing", version, detail: "e2e seeded install" },
    { event: "success", version, detail: "e2e seeded success" },
  ];
  await ingestTelemetry(request, dev.deviceToken, events);
  return { ...dev, events };
}

// Add device ids to a group via the real batch endpoint; returns the disposition.
export async function seedGroupMembers(
  request: APIRequestContext,
  token: string,
  groupId: string,
  deviceIds: string[],
): Promise<{ added: string[]; already_member: string[]; not_found: string[] }> {
  const res = await request.post(`${API_BASE}/groups/${encodeURIComponent(groupId)}/members`, {
    headers: { Authorization: `Bearer ${token}` },
    data: { device_ids: deviceIds },
  });
  expect(res.ok(), `member add failed: ${res.status()} ${await res.text()}`).toBeTruthy();
  return (await res.json()) as {
    added: string[];
    already_member: string[];
    not_found: string[];
  };
}
