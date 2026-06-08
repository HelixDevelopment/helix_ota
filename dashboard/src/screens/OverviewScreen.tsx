// Helix OTA — DashboardOverview (design §6, route "/").
// Recent releases + best-effort server-health badge. The /healthz route is not a defined
// MVP JSON endpoint (design §8) — the badge is best-effort and degrades silently.

import { Link } from "react-router-dom";
import { apiClient } from "../api/client";
import { useApi } from "../query/useApi";
import { Badge, Card, EmptyState, ErrorPanel, Table } from "../components/ui";

export function DashboardOverview() {
  const releases = useApi(() => apiClient.listReleases({ limit: 5 }));
  const health = useApi(() => apiClient.health());

  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>Overview</h1>
        {/* Best-effort health badge — silent if the route is not served. */}
        {health.data ? <Badge tone="ok">server: {health.data.status}</Badge> : null}
      </div>

      <Card title="Recent releases">
        {releases.error ? <ErrorPanel error={releases.error} /> : null}
        {releases.loading ? <EmptyState>Loading…</EmptyState> : null}
        {releases.data && releases.data.items.length === 0 ? (
          <EmptyState>No releases yet.</EmptyState>
        ) : null}
        {releases.data && releases.data.items.length > 0 ? (
          <Table head={["version", "os", "target_model", "status", ""]}>
            {releases.data.items.map((r) => (
              <tr key={r.release_id}>
                <td style={td}>{r.version}</td>
                <td style={td}>{r.os}</td>
                <td style={td}>{r.target_model}</td>
                <td style={td}>
                  <Badge tone="info">{r.status}</Badge>
                </td>
                <td style={td}>
                  <Link to={`/releases/${encodeURIComponent(r.release_id)}`}>view</Link>
                </td>
              </tr>
            ))}
          </Table>
        ) : null}
      </Card>
    </div>
  );
}

const td: React.CSSProperties = { padding: "8px 10px", borderBottom: "1px solid #eef1f5" };
