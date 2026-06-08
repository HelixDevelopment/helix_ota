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
  Table,
  TextInput,
} from "../components/ui";
import { RoleGate } from "../components/AppShell";
import type {
  DeploymentCreate,
  DeploymentProgress,
  RolloutPhaseSpec,
} from "../types/api";

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
      <RecallPanel deploymentId={deploymentId} />
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

function statusTone(status: string): Parameters<typeof Badge>[0]["tone"] {
  switch (status) {
    case "completed":
      return "ok";
    case "halted":
    case "rolled_back":
      return "err";
    case "running":
      return "info";
    default:
      return "neutral";
  }
}

// Rollout panel — reads + drives the staged-rollout API (1.0.1, phase-based).
//  - GET state degrades to a "no rollout yet" empty state on 404.
//  - Create starts a default phased plan (10% → 50% → 100%).
//  - Evaluate applies a health verdict to the current phase and shows the
//    engine decision (advance / hold / halt / complete).
function RolloutPanel({ deploymentId }: { deploymentId: string }) {
  const { data, error, refetch } = useApi(() => apiClient.getRollout(deploymentId), {
    deps: [deploymentId],
  });
  const noRollout = error instanceof ApiError && error.status === 404;

  const [busy, setBusy] = useState(false);
  const [cmdError, setCmdError] = useState<unknown>(null);
  const [decision, setDecision] = useState<{ action: string; reason: string } | null>(null);

  // Evaluate-verdict form (health summary for the current phase cohort).
  const [successRate, setSuccessRate] = useState("0.95");
  const [errorRate, setErrorRate] = useState("0.02");
  const [healthFailed, setHealthFailed] = useState(false);

  async function create() {
    setBusy(true);
    setCmdError(null);
    setDecision(null);
    try {
      const phases: RolloutPhaseSpec[] = [
        { percentage: 10, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 3600, auto_progress: false },
        { percentage: 50, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 3600, auto_progress: false },
        { percentage: 100, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 0, auto_progress: false },
      ];
      await apiClient.createRollout(deploymentId, { phases });
      refetch();
    } catch (err) {
      setCmdError(err);
    } finally {
      setBusy(false);
    }
  }

  async function evaluate() {
    setBusy(true);
    setCmdError(null);
    setDecision(null);
    try {
      const dec = await apiClient.evaluateRollout(deploymentId, {
        success_rate: Number(successRate),
        error_rate: Number(errorRate),
        post_boot_health_failed: healthFailed,
      });
      setDecision({ action: dec.action, reason: dec.reason });
      refetch();
    } catch (err) {
      setCmdError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title="Staged rollout">
      {noRollout ? (
        <>
          <EmptyState>No staged rollout for this deployment yet.</EmptyState>
          <RoleGate allow={["operator", "admin"]}>
            <Button disabled={busy} onClick={create}>
              {busy ? "Starting…" : "Start staged rollout (10% → 50% → 100%)"}
            </Button>
          </RoleGate>
        </>
      ) : null}
      {error && !noRollout ? <ErrorPanel error={error} /> : null}
      {data ? (
        <>
          <div style={{ marginBottom: 8 }}>
            <Badge tone={statusTone(data.status)}>{data.status}</Badge> · phase{" "}
            {data.current_phase + 1} of {data.phases.length}
          </div>
          {data.phases[data.current_phase] ? (
            <div style={{ marginBottom: 12 }}>
              <ProgressBar value={data.phases[data.current_phase].percentage} />
            </div>
          ) : null}
          <Table head={["#", "percentage", "success_threshold", "error_threshold", "auto_progress"]}>
            {data.phases.map((p, i) => (
              <tr key={i} style={i === data.current_phase ? { background: "#eff6ff" } : undefined}>
                <td style={td}>{i === data.current_phase ? `▶ ${i + 1}` : i + 1}</td>
                <td style={td}>{p.percentage}%</td>
                <td style={td}>{p.success_threshold}</td>
                <td style={td}>{p.error_threshold}</td>
                <td style={td}>{p.auto_progress ? "yes" : "no"}</td>
              </tr>
            ))}
          </Table>
          <RoleGate allow={["operator", "admin"]}>
            <div style={{ marginTop: 16, borderTop: "1px solid #eef1f5", paddingTop: 12 }}>
              <strong style={{ fontSize: 13 }}>Evaluate current phase</strong>
              <div style={{ display: "flex", gap: 8, alignItems: "flex-end", flexWrap: "wrap", marginTop: 8 }}>
                <Field label="success_rate (0–1)">
                  <TextInput type="number" value={successRate} onChange={setSuccessRate} />
                </Field>
                <Field label="error_rate (0–1)">
                  <TextInput type="number" value={errorRate} onChange={setErrorRate} />
                </Field>
                <label style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 12 }}>
                  <input
                    type="checkbox"
                    checked={healthFailed}
                    onChange={(e) => setHealthFailed(e.target.checked)}
                  />
                  <span style={{ fontSize: 13 }}>post_boot_health_failed</span>
                </label>
                <Button disabled={busy} onClick={evaluate}>
                  {busy ? "Evaluating…" : "Evaluate"}
                </Button>
              </div>
            </div>
          </RoleGate>
          {decision ? (
            <div style={{ marginTop: 8 }}>
              decision: <Badge tone="info">{decision.action}</Badge>{" "}
              <span style={{ color: "#6b7280" }}>{decision.reason}</span>
            </div>
          ) : null}
          {cmdError ? <ErrorPanel error={cmdError} /> : null}
        </>
      ) : null}
      {noRollout && cmdError ? <ErrorPanel error={cmdError} /> : null}
    </Card>
  );
}

// Recall panel — server-driven rollback to a previous-good release + the
// deployment's rollback/abort history.
function RecallPanel({ deploymentId }: { deploymentId: string }) {
  const history = useApi(() => apiClient.listRollbacks(deploymentId), {
    deps: [deploymentId],
  });
  const historyMissing =
    history.error instanceof ApiError && history.error.status === 404;

  const [toRelease, setToRelease] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [cmdError, setCmdError] = useState<unknown>(null);
  const [ok, setOk] = useState<string | null>(null);

  async function recall() {
    setBusy(true);
    setCmdError(null);
    setOk(null);
    try {
      const rec = await apiClient.recallDeployment(deploymentId, {
        to_release_id: toRelease.trim(),
        ...(reason.trim() ? { reason: reason.trim() } : {}),
      });
      setOk(`Recall recorded → new deployment ${rec.recall_deployment_id ?? "(n/a)"}`);
      setToRelease("");
      setReason("");
      history.refetch();
    } catch (err) {
      setCmdError(err);
    } finally {
      setBusy(false);
    }
  }

  return (
    <Card title="Recall (roll back to a previous-good release)">
      <RoleGate allow={["operator", "admin"]}>
        <div style={{ display: "flex", gap: 8, alignItems: "flex-end", flexWrap: "wrap" }}>
          <Field label="to_release_id">
            <TextInput value={toRelease} onChange={setToRelease} placeholder="rel_…" />
          </Field>
          <Field label="reason (optional)">
            <TextInput value={reason} onChange={setReason} placeholder="e.g. boot regression" />
          </Field>
          <Button variant="secondary" disabled={busy || toRelease.trim() === ""} onClick={recall}>
            {busy ? "Recalling…" : "Recall"}
          </Button>
        </div>
      </RoleGate>
      {ok ? <div style={{ marginTop: 8, color: "#166534", fontSize: 13 }}>{ok}</div> : null}
      {cmdError ? <ErrorPanel error={cmdError} /> : null}

      <h3 style={{ fontSize: 14, color: "#374151", margin: "16px 0 8px" }}>Rollback history</h3>
      {historyMissing ? (
        <EmptyState>Rollback history is not available (endpoint returned 404).</EmptyState>
      ) : null}
      {history.error && !historyMissing ? <ErrorPanel error={history.error} /> : null}
      {history.loading && !history.data ? <EmptyState>Loading…</EmptyState> : null}
      {history.data && history.data.items.length === 0 ? (
        <EmptyState>No rollbacks recorded for this deployment.</EmptyState>
      ) : null}
      {history.data && history.data.items.length > 0 ? (
        <Table head={["kind", "from → to", "reason", "by", "at"]}>
          {history.data.items.map((r) => (
            <tr key={r.id}>
              <td style={td}>
                <Badge tone={r.kind === "rollback" ? "warn" : "neutral"}>{r.kind}</Badge>
              </td>
              <td style={td}>
                {(r.from_release_id ?? "—") + " → " + (r.to_release_id ?? "—")}
              </td>
              <td style={td}>{r.reason || "—"}</td>
              <td style={td}>{r.triggered_by || "—"}</td>
              <td style={td}>{r.created_at}</td>
            </tr>
          ))}
        </Table>
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

const td: React.CSSProperties = { padding: "8px 10px", borderBottom: "1px solid #eef1f5" };
