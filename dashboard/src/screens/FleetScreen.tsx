// Helix OTA — Fleet health + telemetry (design §9.4).
// Per-device status via GET /devices/{id}/status. Telemetry read API is PARTIAL (gap G4):
// telemetry panels render a graceful empty state when the route is not yet served, rather
// than calling an undefined endpoint or surfacing it as an error (design §9.4 / §13).

import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { apiClient, ApiError } from "../api/client";
import { useApi } from "../query/useApi";
import {
  Badge,
  type BadgeTone,
  Button,
  Card,
  EmptyState,
  ErrorPanel,
  Field,
  Table,
  TextInput,
} from "../components/ui";
import type { UpdateState } from "../types/api";

function toneForState(s: UpdateState): BadgeTone {
  switch (s) {
    case "success":
      return "ok";
    case "failure":
      return "err";
    case "idle":
      return "neutral";
    default:
      return "info";
  }
}

export function FleetHealth() {
  // No device-list endpoint is defined in MVP; the operator opens a device by id, and the
  // fleet KPIs are sourced from the (PARTIAL) telemetry overview when available.
  const [deviceId, setDeviceId] = useState("");
  const overview = useApi(() => apiClient.getTelemetryOverview());
  const overviewMissing =
    overview.error instanceof ApiError && overview.error.status === 404;

  return (
    <div>
      <h1>Fleet</h1>

      <Card title="Open a device">
        <Field label="device_id">
          <TextInput value={deviceId} onChange={setDeviceId} placeholder="dev_…" />
        </Field>
        <Link
          to={deviceId.trim() ? `/fleet/${encodeURIComponent(deviceId.trim())}` : "#"}
          style={{ pointerEvents: deviceId.trim() ? "auto" : "none" }}
        >
          <Button disabled={deviceId.trim() === ""}>Open device</Button>
        </Link>
      </Card>

      <Card title="Fleet overview">
        {overviewMissing ? (
          <EmptyState>
            Fleet telemetry overview is available when the telemetry read API (G4) ships.
          </EmptyState>
        ) : null}
        {overview.error && !overviewMissing ? <ErrorPanel error={overview.error} /> : null}
        {overview.loading ? <EmptyState>Loading…</EmptyState> : null}
        {overview.data ? (
          <>
            <div style={{ marginBottom: 12 }}>
              <strong>{overview.data.total_devices}</strong> devices
            </div>
            <Table head={["update_state", "count"]}>
              {Object.entries(overview.data.by_update_state).map(([state, n]) => (
                <tr key={state}>
                  <td style={td}>
                    <Badge tone={toneForState(state as UpdateState)}>{state}</Badge>
                  </td>
                  <td style={td}>{n}</td>
                </tr>
              ))}
            </Table>
          </>
        ) : null}
      </Card>
    </div>
  );
}

export function DeviceDetail() {
  const { deviceId = "" } = useParams();
  const status = useApi(() => apiClient.getDeviceStatus(deviceId), {
    deps: [deviceId],
    intervalMs: 8000,
  });
  const telemetry = useApi(() => apiClient.getDeviceTelemetry(deviceId), {
    deps: [deviceId],
  });
  const telemetryMissing =
    telemetry.error instanceof ApiError && telemetry.error.status === 404;

  return (
    <div>
      <h1>Device {deviceId}</h1>

      <Card title="Status">
        {status.error ? <ErrorPanel error={status.error} /> : null}
        {status.loading && !status.data ? <EmptyState>Loading…</EmptyState> : null}
        {status.data ? (
          <dl>
            <Row label="current_version" value={status.data.current_version} />
            <Row label="target_version" value={status.data.target_version ?? "—"} />
            <Row label="update_state">
              <Badge tone={toneForState(status.data.update_state)}>
                {status.data.update_state}
              </Badge>
            </Row>
            <Row label="active_slot" value={status.data.active_slot ?? "—"} />
            <Row label="last_seen" value={status.data.last_seen} />
            <Row label="health" value={status.data.health ?? "—"} />
          </dl>
        ) : null}
      </Card>

      <Card title="Telemetry history">
        {telemetryMissing ? (
          <EmptyState>
            Per-device telemetry history is available when the telemetry read API (G4) ships.
          </EmptyState>
        ) : null}
        {telemetry.error && !telemetryMissing ? <ErrorPanel error={telemetry.error} /> : null}
        {telemetry.data && telemetry.data.items.length === 0 ? (
          <EmptyState>No telemetry events yet.</EmptyState>
        ) : null}
        {telemetry.data && telemetry.data.items.length > 0 ? (
          <Table head={["event_type", "at", "detail"]}>
            {telemetry.data.items.map((ev, i) => (
              <tr key={i}>
                <td style={td}>
                  <Badge tone={toneForState(ev.event_type)}>{ev.event_type}</Badge>
                </td>
                <td style={td}>{ev.at}</td>
                <td style={td}>{ev.detail ?? "—"}</td>
              </tr>
            ))}
          </Table>
        ) : null}
      </Card>
    </div>
  );
}

function Row({ label, value, children }: { label: string; value?: string; children?: React.ReactNode }) {
  return (
    <div style={{ display: "flex", gap: 8, padding: "4px 0" }}>
      <dt style={{ width: 160, color: "#6b7280" }}>{label}</dt>
      <dd style={{ margin: 0, fontFamily: "monospace" }}>{children ?? value}</dd>
    </div>
  );
}

const td: React.CSSProperties = { padding: "8px 10px", borderBottom: "1px solid #eef1f5" };
