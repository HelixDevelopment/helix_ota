// Helix OTA — Deployments: list, create (all-targets), detail + rollout panel (design §9.3).
// MVP strategy is locked to "all-targets"; staged-rollout controls are G7/1.0.1 and the
// rollout routes are UNVERIFIED (design §13) — the panel degrades gracefully when absent.

import { useState } from "react";
import { Link, useLocation, useNavigate, useParams } from "react-router-dom";
import { apiClient, ApiError } from "../api/client";
import { useApi } from "../query/useApi";
import {
  Badge,
  Button,
  Card,
  EmptyState,
  ErrorPanel,
  Field,
  ProgressBar,
  TextInput,
} from "../components/ui";
import { RoleGate } from "../components/AppShell";
import type { DeploymentCreate, DeploymentProgress } from "../types/api";

// No GET /deployments list endpoint is defined in MVP (design §6): the list screen lets the
// operator look a deployment up by id rather than inventing a list route.
export function DeploymentList() {
  const navigate = useNavigate();
  const [id, setId] = useState("");
  return (
    <div>
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <h1>Deployments</h1>
        <RoleGate allow={["operator", "admin"]}>
          <Link to="/deployments/new">
            <Button>New deployment</Button>
          </Link>
        </RoleGate>
      </div>
      <Card title="Open a deployment">
        <p style={{ color: "#6b7280", marginTop: 0 }}>
          A deployments-list endpoint is a documented server-side follow-up (design §6); the
          dashboard does not invent it. Open a deployment by id:
        </p>
        <Field label="deployment_id">
          <TextInput value={id} onChange={setId} placeholder="dep_…" />
        </Field>
        <Button
          disabled={id.trim() === ""}
          onClick={() => navigate(`/deployments/${encodeURIComponent(id.trim())}`)}
        >
          Open
        </Button>
      </Card>
    </div>
  );
}

interface CreateState {
  release_id?: string;
}

export function DeploymentCreateScreen() {
  const location = useLocation();
  const navigate = useNavigate();
  const prefill = (location.state as CreateState | null) ?? {};

  const [form, setForm] = useState<DeploymentCreate>({
    release_id: prefill.release_id ?? "",
    strategy: "all-targets", // locked single value in MVP
    group: "",
  });
  const [error, setError] = useState<unknown>(null);
  const [submitting, setSubmitting] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      const payload: DeploymentCreate = {
        release_id: form.release_id,
        strategy: "all-targets",
        ...(form.group?.trim() ? { group: form.group.trim() } : {}),
      };
      const dep = await apiClient.createDeployment(payload);
      navigate(`/deployments/${encodeURIComponent(dep.deployment_id)}`);
    } catch (err) {
      setError(err);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <div>
      <h1>New deployment</h1>
      <Card>
        <form onSubmit={onSubmit}>
          <Field label="release_id">
            <TextInput
              value={form.release_id}
              onChange={(v) => setForm((f) => ({ ...f, release_id: v }))}
            />
          </Field>
          <Field label="strategy">
            {/* Locked to all-targets; staged rollout arrives in 1.0.1 (design §9.3). */}
            <select disabled value="all-targets" style={selectStyle}>
              <option value="all-targets">all-targets</option>
            </select>
          </Field>
          <p style={{ color: "#854d0e", fontSize: 13, marginTop: 0 }}>
            Staged / percentage rollout arrives in 1.0.1.
          </p>
          <Field label="group (optional)">
            <TextInput
              value={form.group ?? ""}
              onChange={(v) => setForm((f) => ({ ...f, group: v }))}
            />
          </Field>
          {error ? <ErrorPanel error={error} /> : null}
          <div style={{ marginTop: 12 }}>
            <Button type="submit" disabled={form.release_id.trim() === "" || submitting}>
              {submitting ? "Deploying…" : "Create deployment"}
            </Button>
          </div>
        </form>
      </Card>
    </div>
  );
}

function progressTotal(p: DeploymentProgress): number {
  return p.pending + p.downloading + p.installed + p.succeeded + p.failed;
}

export function DeploymentDetail() {
  const { deploymentId = "" } = useParams();
  // Auto-refresh on the query-cache poll interval (design §9.3).
  const { data, error, loading, refetch } = useApi(
    () => apiClient.getDeployment(deploymentId),
    { deps: [deploymentId], intervalMs: 5000 },
  );

  return (
    <div>
      <h1>Deployment</h1>
      <Card>
        {error ? <ErrorPanel error={error} /> : null}
        {loading && !data ? <EmptyState>Loading…</EmptyState> : null}
        {data ? (
          <>
            <div style={{ marginBottom: 8 }}>
              <strong>{data.deployment_id}</strong> · <Badge tone="info">{data.status}</Badge> ·{" "}
              {data.strategy}
            </div>
            <div style={{ marginBottom: 12 }}>
              <ProgressBar
                value={data.progress.succeeded}
                max={Math.max(1, progressTotal(data.progress))}
              />
            </div>
            <Counts progress={data.progress} />
            {/* No pause/resume/abort in MVP — action row reserved (design §9.3). */}
            <div style={{ marginTop: 12 }}>
              <Button variant="secondary" onClick={refetch}>
                Refresh now
              </Button>
            </div>
          </>
        ) : null}
      </Card>

      <RolloutPanel deploymentId={deploymentId} />
    </div>
  );
}

function Counts({ progress }: { progress: DeploymentProgress }) {
  const items: { label: string; tone: Parameters<typeof Badge>[0]["tone"]; n: number }[] = [
    { label: "pending", tone: "neutral", n: progress.pending },
    { label: "downloading", tone: "info", n: progress.downloading },
    { label: "installed", tone: "info", n: progress.installed },
    { label: "succeeded", tone: "ok", n: progress.succeeded },
    { label: "failed", tone: "err", n: progress.failed },
  ];
  return (
    <div style={{ display: "flex", gap: 16, flexWrap: "wrap" }}>
      {items.map((i) => (
        <div key={i.label}>
          <Badge tone={i.tone}>{i.label}</Badge> <strong>{i.n}</strong>
        </div>
      ))}
    </div>
  );
}

// Rollout panel — reads + commands the staged-rollout API (G7/1.0.1, UNVERIFIED routes).
// If the route is not yet served (404 NOT_FOUND), the panel shows a graceful "available in
// 1.0.1" empty state instead of an error (design §9.3 / §13 graceful degradation).
function RolloutPanel({ deploymentId }: { deploymentId: string }) {
  const { data, error, refetch } = useApi(() => apiClient.getRollout(deploymentId), {
    deps: [deploymentId],
  });
  const [pct, setPct] = useState("");
  const [busy, setBusy] = useState(false);
  const [cmdError, setCmdError] = useState<unknown>(null);

  const notYetAvailable = error instanceof ApiError && error.status === 404;

  async function send(paused?: boolean) {
    setBusy(true);
    setCmdError(null);
    try {
      await apiClient.postRollout(deploymentId, {
        ...(pct.trim() !== "" ? { percentage: Number(pct) } : {}),
        ...(paused !== undefined ? { paused } : {}),
      });
      refetch();
    } catch (err) {
      setCmdError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title="Rollout control (1.0.1 preview)">
      {notYetAvailable ? (
        <EmptyState>Staged rollout control is available when the 1.0.1 rollout API ships.</EmptyState>
      ) : null}
      {error && !notYetAvailable ? <ErrorPanel error={error} /> : null}
      {data ? (
        <>
          <div style={{ marginBottom: 8 }}>
            <Badge tone={data.paused ? "warn" : "ok"}>{data.paused ? "paused" : "active"}</Badge>{" "}
            · {data.percentage}% · {data.strategy}
          </div>
          <ProgressBar value={data.percentage} />
          <RoleGate allow={["operator", "admin"]}>
            <div style={{ display: "flex", gap: 8, alignItems: "flex-end", marginTop: 12 }}>
              <Field label="percentage">
                <TextInput type="number" value={pct} onChange={setPct} placeholder="0–100" />
              </Field>
              <Button disabled={busy} onClick={() => send()}>
                Set %
              </Button>
              <Button
                variant="secondary"
                disabled={busy}
                onClick={() => send(!data.paused)}
              >
                {data.paused ? "Resume" : "Pause"}
              </Button>
            </div>
          </RoleGate>
          {cmdError ? <ErrorPanel error={cmdError} /> : null}
        </>
      ) : null}
    </Card>
  );
}

const selectStyle: React.CSSProperties = {
  width: "100%",
  padding: "8px 10px",
  fontSize: 14,
  border: "1px solid #cbd2dc",
  borderRadius: 6,
  background: "#f3f4f6",
};
