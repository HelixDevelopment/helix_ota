// Helix OTA dashboard — component test for the AuditScreen (operational_endpoints.md §4).
// Drives the screen through loading / populated / empty / 403-forbidden / 404-missing /
// generic-error states AND the filter-apply interaction (which re-commits the query +
// re-issues listAudit with the typed filter args). Only apiClient.listAudit is stubbed
// (unit/component-test mock, permitted §11.4.27) — every assertion exercises real
// user-visible behaviour (table rows, actor projection, empty/error panels, the
// next_cursor hint, the Apply/Reset form wiring).

import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent, within } from "@testing-library/react";
import { ApiError } from "../api/client";
import type { AuditList, AuditQuery } from "../types/api";

const listAudit = vi.fn<[AuditQuery | undefined], Promise<AuditList>>();
vi.mock("../api/client", async () => {
  const actual = await vi.importActual<typeof import("../api/client")>("../api/client");
  return {
    ...actual,
    apiClient: {
      listAudit: (...args: unknown[]) => listAudit(...(args as [AuditQuery | undefined])),
    },
  };
});

// Import AFTER the mock is registered.
import { AuditScreen } from "./AuditScreen";

function renderScreen() {
  return render(<AuditScreen />);
}

const twoEntries: AuditList = {
  items: [
    {
      id: "aud_1",
      actor: { subject: "alice", user_id: "usr_alice" },
      action: "RELEASE_CREATE",
      resource_type: "release",
      resource_id: "rel_42",
      ip_address: "10.0.0.7",
      created_at: "2026-06-10T09:00:00Z",
    },
    {
      id: "aud_2",
      // no user_id, no resource_id, no ip — exercises the fallback projections.
      actor: { subject: "system" },
      action: "DEPLOYMENT_HALT",
      resource_type: "deployment",
      created_at: "2026-06-10T08:00:00Z",
    },
  ],
  next_cursor: null,
};

describe("AuditScreen", () => {
  beforeEach(() => {
    listAudit.mockReset();
  });

  it("issues the initial listAudit with the default limit and shows Loading… first", () => {
    listAudit.mockReturnValue(new Promise(() => {})); // pending forever
    renderScreen();
    expect(screen.getByRole("heading", { name: "Audit log", level: 1 })).toBeInTheDocument();
    expect(screen.getByText("Loading…")).toBeInTheDocument();
    // The committed query on first mount is { limit: 50 } with no filters.
    expect(listAudit).toHaveBeenCalledTimes(1);
    expect(listAudit.mock.calls[0][0]).toEqual({ limit: 50 });
  });

  it("renders audit rows with actor + resource projections once loaded", async () => {
    listAudit.mockResolvedValue(twoEntries);
    renderScreen();

    await waitFor(() => expect(screen.getByText("RELEASE_CREATE")).toBeInTheDocument());
    const table = screen.getByRole("table");
    const rows = table.querySelectorAll("tbody tr");
    expect(rows.length).toBe(2);

    // Row 0: actor subject + user_id projection, resource_type + resource_id, ip.
    expect(rows[0].textContent).toContain("alice");
    expect(rows[0].textContent).toContain("usr_alice");
    expect(rows[0].textContent).toContain("rel_42");
    expect(rows[0].textContent).toContain("10.0.0.7");
    // Row 1: no user_id / no resource_id / no ip -> the em-dash IP fallback renders.
    expect(rows[1].textContent).toContain("system");
    expect(rows[1].textContent).toContain("DEPLOYMENT_HALT");
    expect(within(rows[1] as HTMLElement).getByText("—")).toBeInTheDocument();
    // No empty/error states present.
    expect(screen.queryByText("No audit entries match.")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("renders the 'No audit entries match.' empty state for an empty list", async () => {
    listAudit.mockResolvedValue({ items: [], next_cursor: null });
    renderScreen();
    await waitFor(() => expect(screen.getByText("No audit entries match.")).toBeInTheDocument());
    expect(screen.queryByRole("table")).not.toBeInTheDocument();
  });

  it("shows the admin-only message on a 403 (NOT a raw error panel)", async () => {
    listAudit.mockRejectedValue(new ApiError(403, "FORBIDDEN", "admin required", "req-403"));
    renderScreen();
    await waitFor(() =>
      expect(screen.getByText("The audit log requires the admin role.")).toBeInTheDocument(),
    );
    // A 403 is a graceful gate, not an error alert.
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("shows the not-available message on a 404 (NOT a raw error panel)", async () => {
    listAudit.mockRejectedValue(new ApiError(404, "NOT_FOUND", "no audit", "req-404"));
    renderScreen();
    await waitFor(() =>
      expect(
        screen.getByText("The audit log is not available (endpoint returned 404)."),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("surfaces a non-403/404 ApiError via the error panel", async () => {
    listAudit.mockRejectedValue(new ApiError(500, "INTERNAL", "boom", "req-500"));
    renderScreen();
    await waitFor(() => {
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent("500 INTERNAL");
      expect(alert).toHaveTextContent("boom");
    });
  });

  it("re-issues listAudit with action + resource_type filters when Apply is clicked", async () => {
    listAudit.mockResolvedValue({ items: [], next_cursor: null });
    renderScreen();
    await waitFor(() => expect(listAudit).toHaveBeenCalledTimes(1));

    // Fill the two text filters and Apply.
    fireEvent.change(
      screen.getByText("action (e.g. RELEASE_CREATE)").parentElement!.querySelector("input")!,
      { target: { value: "RELEASE_CREATE" } },
    );
    fireEvent.change(
      screen.getByText("resource_type (e.g. release)").parentElement!.querySelector("input")!,
      { target: { value: "release" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Apply filter" }));

    // The query was re-committed (deps change) → a second call with the filter args.
    await waitFor(() => expect(listAudit).toHaveBeenCalledTimes(2));
    expect(listAudit.mock.calls[1][0]).toEqual({
      limit: 50,
      action: "RELEASE_CREATE",
      resource_type: "release",
    });
  });

  it("Reset clears applied filters and re-queries with the bare default", async () => {
    listAudit.mockResolvedValue({ items: [], next_cursor: null });
    renderScreen();
    await waitFor(() => expect(listAudit).toHaveBeenCalledTimes(1));

    fireEvent.change(
      screen.getByText("action (e.g. RELEASE_CREATE)").parentElement!.querySelector("input")!,
      { target: { value: "RELEASE_CREATE" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Apply filter" }));
    await waitFor(() => expect(listAudit).toHaveBeenCalledTimes(2));

    fireEvent.click(screen.getByRole("button", { name: "Reset" }));
    // Reset commits { limit: 50 } again — a fresh query distinct from the filtered one.
    await waitFor(() => expect(listAudit).toHaveBeenCalledTimes(3));
    expect(listAudit.mock.calls[2][0]).toEqual({ limit: 50 });
  });

  it("renders the next_cursor 'more entries' hint when the page is not the last", async () => {
    listAudit.mockResolvedValue({ ...twoEntries, next_cursor: "cur_next_99" });
    renderScreen();
    await waitFor(() =>
      expect(screen.getByText(/More entries available/)).toBeInTheDocument(),
    );
    expect(screen.getByText(/cur_next_99/)).toBeInTheDocument();
  });
});
