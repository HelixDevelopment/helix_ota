// Helix OTA dashboard — component tests for the DeploymentsScreen family:
// DeploymentList (lookup-by-id → navigate), DeploymentCreateScreen (prefill +
// submit → navigate + error panel), DeploymentDetail (progress counts +
// ProgressBar), the RolloutPanel (404 "no rollout" → start; populated phases +
// evaluate → engine decision; current-phase highlight) and the RecallPanel
// (recall success message, rollback-history table, 404 history). Stubs only the
// apiClient methods + useAuth for RoleGate (unit/component-test mocks, §11.4.27).
// Every assertion exercises real user-visible behaviour.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { ApiError } from "../api/client";
import type {
  DeploymentStatus,
  Deployment,
  RolloutState,
  RolloutDecision,
  RollbackList,
  RollbackRecord,
} from "../types/api";
import type { Role } from "../types/api";

const createDeployment = vi.fn<[unknown], Promise<Deployment>>();
const getDeployment = vi.fn<[string], Promise<DeploymentStatus>>();
const getRollout = vi.fn<[string], Promise<RolloutState>>();
const createRollout = vi.fn<[string, unknown], Promise<RolloutState>>();
const evaluateRollout = vi.fn<[string, unknown], Promise<RolloutDecision>>();
const recallDeployment = vi.fn<[string, unknown], Promise<RollbackRecord>>();
const listRollbacks = vi.fn<[string], Promise<RollbackList>>();

vi.mock("../api/client", async () => {
  const actual = await vi.importActual<typeof import("../api/client")>("../api/client");
  return {
    ...actual,
    apiClient: {
      createDeployment: (...a: unknown[]) => createDeployment(...(a as [unknown])),
      getDeployment: (...a: unknown[]) => getDeployment(...(a as [string])),
      getRollout: (...a: unknown[]) => getRollout(...(a as [string])),
      createRollout: (...a: unknown[]) => createRollout(...(a as [string, unknown])),
      evaluateRollout: (...a: unknown[]) => evaluateRollout(...(a as [string, unknown])),
      recallDeployment: (...a: unknown[]) => recallDeployment(...(a as [string, unknown])),
      listRollbacks: (...a: unknown[]) => listRollbacks(...(a as [string])),
    },
  };
});

let mockRoles: Role[] = ["operator"];
vi.mock("../auth/AuthContext", () => ({
  useAuth: () => ({ roles: mockRoles }),
}));

// Import AFTER mocks are registered.
import {
  DeploymentList,
  DeploymentCreateScreen,
  DeploymentDetail,
} from "./DeploymentsScreen";

beforeEach(() => {
  createDeployment.mockReset();
  getDeployment.mockReset();
  getRollout.mockReset();
  createRollout.mockReset();
  evaluateRollout.mockReset();
  recallDeployment.mockReset();
  listRollbacks.mockReset();
  mockRoles = ["operator"];
  // Default the auxiliary panels to "no data" so the detail tests stay focused.
  getRollout.mockRejectedValue(new ApiError(404, "NOT_FOUND", "no rollout", "r"));
  listRollbacks.mockResolvedValue({ items: [] });
});

describe("DeploymentList", () => {
  function renderList() {
    return render(
      <MemoryRouter initialEntries={["/deployments"]}>
        <Routes>
          <Route path="/deployments" element={<DeploymentList />} />
          <Route path="/deployments/:deploymentId" element={<div>detail stub</div>} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it("keeps Open disabled until an id is typed, then navigates to that deployment", async () => {
    renderList();
    const open = screen.getByRole("button", { name: "Open" });
    expect(open).toBeDisabled();

    fireEvent.change(screen.getByPlaceholderText("dep_…"), { target: { value: "dep_77" } });
    await waitFor(() => expect(open).toBeEnabled());
    fireEvent.click(open);
    await waitFor(() => expect(screen.getByText("detail stub")).toBeInTheDocument());
  });

  it("hides the 'New deployment' action for a viewer (RoleGate)", () => {
    mockRoles = ["viewer"];
    renderList();
    expect(screen.queryByRole("link", { name: "New deployment" })).not.toBeInTheDocument();
  });
});

describe("DeploymentCreateScreen", () => {
  function renderCreate(prefillReleaseId?: string) {
    const entry = prefillReleaseId
      ? { pathname: "/deployments/new", state: { release_id: prefillReleaseId } }
      : "/deployments/new";
    return render(
      <MemoryRouter initialEntries={[entry]}>
        <Routes>
          <Route path="/deployments/new" element={<DeploymentCreateScreen />} />
          <Route path="/deployments/:deploymentId" element={<div>detail stub</div>} />
        </Routes>
      </MemoryRouter>,
    );
  }

  it("prefills release_id from router state and submits all-targets → navigates", async () => {
    createDeployment.mockResolvedValue({
      deployment_id: "dep_new",
      release_id: "rel_5",
      strategy: "all-targets",
      created_at: "2026-06-10T00:00:00Z",
    });
    renderCreate("rel_5");

    // The prefilled release_id is in the field; submit is enabled.
    const relInput = screen.getByText("release_id").parentElement!.querySelector("input")!;
    expect(relInput).toHaveValue("rel_5");

    fireEvent.click(screen.getByRole("button", { name: "Create deployment" }));
    await waitFor(() => expect(screen.getByText("detail stub")).toBeInTheDocument());
    expect(createDeployment).toHaveBeenCalledTimes(1);
    expect(createDeployment.mock.calls[0][0]).toEqual({
      release_id: "rel_5",
      strategy: "all-targets",
    });
  });

  it("keeps submit disabled when release_id is empty", () => {
    renderCreate();
    expect(screen.getByRole("button", { name: "Create deployment" })).toBeDisabled();
  });

  it("includes the optional group when provided", async () => {
    createDeployment.mockResolvedValue({
      deployment_id: "dep_g",
      release_id: "rel_9",
      strategy: "all-targets",
      created_at: "2026-06-10T00:00:00Z",
    });
    renderCreate();
    fireEvent.change(screen.getByText("release_id").parentElement!.querySelector("input")!, {
      target: { value: "rel_9" },
    });
    fireEvent.change(
      screen.getByText("group (optional)").parentElement!.querySelector("input")!,
      { target: { value: "canary" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Create deployment" }));
    await waitFor(() => expect(createDeployment).toHaveBeenCalledTimes(1));
    expect(createDeployment.mock.calls[0][0]).toEqual({
      release_id: "rel_9",
      strategy: "all-targets",
      group: "canary",
    });
  });

  it("surfaces a create error via the error panel and stays on the form", async () => {
    createDeployment.mockRejectedValue(
      new ApiError(404, "RELEASE_NOT_FOUND", "no such release", "req-404"),
    );
    renderCreate("rel_x");
    fireEvent.click(screen.getByRole("button", { name: "Create deployment" }));
    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("404 RELEASE_NOT_FOUND");
    });
    expect(screen.queryByText("detail stub")).not.toBeInTheDocument();
  });
});

describe("DeploymentDetail — progress + rollout + recall", () => {
  const DEP_ID = "dep_show";

  function renderDetail() {
    return render(
      <MemoryRouter initialEntries={[`/deployments/${DEP_ID}`]}>
        <Routes>
          <Route path="/deployments/:deploymentId" element={<DeploymentDetail />} />
        </Routes>
      </MemoryRouter>,
    );
  }

  const runningStatus: DeploymentStatus = {
    deployment_id: DEP_ID,
    release_id: "rel_1",
    strategy: "all-targets",
    status: "running",
    progress: { pending: 2, downloading: 1, installed: 3, succeeded: 4, failed: 1 },
    created_at: "2026-06-10T00:00:00Z",
  };

  it("renders the deployment status, per-bucket counts, and a progressbar", async () => {
    getDeployment.mockResolvedValue(runningStatus);
    renderDetail();

    await waitFor(() => expect(screen.getByText(DEP_ID)).toBeInTheDocument());
    // status badge + strategy.
    expect(screen.getByText("running")).toBeInTheDocument();
    // Each count bucket renders its number alongside its label.
    for (const [label, n] of [
      ["pending", "2"],
      ["downloading", "1"],
      ["installed", "3"],
      ["succeeded", "4"],
      ["failed", "1"],
    ] as const) {
      const badge = screen.getByText(label);
      // The number is the <strong> sibling within the same flex cell.
      expect(badge.closest("div")!.textContent).toContain(n);
    }
    // ProgressBar reflects succeeded(4)/total(11) ≈ 36%.
    const bar = screen.getAllByRole("progressbar")[0];
    expect(bar).toHaveAttribute("aria-valuenow", "36");
  });

  it("starts a staged rollout from the 404 'no rollout' empty state", async () => {
    getDeployment.mockResolvedValue(runningStatus);
    // First getRollout → 404 (no rollout). After create, the rollout exists.
    getRollout.mockRejectedValueOnce(new ApiError(404, "NOT_FOUND", "none", "r1"));
    const created: RolloutState = {
      deployment_id: DEP_ID,
      status: "running",
      current_phase: 0,
      phases: [
        { percentage: 10, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 3600, auto_progress: false },
        { percentage: 50, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 3600, auto_progress: false },
        { percentage: 100, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 0, auto_progress: false },
      ],
      updated_at: "2026-06-10T00:00:00Z",
    };
    createRollout.mockResolvedValue(created);
    getRollout.mockResolvedValueOnce(created); // refetch after create

    renderDetail();
    await waitFor(() =>
      expect(screen.getByText("No staged rollout for this deployment yet.")).toBeInTheDocument(),
    );
    fireEvent.click(
      screen.getByRole("button", { name: /Start staged rollout/ }),
    );

    // The phase table renders after creation, with the default 10/50/100 plan.
    await waitFor(() => expect(screen.getByText("10%")).toBeInTheDocument());
    expect(screen.getByText("50%")).toBeInTheDocument();
    expect(screen.getByText("100%")).toBeInTheDocument();
    expect(createRollout).toHaveBeenCalledTimes(1);
    // The created phases are the documented default plan (3 phases ending at 100).
    const sentPhases = (createRollout.mock.calls[0][1] as { phases: unknown[] }).phases;
    expect(sentPhases.length).toBe(3);
  });

  it("evaluates the current phase and renders the engine decision", async () => {
    getDeployment.mockResolvedValue(runningStatus);
    const state: RolloutState = {
      deployment_id: DEP_ID,
      status: "running",
      current_phase: 1,
      phases: [
        { percentage: 10, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 3600, auto_progress: false },
        { percentage: 50, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 3600, auto_progress: true },
        { percentage: 100, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 0, auto_progress: false },
      ],
      updated_at: "2026-06-10T00:00:00Z",
    };
    getRollout.mockReset();
    getRollout.mockResolvedValue(state);
    evaluateRollout.mockResolvedValue({
      action: "advance",
      reason: "thresholds met",
      state,
    });

    renderDetail();
    // The current phase (index 1) is highlighted with the ▶ marker.
    await waitFor(() => expect(screen.getByText("▶ 2")).toBeInTheDocument());
    // phase X of N header reflects current_phase + 1.
    expect(screen.getByText(/phase 2 of 3/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Evaluate" }));

    await waitFor(() => expect(screen.getByText("advance")).toBeInTheDocument());
    expect(screen.getByText("thresholds met")).toBeInTheDocument();
    expect(evaluateRollout).toHaveBeenCalledTimes(1);
    // The evaluate body carries the form's numeric verdict + boolean flag.
    expect(evaluateRollout.mock.calls[0][1]).toEqual({
      success_rate: 0.95,
      error_rate: 0.02,
      post_boot_health_failed: false,
    });
  });

  it("records a recall and refetches the rollback history table", async () => {
    getDeployment.mockResolvedValue(runningStatus);
    // history starts empty, then after recall shows one rollback row.
    const rollbackRow: RollbackRecord = {
      id: "rb_1",
      deployment_id: DEP_ID,
      kind: "rollback",
      from_release_id: "rel_2",
      to_release_id: "rel_1",
      reason: "boot regression",
      triggered_by: "alice",
      created_at: "2026-06-10T03:00:00Z",
    };
    listRollbacks.mockReset();
    listRollbacks
      .mockResolvedValueOnce({ items: [] })
      .mockResolvedValueOnce({ items: [rollbackRow] });
    recallDeployment.mockResolvedValue({
      ...rollbackRow,
      recall_deployment_id: "dep_recall_9",
    });

    renderDetail();
    await waitFor(() =>
      expect(screen.getByText("No rollbacks recorded for this deployment.")).toBeInTheDocument(),
    );

    fireEvent.change(
      screen.getByPlaceholderText("rel_…"),
      { target: { value: "rel_1" } },
    );
    fireEvent.change(
      screen.getByPlaceholderText("e.g. boot regression"),
      { target: { value: "boot regression" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Recall" }));

    // The success line cites the new recall deployment id.
    await waitFor(() =>
      expect(screen.getByText(/Recall recorded → new deployment dep_recall_9/)).toBeInTheDocument(),
    );
    expect(recallDeployment).toHaveBeenCalledTimes(1);
    expect(recallDeployment.mock.calls[0][1]).toEqual({
      to_release_id: "rel_1",
      reason: "boot regression",
    });

    // History refetched → the rollback row now renders in the table.
    await waitFor(() => expect(screen.getByText("rb_1".length ? "alice" : "")).toBeInTheDocument());
    expect(screen.getByText("rel_2 → rel_1")).toBeInTheDocument();
    const kindCell = screen.getByText("rollback");
    expect(kindCell).toBeInTheDocument();
  });

  it("degrades the rollback-history to a 404 empty state", async () => {
    getDeployment.mockResolvedValue(runningStatus);
    listRollbacks.mockReset();
    listRollbacks.mockRejectedValue(new ApiError(404, "NOT_FOUND", "no history", "h"));
    renderDetail();
    await waitFor(() =>
      expect(
        screen.getByText("Rollback history is not available (endpoint returned 404)."),
      ).toBeInTheDocument(),
    );
  });

  it("hides the recall + evaluate controls for a viewer (RoleGate)", async () => {
    mockRoles = ["viewer"];
    getDeployment.mockResolvedValue(runningStatus);
    getRollout.mockReset();
    getRollout.mockResolvedValue({
      deployment_id: DEP_ID,
      status: "running",
      current_phase: 0,
      phases: [
        { percentage: 100, success_threshold: 0.9, error_threshold: 0.1, duration_seconds: 0, auto_progress: false },
      ],
      updated_at: "2026-06-10T00:00:00Z",
    });
    renderDetail();
    await waitFor(() => expect(screen.getByText(DEP_ID)).toBeInTheDocument());
    // Operator-only controls are absent for a viewer.
    expect(screen.queryByRole("button", { name: "Recall" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Evaluate" })).not.toBeInTheDocument();
  });
});
