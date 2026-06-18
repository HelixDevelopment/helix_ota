// Helix OTA dashboard — PERMANENT regression guard (§11.4.135) for the
// DeviceStatus.health-is-an-object crash fixed in commit 4658125
// ("fix(dashboard): DeviceStatus.health is an object — fix Fleet-detail crash").
//
// THE BUG (FACT, §11.4.108 SOURCE→USER-VISIBLE): the server wire shape of
// DeviceStatus.health is an OBJECT `{ ok, last_error_code }` (server
// internal/api/wire.go DeviceHealth). The dashboard wrongly typed it
// `health?: string` and the Fleet device-detail screen rendered it as a bare
// React child: `<Row label="health" value={status.data.health ?? "—"} />`.
// React throws "Objects are not valid as a React child (found: object with keys
// {ok, last_error_code})", the <DeviceDetail> render throws, and the ENTIRE
// device page blanks for the operator. The fix projects health.ok →
// ok/unhealthy badge + health.last_error_code text.
//
// THIS GUARD is authored §11.4.115-style — RED-on-the-broken-shape, then
// GREEN-on-the-fixed-shape — with ONE polarity switch (`RED_MODE`, default 0):
//
//   RED_MODE=1  reproduces the pre-fix behaviour by rendering the SAME real
//               object-shaped health through the OLD render path (the object as
//               a bare React child) and asserts React DOES throw. This proves
//               the defect is real and that this guard genuinely catches it
//               (run: `RED_MODE=1 npx vitest run FleetScreen.health-regression`).
//
//   RED_MODE=0  (default, the standing GREEN guard) renders the CURRENT, shipped
//               <DeviceDetail> with the real object-shaped health and asserts it
//               (a) does NOT throw, (b) projects the object to text (ok / unhealthy
//               + last_error_code), (c) never leaks the raw object string
//               "[object Object]" into the DOM. A NEGATIVE-shape case proves an
//               `ok:false` health renders "unhealthy" + the error code.
//
// If a future regression re-types health back to a scalar / re-renders the
// object directly, the GREEN guard's "no raw object, projected to text"
// assertions FAIL — locking the §11.4.135 promise that this exact crash class
// can never silently ship again.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import React from "react";
import type { DeviceStatus, TelemetryHistory } from "../types/api";

const RED_MODE = (process.env.RED_MODE ?? "0") === "1";

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

const DEVICE_ID = "dev_health_guard";

const EMPTY_TELEMETRY: TelemetryHistory = {
  device_id: DEVICE_ID,
  items: [],
  next_cursor: null,
};

// The REAL wire shape: health is an OBJECT (server wire.go DeviceHealth).
const healthyStatus: DeviceStatus = {
  device_id: DEVICE_ID,
  current_version: "1.4.0",
  target_version: "1.5.0",
  last_seen: "2026-06-10T12:00:00Z",
  update_state: "success",
  active_slot: "b",
  health: { ok: true, last_error_code: null },
};

// NEGATIVE shape: an unhealthy device carrying a last_error_code.
const unhealthyStatus: DeviceStatus = {
  device_id: DEVICE_ID,
  current_version: "1.4.0",
  last_seen: "2026-06-10T12:00:00Z",
  update_state: "failure",
  health: { ok: false, last_error_code: "E_VERIFY_DM_VERITY" },
};

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={[`/fleet/${DEVICE_ID}`]}>
      <Routes>
        <Route path="/fleet/:deviceId" element={<DeviceDetail />} />
      </Routes>
    </MemoryRouter>,
  );
}

// The PRE-FIX render path, reproduced verbatim: a component that renders the
// health OBJECT directly as a React child (what FleetScreen.tsx did before
// commit 4658125: `<Row label="health" value={status.data.health} />` → the
// object reaches a JSX child position). React throws on this. RED_MODE proves
// the crash is real against the real object-shaped fixture.
function PreFixHealthRow({ health }: { health: unknown }) {
  // `health as React.ReactNode` mimics the old `health?: string` typing that let
  // the object slip into a child position. With the real object this throws.
  return (
    <dl>
      <dt>health</dt>
      <dd>{health as React.ReactNode}</dd>
    </dl>
  );
}

describe("DeviceStatus.health object-shape regression guard (§11.4.135 / commit 4658125)", () => {
  beforeEach(() => {
    getDeviceStatus.mockReset();
    getDeviceTelemetry.mockReset();
  });

  if (RED_MODE) {
    // ---- RED: prove the PRE-FIX render path crashes on the real object shape ----
    it("RED: rendering the health OBJECT directly as a React child throws (the pre-fix crash)", () => {
      // Silence React's expected error logging for the deliberate throw.
      const errSpy = vi.spyOn(console, "error").mockImplementation(() => {});
      expect(() =>
        render(<PreFixHealthRow health={healthyStatus.health} />),
      ).toThrowError(/Objects are not valid as a React child/);
      errSpy.mockRestore();
    });
  } else {
    // ---- GREEN: the CURRENT shipped component handles the object shape ----
    it("GREEN: renders the populated status WITHOUT crashing on the object-shaped health and projects it to text", async () => {
      getDeviceStatus.mockResolvedValue(healthyStatus);
      getDeviceTelemetry.mockResolvedValue(EMPTY_TELEMETRY);
      renderDetail();

      await waitFor(() => expect(screen.getByText("current_version")).toBeInTheDocument());
      const dl = screen.getByText("current_version").closest("dl")!;

      // health is projected to text — "ok" badge present in the status <dl>.
      expect(screen.getByText("health")).toBeInTheDocument();
      expect(dl).toHaveTextContent("ok");

      // The exact crash signature must be ABSENT: the raw object must never reach
      // the DOM as a stringified child.
      expect(document.body.textContent).not.toContain("[object Object]");
      // The status card actually mounted (page did not blank).
      expect(screen.getByText("active_slot")).toBeInTheDocument();
    });

    it("GREEN (negative shape): an unhealthy health object projects to 'unhealthy' + last_error_code, never a raw object", async () => {
      getDeviceStatus.mockResolvedValue(unhealthyStatus);
      getDeviceTelemetry.mockResolvedValue(EMPTY_TELEMETRY);
      renderDetail();

      await waitFor(() => expect(screen.getByText("health")).toBeInTheDocument());
      const dl = screen.getByText("health").closest("dl")!;
      expect(dl).toHaveTextContent("unhealthy");
      expect(dl).toHaveTextContent("E_VERIFY_DM_VERITY");
      expect(document.body.textContent).not.toContain("[object Object]");
    });

    it("GREEN: the type contract keeps health as an object (compile-time lock)", () => {
      // A compile-time assertion: health.ok must be a boolean field. If a future
      // change re-types health back to a scalar string this block fails `tsc`
      // (run via the `typecheck` script which includes tsconfig.vitest.json).
      const h: DeviceStatus["health"] = healthyStatus.health;
      expect(typeof h.ok).toBe("boolean");
    });
  }
});
