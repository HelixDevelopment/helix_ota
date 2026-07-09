// Helix OTA dashboard — component test for the AppShell theme-toggle wiring.
//
// Anti-bluff (§11.4 / §11.4.142 review): renders the REAL AppShell and clicks
// the REAL theme-toggle button the operator sees, asserting the <html>
// `data-theme` attribute flips light<->dark AND the button's accessible label
// re-renders to match. This proves the click handler is wired to theme.ts
// end-to-end (the seam the §11.4.170 host-render harness bypasses by stamping
// data-theme directly). @testing-library/react is already a devDependency, so
// no new dependency is introduced.

import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";

// Stub the auth context so AppShell mounts without a real session/backend
// (unit/component-test mock, permitted §11.4.27). Only the fields AppShell reads.
vi.mock("../auth/AuthContext", () => ({
  useAuth: () => ({
    subject: "operator",
    roles: ["admin"],
    logout: vi.fn(),
  }),
}));

// Import AFTER the mock is registered.
import { AppShell } from "./AppShell";

function renderShell() {
  return render(
    <MemoryRouter initialEntries={["/"]}>
      <AppShell />
    </MemoryRouter>,
  );
}

beforeEach(() => {
  // Start from an explicit known theme so the initial button label is
  // deterministic (AppShell seeds its state from currentTheme() == the DOM).
  document.documentElement.setAttribute("data-theme", "light");
});

afterEach(() => {
  document.documentElement.removeAttribute("data-theme");
  vi.restoreAllMocks();
});

describe("AppShell theme toggle", () => {
  it("flips <html> data-theme light<->dark on each click and updates the button label", () => {
    renderShell();

    // Initial: light -> the toggle offers to switch to dark.
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    const toggle = screen.getByRole("button", { name: "Switch to dark theme" });

    // Click 1: light -> dark.
    fireEvent.click(toggle);
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    // The button re-rendered: it now offers to switch back to light.
    expect(
      screen.getByRole("button", { name: "Switch to light theme" }),
    ).toBeInTheDocument();

    // Click 2: dark -> light (proves it is a genuine flip, not a one-way set).
    fireEvent.click(screen.getByRole("button", { name: "Switch to light theme" }));
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(
      screen.getByRole("button", { name: "Switch to dark theme" }),
    ).toBeInTheDocument();
  });
});
