// Helix OTA dashboard — component test for the DashboardOverview screen.
// Drives the screen through its loading / empty / populated / error states by
// stubbing the apiClient module (unit/component-test mock, permitted §11.4.27).
// Includes a component-level accessibility assertion via vitest-axe.

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { axe } from "vitest-axe";
import * as matchers from "vitest-axe/matchers";
import { expect as vitestExpect } from "vitest";
import { ApiError } from "../api/client";
import type { ReleaseList } from "../types/api";

vitestExpect.extend(matchers);

// Mock the singleton client the screen imports.
const listReleases = vi.fn<[], Promise<ReleaseList>>();
const health = vi.fn();
vi.mock("../api/client", async () => {
  const actual = await vi.importActual<typeof import("../api/client")>("../api/client");
  return {
    ...actual,
    apiClient: {
      listReleases: (...args: unknown[]) => listReleases(...(args as [])),
      health: (...args: unknown[]) => health(...(args as [])),
    },
  };
});

// Import AFTER the mock is registered.
import { DashboardOverview } from "./OverviewScreen";

function renderOverview() {
  return render(
    <MemoryRouter>
      <DashboardOverview />
    </MemoryRouter>,
  );
}

const emptyList: ReleaseList = { items: [] };

describe("DashboardOverview", () => {
  beforeEach(() => {
    listReleases.mockReset();
    health.mockReset();
    // health is best-effort; default to a never-resolving promise so the badge stays hidden.
    health.mockReturnValue(new Promise(() => {}));
  });

  it("shows the Loading… placeholder before the releases resolve", () => {
    listReleases.mockReturnValue(new Promise(() => {})); // pending forever
    renderOverview();
    expect(screen.getByRole("heading", { name: "Overview", level: 1 })).toBeInTheDocument();
    expect(screen.getByText("Loading…")).toBeInTheDocument();
  });

  it("renders the 'No releases yet.' empty state for an empty list", async () => {
    listReleases.mockResolvedValue(emptyList);
    renderOverview();
    await waitFor(() => expect(screen.getByText("No releases yet.")).toBeInTheDocument());
    // No table is rendered in the empty state.
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("renders a release row when the list is populated", async () => {
    listReleases.mockResolvedValue({
      items: [
        {
          release_id: "rel_1",
          artifact_id: "art_1",
          version: "1.0.0",
          os: "android",
          target_model: "rk3588",
          status: "published",
          created_at: "2026-06-10T00:00:00Z",
        },
      ],
    });
    renderOverview();
    await waitFor(() => expect(screen.getByText("1.0.0")).toBeInTheDocument());
    expect(screen.getByText("rk3588")).toBeInTheDocument();
    expect(screen.getByText("published")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "view" })).toHaveAttribute(
      "href",
      "/releases/rel_1",
    );
  });

  it("surfaces an ApiError via the error panel", async () => {
    listReleases.mockRejectedValue(new ApiError(500, "INTERNAL", "boom", "req-x"));
    renderOverview();
    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("500 INTERNAL");
      expect(alert).toHaveTextContent("boom");
    });
  });

  it("has no detectable accessibility violations in the empty state", async () => {
    listReleases.mockResolvedValue(emptyList);
    const { container } = renderOverview();
    await waitFor(() => expect(screen.getByText("No releases yet.")).toBeInTheDocument());
    // color-contrast cannot be computed reliably under jsdom (no canvas); it is
    // verified at the Playwright/browser layer instead (e2e/a11y.spec.ts).
    const results = await axe(container, {
      rules: { "color-contrast": { enabled: false } },
    });
    expect(results).toHaveNoViolations();
  });
});
