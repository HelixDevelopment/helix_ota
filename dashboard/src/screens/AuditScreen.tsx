// Helix OTA — Audit log viewer (operational_endpoints.md §4; admin-only).
// GET /audit supports ?action / ?resource_type filters + ?since / ?until RFC3339
// time bounds + ?limit/?cursor. Each row carries an actor object (subject +
// optional user_id), the action verb, resource type/id, IP, and created_at.

import { useState } from "react";
import { apiClient, ApiError } from "../api/client";
import { useApi } from "../query/useApi";
import { Badge, Button, Card, EmptyState, ErrorPanel, Field, Table, TextInput } from "../components/ui";
import type { AuditQuery } from "../types/api";

// Convert a datetime-local input value (YYYY-MM-DDTHH:mm) to RFC3339 (UTC Z).
// Empty input -> undefined (omit the bound).
function toRfc3339(local: string): string | undefined {
  if (!local.trim()) return undefined;
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toISOString();
}

export function AuditScreen() {
  // Form state (applied to the active query only on "Apply").
  const [action, setAction] = useState("");
  const [resourceType, setResourceType] = useState("");
  const [since, setSince] = useState("");
  const [until, setUntil] = useState("");

  // The query actually sent to the server (committed on Apply).
  const [query, setQuery] = useState<AuditQuery>({ limit: 50 });

  const { data, error, loading, refetch } = useApi(() => apiClient.listAudit(query), {
    deps: [JSON.stringify(query)],
  });

  // 403 means the session lacks the admin role (GET /audit is admin-only).
  const forbidden = error instanceof ApiError && error.status === 403;
  const auditMissing = error instanceof ApiError && error.status === 404;

  function apply(e: React.FormEvent) {
    e.preventDefault();
    setQuery({
      limit: 50,
      ...(action.trim() ? { action: action.trim() } : {}),
      ...(resourceType.trim() ? { resource_type: resourceType.trim() } : {}),
      ...(toRfc3339(since) ? { since: toRfc3339(since) } : {}),
      ...(toRfc3339(until) ? { until: toRfc3339(until) } : {}),
    });
  }

  function reset() {
    setAction("");
    setResourceType("");
    setSince("");
    setUntil("");
    setQuery({ limit: 50 });
  }

  return (
    <div>
      <h1>Audit log</h1>

      <Card title="Filter">
        <form onSubmit={apply}>
          <div style={{ display: "flex", gap: 12, flexWrap: "wrap" }}>
            <Field label="action (e.g. RELEASE_CREATE)">
              <TextInput value={action} onChange={setAction} placeholder="optional" />
            </Field>
            <Field label="resource_type (e.g. release)">
              <TextInput value={resourceType} onChange={setResourceType} placeholder="optional" />
            </Field>
            <Field label="since">
              <input
                style={dtInput}
                type="datetime-local"
                value={since}
                onChange={(e) => setSince(e.target.value)}
              />
            </Field>
            <Field label="until">
              <input
                style={dtInput}
                type="datetime-local"
                value={until}
                onChange={(e) => setUntil(e.target.value)}
              />
            </Field>
          </div>
          <div style={{ display: "flex", gap: 8, marginTop: 4 }}>
            <Button type="submit">Apply filter</Button>
            <Button type="button" variant="secondary" onClick={reset}>
              Reset
            </Button>
          </div>
        </form>
      </Card>

      <Card title="Entries">
        {forbidden ? (
          <EmptyState>The audit log requires the admin role.</EmptyState>
        ) : null}
        {auditMissing ? (
          <EmptyState>The audit log is not available (endpoint returned 404).</EmptyState>
        ) : null}
        {error && !forbidden && !auditMissing ? <ErrorPanel error={error} /> : null}
        {loading && !data ? <EmptyState>Loading…</EmptyState> : null}
        {data && data.items.length === 0 ? <EmptyState>No audit entries match.</EmptyState> : null}
        {data && data.items.length > 0 ? (
          <Table head={["at", "actor", "action", "resource", "ip"]}>
            {data.items.map((e) => (
              <tr key={e.id}>
                <td style={td}>{e.created_at}</td>
                <td style={td}>
                  {e.actor.subject}
                  {e.actor.user_id ? (
                    <span style={{ color: "#6b7280" }}> ({e.actor.user_id})</span>
                  ) : null}
                </td>
                <td style={td}>
                  <Badge tone="info">{e.action}</Badge>
                </td>
                <td style={td}>
                  {e.resource_type}
                  {e.resource_id ? (
                    <span style={{ fontFamily: "monospace", color: "#6b7280" }}>
                      {" "}
                      {e.resource_id}
                    </span>
                  ) : null}
                </td>
                <td style={td}>{e.ip_address || "—"}</td>
              </tr>
            ))}
          </Table>
        ) : null}
        {data?.next_cursor ? (
          <div style={{ marginTop: 8, color: "#6b7280", fontSize: 13 }}>
            More entries available (next_cursor: {data.next_cursor}).
          </div>
        ) : null}
        {data ? (
          <div style={{ marginTop: 12 }}>
            <Button variant="secondary" onClick={refetch}>
              Refresh
            </Button>
          </div>
        ) : null}
      </Card>
    </div>
  );
}

const td: React.CSSProperties = { padding: "8px 10px", borderBottom: "1px solid #eef1f5" };
const dtInput: React.CSSProperties = {
  width: "100%",
  boxSizing: "border-box",
  padding: "8px 10px",
  fontSize: 14,
  border: "1px solid #cbd2dc",
  borderRadius: 6,
};
