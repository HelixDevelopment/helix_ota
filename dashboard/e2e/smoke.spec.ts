// Helix OTA dashboard — end-to-end smoke test (anti-bluff browser proof).
// Drives a real Chromium against the real SPA (Vite dev) wired through the /api
// proxy to the real Go control plane booted in global-setup. Logs in with the
// seeded admin credentials and asserts authenticated screens render and consume
// the live API. No mocks (Constitution §11.4).

import { expect, test } from "@playwright/test";

// Same constants as e2e/global-setup.ts (test-only, non-secret).
const USER = "admin@helix.example";
const PASS = "e2e-smoke-pass-1234";

async function login(page: import("@playwright/test").Page) {
  await page.goto("/login");
  await expect(page.getByText("Helix OTA — operator login")).toBeVisible();
  await page.getByPlaceholder("operator@example.com").fill(USER);
  await page.locator('input[type="password"]').fill(PASS);
  await page.getByRole("button", { name: "Sign in" }).click();
}

test("login screen renders", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByText("Helix OTA — operator login")).toBeVisible();
  await expect(page.getByRole("button", { name: "Sign in" })).toBeVisible();
});

test("operator logs in and the Overview screen renders against the live API", async ({ page }) => {
  await login(page);
  // After auth we land on "/" (Overview) and the nav shell appears.
  await expect(page.getByRole("heading", { name: "Overview", level: 1 })).toBeVisible();
  await expect(page.getByRole("link", { name: "Releases" })).toBeVisible();
  // The "Recent releases" card hits GET /releases. The card always renders; its
  // body is the empty-state OR a populated table depending on whether another spec
  // seeded a release into the shared in-memory server (populated-detail.spec.ts).
  // The empty-state itself is asserted strictly at the component layer
  // (src/screens/OverviewScreen.test.tsx). Here we only require the card to render
  // a non-error releases surface against the live API.
  await expect(page.getByText("Recent releases")).toBeVisible();
  await expect(page.getByText("No releases yet.").or(page.getByRole("table")).first()).toBeVisible();
  // Whichever state, the releases fetch did NOT error.
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("Fleet overview consumes the live telemetry overview endpoint", async ({ page }) => {
  await login(page);
  await page.getByRole("link", { name: "Fleet" }).click();
  await expect(page.getByRole("heading", { name: "Fleet", level: 1 })).toBeVisible();
  // GET /telemetry/overview returns failure_rate 0 on a fresh server -> "0.0%".
  await expect(page.getByText("update failure rate")).toBeVisible();
  await expect(page.getByText("0.0%")).toBeVisible();
});

test("Groups screen renders and lists from the live groups endpoint", async ({ page }) => {
  await login(page);
  await page.getByRole("link", { name: "Groups" }).click();
  await expect(page.getByRole("heading", { name: "Device groups", level: 1 })).toBeVisible();
  // The screen consumes the live GET /groups endpoint. On a pristine server this is the
  // empty state; if a prior spec seeded groups into the shared in-memory server the list
  // table renders instead. Assert order-independently that one of the two live-data views
  // appears (the "Refresh" button only renders once data has been fetched, proving the
  // endpoint was consumed). The dedicated empty-state proof lives in groups-detail.spec.ts.
  await expect(
    page.getByText("No groups yet.").or(page.getByRole("button", { name: "Refresh" })),
  ).toBeVisible();
});

test("Audit screen renders and consumes the live admin-only audit endpoint", async ({ page }) => {
  await login(page);
  await page.getByRole("link", { name: "Audit" }).click();
  await expect(page.getByRole("heading", { name: "Audit log", level: 1 })).toBeVisible();
  await expect(page.getByRole("button", { name: "Apply filter" })).toBeVisible();
});
