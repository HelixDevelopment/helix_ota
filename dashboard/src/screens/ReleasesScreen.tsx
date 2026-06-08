// Helix OTA — Releases: list, create, detail (design §9.3).

import { useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { apiClient } from "../api/client";
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
import type { ReleaseCreate } from "../types/api";

export function ReleaseList() {
  const { data, error, loading } = useApi(() => apiClient.listReleases());
  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>Releases</h1>
        <RoleGate allow={["operator", "admin"]}>
          <Link to="/releases/new">
            <Button>New release</Button>
          </Link>
        </RoleGate>
      </div>
      <Card>
        {error ? <ErrorPanel error={error} /> : null}
        {loading ? <EmptyState>Loading…</EmptyState> : null}
        {data && data.items.length === 0 ? <EmptyState>No releases yet.</EmptyState> : null}
        {data && data.items.length > 0 ? (
          <Table head={["version", "os", "target_model", "status", "created_at", ""]}>
            {data.items.map((r) => (
              <tr key={r.release_id}>
                <td style={td}>{r.version}</td>
                <td style={td}>{r.os}</td>
                <td style={td}>{r.target_model}</td>
                <td style={td}>
                  <Badge tone="info">{r.status}</Badge>
                </td>
                <td style={td}>{r.created_at}</td>
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

interface CreateState {
  artifact_id?: string;
  version?: string;
  os?: string;
  target_model?: string;
}

export function ReleaseCreateScreen() {
  const location = useLocation();
  const navigate = useNavigate();
  const prefill = (location.state as CreateState | null) ?? {};

  const [form, setForm] = useState<ReleaseCreate>({
    artifact_id: prefill.artifact_id ?? "",
    version: prefill.version ?? "",
    os: prefill.os ?? "android",
    target_model: prefill.target_model ?? "",
    notes: "",
    min_current_version: "",
  });
  const [error, setError] = useState<unknown>(null);
  const [submitting, setSubmitting] = useState(false);

  function set<K extends keyof ReleaseCreate>(key: K, value: ReleaseCreate[K]) {
    setForm((f) => ({ ...f, [key]: value }));
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const release = await apiClient.createRelease(form);
      navigate(`/releases/${encodeURIComponent(release.release_id)}`);
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  const canSubmit =
    form.artifact_id.trim() !== "" &&
    form.version.trim() !== "" &&
    form.os.trim() !== "" &&
    form.target_model.trim() !== "" &&
    !submitting;

  return (
    <div>
      <h1>New release</h1>
      <Card>
        <form onSubmit={onSubmit}>
          <Field label="artifact_id">
            <TextInput value={form.artifact_id} onChange={(v) => set("artifact_id", v)} />
          </Field>
          <Field label="version">
            <TextInput value={form.version} onChange={(v) => set("version", v)} />
          </Field>
          <Field label="os">
            <TextInput value={form.os} onChange={(v) => set("os", v)} />
          </Field>
          <Field label="target_model">
            <TextInput value={form.target_model} onChange={(v) => set("target_model", v)} />
          </Field>
          <Field label="min_current_version (optional)">
            <TextInput
              value={form.min_current_version ?? ""}
              onChange={(v) => set("min_current_version", v)}
            />
          </Field>
          <Field label="notes (optional)">
            <TextInput value={form.notes ?? ""} onChange={(v) => set("notes", v)} />
          </Field>
          {/* 409 VERSION_NOT_MONOTONIC / 404 (artifact) surface via ErrorPanel (design §9.3). */}
          {error ? <ErrorPanel error={error} /> : null}
          <div style={{ marginTop: 12 }}>
            <Button type="submit" disabled={!canSubmit}>
              {submitting ? "Publishing…" : "Publish release"}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}

export function ReleaseDetail() {
  const { releaseId = "" } = useParams();
  const navigate = useNavigate();
  const { data, error, loading } = useApi(() => apiClient.getRelease(releaseId), {
    deps: [releaseId],
  });

  return (
    <div>
      <h1>Release</h1>
      <Card>
        {error ? <ErrorPanel error={error} /> : null}
        {loading ? <EmptyState>Loading…</EmptyState> : null}
        {data ? (
          <>
            <dl>
              <Row label="release_id" value={data.release_id} />
              <Row label="version" value={data.version} />
              <Row label="os" value={data.os} />
              <Row label="target_model" value={data.target_model} />
              <Row label="status" value={data.status} />
              <Row label="artifact_id" value={data.artifact_id} />
              <Row label="created_at" value={data.created_at} />
            </dl>
            <RoleGate allow={["operator", "admin"]}>
              <Button
                onClick={() =>
                  navigate("/deployments/new", { state: { release_id: data.release_id } })
                }
              >
                Deploy this release
              </Button>
            </RoleGate>
          </>
        ) : null}
      </Card>
    </div>
  );
}

function Row({ label, value }: { label: string; value: string }) {
  return (
    <div style={{ display: "flex", gap: 8, padding: "4px 0" }}>
      <dt style={{ width: 160, color: "#6b7280" }}>{label}</dt>
      <dd style={{ margin: 0, fontFamily: "monospace" }}>{value}</dd>
    </div>
  );
}

const td: React.CSSProperties = { padding: "8px 10px", borderBottom: "1px solid #eef1f5" };
