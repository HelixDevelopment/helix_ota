// Helix OTA dashboard — component test for the DeviceDetail (Fleet device-detail)
// screen. Drives the screen through its loading / empty / populated / 404 /
// error states by stubbing the apiClient module (unit/component-test mock,
// permitted §11.4.27).
//
// REGRESSION GUARD (§11.4.1/§11.4.108): the populated test feeds a REAL
// DeviceStatus whose `health` is the wire OBJECT { ok, last_error_code }. Before
// this wave the type was wrongly `health?: string` and the screen rendered the
// object directly into a <dd>, throwing "Objects are not valid as a React child"
// and blanking the page. The populated test asserts the screen mounts + projects
// health to text — the bug the e2e surfaced, locked at the component layer.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { ApiError } from "../api/client";
import type { DeviceStatus, TelemetryHistory } from "../types/api";

const getDeviceStatus = vi.fn<[string], Promise<DeviceStatus>>();
const getDeviceTelemetry = vi.fn<[string], Promise<TelemetryHistory>>();
vi.mock("../api/client", async () => {
  const actual = await vi.importActual<typeof import("../api/client")>("../api/client");
  return {
    ...actual,
    apiClient: {
      getDeviceStatus: (...args: unknown[]) => getDeviceStatus(...(args as [string])),
      getDeviceTelemetry: (...args: unknown[]) => getDeviceTelemetry(...(args as [string])),
    },
  };
});

// Import AFTER the mock is registered.
import { DeviceDetail } from "./FleetScreen";

const DEVICE_ID = "dev_abc123";

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={[`/fleet/${DEVICE_ID}`]}>
      <Routes>
        <Route path="/fleet/:deviceId" element={<DeviceDetail />} />
      </Routes>
    </MemoryRouter>,
  );
}

const populatedStatus: DeviceStatus = {
  device_id: DEVICE_ID,
  current_version: "1.4.0",
  target_version: "1.5.0",
  last_seen: "2026-06-10T12:00:00Z",
  update_state: "success",
  active_slot: "b",
  // The wire shape: an OBJECT, not a string. This is the regression-guard fixture.
  health: { ok: true, last_error_code: null },
};

const populatedTelemetry: TelemetryHistory = {
  device_id: DEVICE_ID,
  items: [
    // newest-first (as the server returns)
    {
      event: "success",
      version: "1.4.0",
      detail: "applied ok",
      timestamp: "2026-06-10T12:00:02Z",
      received_at: "2026-06-10T12:00:02Z",
    },
    {
      event: "download_started",
      version: "1.4.0",
      detail: "started",
      timestamp: "2026-06-10T12:00:00Z",
      received_at: "2026-06-10T12:00:00Z",
    },
  ],
  next_cursor: null,
};

describe("DeviceDetail (Fleet device-detail)", () => {
  beforeEach(() => {
    getDeviceStatus.mockReset();
    getDeviceTelemetry.mockReset();
  });

  it("shows the Loading… placeholders before status + telemetry resolve", () => {
    getDeviceStatus.mockReturnValue(new Promise(() => {}));
    getDeviceTelemetry.mockReturnValue(new Promise(() => {}));
    renderDetail();
    expect(
      screen.getByRole("heading", { name: `Device ${DEVICE_ID}`, level: 1 }),
    ).toBeInTheDocument();
    // Both cards show their loading placeholder.
    expect(screen.getAllByText("Loading…").length).toBeGreaterThanOrEqual(1);
  });

  it("renders a populated status card (health OBJECT projected) + newest-first telemetry table", async () => {
    getDeviceStatus.mockResolvedValue(populatedStatus);
    getDeviceTelemetry.mockResolvedValue(populatedTelemetry);
    renderDetail();

    // Status card fields render — and crucially the screen did NOT crash on the
    // health object (the bug this guards).
    await waitFor(() => expect(screen.getByText("current_version")).toBeInTheDocument());
    // current_version (1.4.0) appears in the status <dl> AND the telemetry table —
    // scope to the status definition list.
    const dl = screen.getByText("current_version").closest("dl")!;
    expect(dl).toHaveTextContent("1.4.0");
    expect(dl).toHaveTextContent("1.5.0"); // target_version
    // health is projected to an "ok" badge (NOT rendered as a raw object).
    expect(screen.getByText("health")).toBeInTheDocument();
    expect(dl).toHaveTextContent("ok");

    // Telemetry history table is populated, newest-first.
    const table = screen.getByRole("table");
    const rows = table.querySelectorAll("tbody tr");
    expect(rows.length).toBe(2);
    // Row 0 is the newest event (success), row 1 the oldest (download_started).
    expect(rows[0].textContent).toContain("success");
    expect(rows[0].textContent).toContain("applied ok");
    expect(rows[1].textContent).toContain("download_started");
    expect(rows[1].textContent).toContain("started");
    // No empty-state, no error alert.
    expect(screen.queryByText("No telemetry events yet.")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders the empty telemetry state when the history is empty", async () => {
    getDeviceStatus.mockResolvedValue(populatedStatus);
    getDeviceTelemetry.mockResolvedValue({ device_id: DEVICE_ID, items: [], next_cursor: null });
    renderDetail();
    await waitFor(() => expect(screen.getByText("No telemetry events yet.")).toBeInTheDocument());
    // The status card still rendered (no table for telemetry though).
    expect(screen.getByText("current_version")).toBeInTheDocument();
  });

  it("degrades to the 404 empty state when per-device telemetry is unavailable", async () => {
    getDeviceStatus.mockResolvedValue(populatedStatus);
    getDeviceTelemetry.mockRejectedValue(new ApiError(404, "NOT_FOUND", "no telemetry", "req-1"));
    renderDetail();
    await waitFor(() =>
      expect(
        screen.getByText("Per-device telemetry history is not available (endpoint returned 404)."),
      ).toBeInTheDocument(),
    );
    // A 404 is a graceful empty state, NOT an error alert.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("surfaces a device-status ApiError via the error panel", async () => {
    getDeviceStatus.mockRejectedValue(new ApiError(500, "INTERNAL", "boom", "req-2"));
    getDeviceTelemetry.mockResolvedValue({ device_id: DEVICE_ID, items: [], next_cursor: null });
    renderDetail();
    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("500 INTERNAL");
      expect(alert).toHaveTextContent("boom");
    });
  });
});
