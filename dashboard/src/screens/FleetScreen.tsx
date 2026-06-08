// Helix OTA — Fleet health + telemetry (design §9.4).
// Per-device status via GET /devices/{id}/status. Fleet KPIs come from
// GET /telemetry/overview (failure_rate + by_state + event_counts). Per-device
// history from GET /devices/{id}/telemetry (newest-first, cursor-paginated).
// Endpoints degrade to a graceful empty state on 404 rather than erroring.

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
  ProgressBar,
  Table,
  TextInput,
} from "../components/ui";

function toneForState(s: string): BadgeTone {
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

// failure_rate is a ratio in [0,1]; render it as a percentage with a tone.
function failureTone(rate: number): BadgeTone {
  if (rate <= 0) return "ok";
  if (rate < 0.1) return "warn";
  return "err";
}

export function FleetHealth() {
  // No device-list endpoint is defined in MVP; the operator opens a device by id,
  // and the fleet KPIs are sourced from the telemetry overview.
  const [deviceId, setDeviceId] = useState("");
  const overview = useApi(() => apiClient.getTelemetryOverview(), { intervalMs: 15000 });
  const overviewMissing =
    overview.error instanceof ApiError && overview.error.status === 404;

  const byState = overview.data?.by_state ?? {};
  const eventCounts = overview.data?.event_counts ?? {};

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
          <EmptyState>Fleet telemetry overview is not available (endpoint returned 404).</EmptyState>
        ) : null}
        {overview.error && !overviewMissing ? <ErrorPanel error={overview.error} /> : null}
        {overview.loading && !overview.data ? <EmptyState>Loading…</EmptyState> : null}
        {overview.data ? (
          <>
            <div style={{ display: "flex", gap: 24, alignItems: "center", marginBottom: 16, flexWrap: "wrap" }}>
              <div>
                <div style={kpiLabel}>devices</div>
                <div style={kpiValue}>{Object.values(byState).reduce((a, b) => a + b, 0)}</div>
              </div>
              <div>
                <div style={kpiLabel}>telemetry events</div>
                <div style={kpiValue}>{overview.data.total}</div>
              </div>
              <div>
                <div style={kpiLabel}>update failure rate</div>
                <div style={kpiValue}>
                  <Badge tone={failureTone(overview.data.failure_rate)}>
                    {(overview.data.failure_rate * 100).toFixed(1)}%
                  </Badge>
                </div>
                <div style={{ width: 180, marginTop: 6 }}>
                  <ProgressBar value={overview.data.failure_rate * 100} />
                </div>
              </div>
            </div>

            <h3 style={subhead}>Devices by update state</h3>
            {Object.keys(byState).length === 0 ? (
              <EmptyState>No device states reported yet.</EmptyState>
            ) : (
              <Table head={["update_state", "device count"]}>
                {Object.entries(byState).map(([state, n]) => (
                  <tr key={state}>
                    <td style={td}>
                      <Badge tone={toneForState(state)}>{state}</Badge>
                    </td>
                    <td style={td}>{n}</td>
                  </tr>
                ))}
              </Table>
            )}

            <h3 style={subhead}>Telemetry events by type</h3>
            {Object.keys(eventCounts).length === 0 ? (
              <EmptyState>No telemetry events yet.</EmptyState>
            ) : (
              <Table head={["event_type", "count"]}>
                {Object.entries(eventCounts).map(([ev, n]) => (
                  <tr key={ev}>
                    <td style={td}>
                      <Badge tone={toneForState(ev)}>{ev}</Badge>
                    </td>
                    <td style={td}>{n}</td>
                  </tr>
                ))}
              </Table>
            )}
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
          <EmptyState>Per-device telemetry history is not available (endpoint returned 404).</EmptyState>
        ) : null}
        {telemetry.error && !telemetryMissing ? <ErrorPanel error={telemetry.error} /> : null}
        {telemetry.loading && !telemetry.data ? <EmptyState>Loading…</EmptyState> : null}
        {telemetry.data && telemetry.data.items.length === 0 ? (
          <EmptyState>No telemetry events yet.</EmptyState>
        ) : null}
        {telemetry.data && telemetry.data.items.length > 0 ? (
          <Table head={["event", "version", "timestamp", "detail"]}>
            {telemetry.data.items.map((ev, i) => (
              <tr key={i}>
                <td style={td}>
                  <Badge tone={toneForState(ev.event)}>{ev.event}</Badge>
                </td>
                <td style={td}>{ev.version ?? "—"}</td>
                <td style={td}>{ev.timestamp}</td>
                <td style={td}>{ev.detail || ev.error_code || "—"}</td>
              </tr>
            ))}
          </Table>
        ) : null}
        {telemetry.data?.next_cursor ? (
          <div style={{ marginTop: 8, color: "#6b7280", fontSize: 13 }}>
            More events available (next_cursor: {telemetry.data.next_cursor}).
          </div>
        ) : null}
      </Card>
    </div>
  );
}

function Row({
  label,
  value,
  children,
}: {
  label: string;
  value?: string;
  children?: React.ReactNode;
}) {
  return (
    <div style={{ display: "flex", gap: 8, padding: "4px 0" }}>
      <dt style={{ width: 160, color: "#6b7280" }}>{label}</dt>
      <dd style={{ margin: 0, fontFamily: "monospace" }}>{children ?? value}</dd>
    </div>
  );
}

const td: React.CSSProperties = { padding: "8px 10px", borderBottom: "1px solid #eef1f5" };
const subhead: React.CSSProperties = { fontSize: 14, color: "#374151", margin: "18px 0 8px" };
const kpiLabel: React.CSSProperties = { fontSize: 12, color: "#6b7280", textTransform: "uppercase" };
const kpiValue: React.CSSProperties = { fontSize: 24, fontWeight: 700 };
