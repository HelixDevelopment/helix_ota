// Helix OTA dashboard — Releases panel e2e (real SPA + real control plane).
// Covers the list (empty state on a fresh server) and the create form (render +
// client-side submit-disabled validation). Navigation is CLIENT-SIDE (nav links /
// in-app links) because the session lives in memory only — a hard page.goto to a
// protected route wipes it and redirects to /login (AuthContext §7.1). No mocks.

import { expect, test } from "@playwright/test";
import { login } from "./helpers";

test.beforeEach(async ({ page }) => {
  await login(page);
});

test("Releases list renders against the live API (empty-state or populated list)", async ({
  page,
}) => {
  await page.getByRole("link", { name: "Releases" }).click();
  await expect(page.getByRole("heading", { name: "Releases", level: 1 })).toBeVisible();
  // GET /releases renders either the empty-state OR a populated table, depending on
  // whether populated-detail.spec.ts seeded a release into the shared in-memory
  // server. The empty-state is asserted strictly at the component layer
  // (src/screens/OverviewScreen.test.tsx + ReleaseList logic); here we require the
  // list surface to render without error.
  await expect(
    page.getByText("No releases yet.").or(page.getByRole("table")).first(),
  ).toBeVisible();
  await expect(page.getByRole("alert")).toHaveCount(0);
  // As admin, the create-action is gated-in and visible.
  await expect(page.getByRole("link", { name: "New release" })).toBeVisible();
});

test("Release create form renders and gates submit until required fields are set", async ({
  page,
}) => {
  // Client-side navigation preserves the in-memory session.
  await page.getByRole("link", { name: "Releases" }).click();
  await page.getByRole("link", { name: "New release" }).click();
  await expect(page.getByRole("heading", { name: "New release", level: 1 })).toBeVisible();

  const publish = page.getByRole("button", { name: "Publish release" });
  // Empty required fields -> submit disabled.
  await expect(publish).toBeDisabled();

  // Fill all four required fields -> submit enabled (os is pre-filled to "android").
  await page.getByText("artifact_id").locator("xpath=following::input[1]").fill("art_test");
  await page
    .getByText("version", { exact: true })
    .locator("xpath=following::input[1]")
    .fill("1.0.0");
  await page
    .getByText("target_model")
    .locator("xpath=following::input[1]")
    .fill("rk3588");
  await expect(publish).toBeEnabled();
});
