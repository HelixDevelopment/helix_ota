// Helix OTA dashboard — POPULATED Fleet device-detail e2e (real SPA + real control
// plane). Closes the documented gap: previously the per-device Fleet detail
// success path was covered only at the empty/404 level because it needs a SEEDED
// device with telemetry. seedDeviceWithTelemetry now registers a real device and
// ingests real telemetry through the SAME device-token endpoints a real device
// uses, so this spec drives the DeviceDetail screen to a populated render against
// genuine server state — no mocks (Constitution §11.4 / §11.4.27 / §11.4.69).
//
// Navigation is CLIENT-SIDE (the Fleet nav link + the in-app "Open a device"
// lookup form) so the in-memory session survives (AuthContext §7.1). Every
// rendered value traces to genuine server state seeded via the real /api/v1 +
// /client/telemetry endpoints.

import { expect, test } from "@playwright/test";
import { adminToken, login, seedDeviceWithTelemetry } from "./helpers";

test("Fleet device-detail renders a populated status card + newest-first telemetry history", async ({
  page,
  request,
}) => {
  const token = await adminToken(request);
  // A unique hardware_id avoids a 409 conflict with sibling seeded devices.
  const hardwareId = `fleet-detail-hw-${Date.now()}`;
  const version = `4.${Date.now() % 100000}.0`;
  const dev = await seedDeviceWithTelemetry(request, token, { hardwareId, version });

  await login(page);

  // Open the device via the in-app Fleet "Open a device" lookup (client-side nav).
  await page.getByRole("link", { name: "Fleet" }).click();
  await expect(page.getByRole("heading", { name: "Fleet", level: 1 })).toBeVisible();
  await page.getByPlaceholder("dev_…").fill(dev.deviceId);
  await page.getByRole("button", { name: "Open device" }).click();

  // Landed on the populated DeviceDetail.
  await expect(
    page.getByRole("heading", { name: `Device ${dev.deviceId}`, level: 1 }),
  ).toBeVisible();

  // The Status card heading renders (the screen mounted without crashing — this
  // regression-guards the health-object render bug surfaced + fixed in this wave).
  await expect(page.getByRole("heading", { name: "Status" })).toBeVisible();

  // --- Status card: the success path (data present, no error panel). ---------
  // The last seeded event is `success` at `version`, which the server's
  // applyDeviceRuntime promotes into the device's last-known runtime status.
  await expect(page.getByText("current_version")).toBeVisible();
  // current_version was promoted to the success event's version.
  await expect(page.getByText(version, { exact: true }).first()).toBeVisible();
  // update_state badge shows the latest event (success).
  await expect(page.getByText("update_state")).toBeVisible();
  await expect(page.getByText("success", { exact: true }).first()).toBeVisible();
  // last_seen + health rows render (the device has been seen via telemetry).
  await expect(page.getByText("last_seen")).toBeVisible();
  // health is an OBJECT { ok, last_error_code } — the screen projects it to a
  // badge. A `success` terminal event leaves the device healthy -> "ok".
  await expect(page.getByText("health")).toBeVisible();
  const statusCard = page
    .locator("section, div")
    .filter({ has: page.getByRole("heading", { name: "Status" }) });
  await expect(statusCard.getByText("ok", { exact: true })).toBeVisible();

  // --- Telemetry history table: populated, newest-first, empty-state gone. ----
  await expect(page.getByRole("heading", { name: "Telemetry history" })).toBeVisible();
  // The empty-state ("No telemetry events yet.") must be ABSENT.
  await expect(page.getByText("No telemetry events yet.")).toHaveCount(0);
  // The 404 empty-state must also be absent (the endpoint returned real data).
  await expect(
    page.getByText("Per-device telemetry history is not available (endpoint returned 404)."),
  ).toHaveCount(0);

  // All three seeded events appear as rows. The history is rendered inside the
  // "Telemetry history" card's table; scope assertions to that table.
  const historyTable = page
    .locator("section, div")
    .filter({ has: page.getByRole("heading", { name: "Telemetry history" }) })
    .getByRole("table");
  // Event badges render (exact match: "success" the event-type vs the
  // "e2e seeded success" detail cell are disambiguated by exact:true).
  await expect(historyTable.getByText("download_started", { exact: true })).toBeVisible();
  await expect(historyTable.getByText("installing", { exact: true })).toBeVisible();
  await expect(historyTable.getByText("success", { exact: true })).toBeVisible();
  // Each seeded detail renders in its row.
  await expect(historyTable.getByText("e2e seeded download")).toBeVisible();
  await expect(historyTable.getByText("e2e seeded install")).toBeVisible();
  await expect(historyTable.getByText("e2e seeded success")).toBeVisible();

  // Newest-first ordering proof: the LAST-posted event (`success`) is the FIRST
  // data row in the table; the FIRST-posted event (`download_started`) is last.
  const rows = historyTable.getByRole("row");
  // Row 0 is the header (event/version/timestamp/detail); rows 1..3 are data.
  await expect(rows.nth(1)).toContainText("success");
  await expect(rows.nth(1)).toContainText("e2e seeded success");
  await expect(rows.nth(3)).toContainText("download_started");
  await expect(rows.nth(3)).toContainText("e2e seeded download");

  // The populated success path renders NO error alert.
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("seeded device + telemetry is retrievable + populated via the real device-detail API path", async ({
  request,
}) => {
  // Sink-side proof (§11.4.69): the server itself confirms the seeded device's
  // status reflects the latest event AND its telemetry history is non-empty +
  // newest-first — the seeding was genuine, not faked.
  const token = await adminToken(request);
  const hardwareId = `fleet-detail-api-hw-${Date.now()}`;
  const version = `5.${Date.now() % 100000}.0`;
  const dev = await seedDeviceWithTelemetry(request, token, { hardwareId, version });

  // Device status: the success event was promoted into last-known runtime state.
  const statusRes = await request.get(
    `http://localhost:8095/api/v1/devices/${encodeURIComponent(dev.deviceId)}/status`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  expect(statusRes.status()).toBe(200);
  const status = (await statusRes.json()) as {
    device_id: string;
    current_version: string;
    update_state: string;
  };
  expect(status.device_id).toBe(dev.deviceId);
  expect(status.update_state).toBe("success");
  expect(status.current_version).toBe(version);

  // Telemetry history: all three seeded events present, newest-first.
  const histRes = await request.get(
    `http://localhost:8095/api/v1/devices/${encodeURIComponent(dev.deviceId)}/telemetry`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  expect(histRes.status()).toBe(200);
  const hist = (await histRes.json()) as {
    device_id: string;
    items: { event: string; detail?: string }[];
  };
  expect(hist.device_id).toBe(dev.deviceId);
  expect(hist.items.length).toBe(3);
  // Newest-first: the last-posted event (success) is items[0].
  expect(hist.items[0].event).toBe("success");
  expect(hist.items[2].event).toBe("download_started");
});
