// Helix OTA — Device groups: list + create + members (operational_endpoints.md §6).
//  - GET /groups -> { items: [{ group_id, name, description?, member_count }] }
//  - POST /groups { name, description? }
//  - GET /groups/{id}/members -> { group_id, items: [{ device_id, added_at }] }
//  - POST /groups/{id}/members { device_ids } -> { added, already_member, not_found }

import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { apiClient, ApiError } from "../api/client";
import { useApi } from "../query/useApi";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorPanel,
  Field,
  Table,
  TextInput,
} from "../components/ui";
import { RoleGate } from "../components/AppShell";
import type { DeviceGroupMembersAddResult } from "../types/api";

export function GroupList() {
  const { data, error, loading, refetch } = useApi(() => apiClient.listGroups());
  const groupsMissing = error instanceof ApiError && error.status === 404;

  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>Device groups</h1>
        <RoleGate allow={["operator", "admin"]}>
          <Link to="/groups/new">
            <Button>New group</Button>
          </Link>
        </RoleGate>
      </div>
      <Card>
        {groupsMissing ? (
          <EmptyState>Device groups are not available (endpoint returned 404).</EmptyState>
        ) : null}
        {error && !groupsMissing ? <ErrorPanel error={error} /> : null}
        {loading && !data ? <EmptyState>Loading…</EmptyState> : null}
        {data && data.items.length === 0 ? <EmptyState>No groups yet.</EmptyState> : null}
        {data && data.items.length > 0 ? (
          <Table head={["name", "description", "members", "created_at", ""]}>
            {data.items.map((g) => (
              <tr key={g.group_id}>
                <td style={td}>{g.name}</td>
                <td style={td}>{g.description || "—"}</td>
                <td style={td}>
                  <Badge tone="info">{g.member_count}</Badge>
                </td>
                <td style={td}>{g.created_at}</td>
                <td style={td}>
                  <Link to={`/groups/${encodeURIComponent(g.group_id)}`}>members</Link>
                </td>
              </tr>
            ))}
          </Table>
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

export function GroupCreateScreen() {
  const navigate = useNavigate();
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [error, setError] = useState<unknown>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const g = await apiClient.createGroup({
        name: name.trim(),
        ...(description.trim() ? { description: description.trim() } : {}),
      });
      navigate(`/groups/${encodeURIComponent(g.group_id)}`);
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <h1>New device group</h1>
      <Card>
        <form onSubmit={onSubmit}>
          <Field label="name">
            <TextInput value={name} onChange={setName} placeholder="e.g. canary-fleet" />
          </Field>
          <Field label="description (optional)">
            <TextInput value={description} onChange={setDescription} />
          </Field>
          {/* 409 (name conflict) surfaces via ErrorPanel. */}
          {error ? <ErrorPanel error={error} /> : null}
          <div style={{ marginTop: 12 }}>
            <Button type="submit" disabled={name.trim() === "" || submitting}>
              {submitting ? "Creating…" : "Create group"}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}

export function GroupDetail() {
  const { groupId = "" } = useParams();
  const members = useApi(() => apiClient.getGroupMembers(groupId), { deps: [groupId] });
  const membersMissing = members.error instanceof ApiError && members.error.status === 404;

  // Batch add-members form. `device_ids` are entered one-per-line / comma-separated.
  const [raw, setRaw] = useState("");
  const [busy, setBusy] = useState(false);
  const [addError, setAddError] = useState<unknown>(null);
  const [result, setResult] = useState<DeviceGroupMembersAddResult | null>(null);

  function parseIds(s: string): string[] {
    return s
      .split(/[\s,]+/)
      .map((x) => x.trim())
      .filter((x) => x !== "");
  }

  async function add() {
    const ids = parseIds(raw);
    if (ids.length === 0) return;
    setBusy(true);
    setAddError(null);
    setResult(null);
    try {
      const res = await apiClient.addGroupMembers(groupId, ids);
      setResult(res);
      setRaw("");
      members.refetch();
    } catch (err) {
      setAddError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <div>
      <h1>Group {groupId}</h1>

      <RoleGate allow={["operator", "admin"]}>
        <Card title="Add devices (batch)">
          <Field label="device_ids (one per line or comma-separated)">
            <textarea
              style={textarea}
              value={raw}
              onChange={(e) => setRaw(e.target.value)}
              placeholder={"dev_001\ndev_002\ndev_003"}
              rows={4}
            />
          </Field>
          <Button disabled={busy || parseIds(raw).length === 0} onClick={add}>
            {busy ? "Adding…" : `Add ${parseIds(raw).length || ""} device(s)`}
          </Button>
          {addError ? <ErrorPanel error={addError} /> : null}
          {result ? (
            <div style={{ marginTop: 12, display: "flex", gap: 16, flexWrap: "wrap" }}>
              <Disposition label="added" tone="ok" ids={result.added} />
              <Disposition label="already_member" tone="warn" ids={result.already_member} />
              <Disposition label="not_found" tone="err" ids={result.not_found} />
            </div>
          ) : null}
        </Card>
      </RoleGate>

      <Card title="Members">
        {membersMissing ? (
          <EmptyState>This group was not found (endpoint returned 404).</EmptyState>
        ) : null}
        {members.error && !membersMissing ? <ErrorPanel error={members.error} /> : null}
        {members.loading && !members.data ? <EmptyState>Loading…</EmptyState> : null}
        {members.data && members.data.items.length === 0 ? (
          <EmptyState>No members in this group yet.</EmptyState>
        ) : null}
        {members.data && members.data.items.length > 0 ? (
          <Table head={["device_id", "added_at", ""]}>
            {members.data.items.map((m) => (
              <tr key={m.device_id}>
                <td style={td}>{m.device_id}</td>
                <td style={td}>{m.added_at}</td>
                <td style={td}>
                  <Link to={`/fleet/${encodeURIComponent(m.device_id)}`}>device</Link>
                </td>
              </tr>
            ))}
          </Table>
        ) : null}
      </Card>
    </div>
  );
}

function Disposition({
  label,
  tone,
  ids,
}: {
  label: string;
  tone: Parameters<typeof Badge>[0]["tone"];
  ids: string[];
}) {
  return (
    <div>
      <Badge tone={tone}>
        {label}: {ids.length}
      </Badge>
      {ids.length > 0 ? (
        <ul style={{ margin: "6px 0 0", paddingLeft: 18, fontFamily: "monospace", fontSize: 12 }}>
          {ids.map((id) => (
            <li key={id}>{id}</li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

const td: React.CSSProperties = { padding: "8px 10px", borderBottom: "1px solid #eef1f5" };
const textarea: React.CSSProperties = {
  width: "100%",
  boxSizing: "border-box",
  padding: "8px 10px",
  fontSize: 14,
  fontFamily: "monospace",
  border: "1px solid #cbd2dc",
  borderRadius: 6,
  resize: "vertical",
};
