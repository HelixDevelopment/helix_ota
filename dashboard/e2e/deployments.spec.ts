// Helix OTA dashboard — Deployments panel e2e (real SPA + real control plane).
// Covers the deployment lookup list, the create form (render + submit gating), and
// the DeploymentDetail screen — the richest, previously-zero-coverage screen —
// including the staged-rollout panel and the recall/rollback-history panel.
// Navigation is CLIENT-SIDE (nav links + the in-app "Open by id" lookup form) so the
// in-memory session survives. Driven against the live in-memory API (no mocks, §11.4).

import { expect, test, type Page } from "@playwright/test";
import { login } from "./helpers";

test.beforeEach(async ({ page }) => {
  await login(page);
});

// Reach DeploymentDetail for an arbitrary id WITHOUT a hard reload: use the
// Deployments list's "Open a deployment" lookup form, which client-side-navigates.
async function openDeployment(page: Page, id: string): Promise<void> {
  await page.getByRole("link", { name: "Deployments" }).click();
  await expect(page.getByRole("heading", { name: "Deployments", level: 1 })).toBeVisible();
  await page.getByPlaceholder("dep_…").fill(id);
  await page.getByRole("button", { name: "Open", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Deployment", level: 1 })).toBeVisible();
}

test("Deployment list renders the lookup-by-id form", async ({ page }) => {
  await page.getByRole("link", { name: "Deployments" }).click();
  await expect(page.getByRole("heading", { name: "Deployments", level: 1 })).toBeVisible();
  await expect(page.getByRole("heading", { name: "Open a deployment" })).toBeVisible();
  // Admin sees the gated-in create action.
  await expect(page.getByRole("button", { name: "New deployment" })).toBeVisible();
  // Open is disabled until an id is typed.
  const open = page.getByRole("button", { name: "Open", exact: true });
  await expect(open).toBeDisabled();
  await page.getByPlaceholder("dep_…").fill("dep_123");
  await expect(open).toBeEnabled();
});

test("Deployment create form renders and gates submit on release_id", async ({ page }) => {
  await page.getByRole("link", { name: "Deployments" }).click();
  await page.getByRole("button", { name: "New deployment" }).click();
  await expect(page.getByRole("heading", { name: "New deployment", level: 1 })).toBeVisible();
  // Strategy is locked to all-targets in MVP.
  await expect(page.getByText("Staged / percentage rollout arrives in 1.0.1.")).toBeVisible();

  const create = page.getByRole("button", { name: "Create deployment" });
  await expect(create).toBeDisabled();
  await page
    .getByText("release_id", { exact: true })
    .locator("xpath=following::input[1]")
    .fill("rel_abc");
  await expect(create).toBeEnabled();
});

test("Deployment detail renders the rollout + recall panels with graceful empty states", async ({
  page,
}) => {
  // An unknown deployment id: the detail card errors (404), while the rollout panel
  // degrades to its "no rollout yet" empty state and recall history degrades to its
  // missing/empty state — all panels must render, none may crash the screen.
  await openDeployment(page, "dep_e2e_unknown");

  // Staged rollout panel is present (card title) and shows its empty/no-rollout state.
  await expect(page.getByRole("heading", { name: "Staged rollout" })).toBeVisible();
  await expect(page.getByText("No staged rollout for this deployment yet.")).toBeVisible();
  // The start-rollout action is gated-in for admin.
  await expect(page.getByRole("button", { name: /Start staged rollout/ })).toBeVisible();

  // Recall panel is present with its form and history sub-section.
  await expect(page.getByRole("heading", { name: /Recall \(roll back/ })).toBeVisible();
  await expect(page.getByPlaceholder("rel_…")).toBeVisible();
  await expect(page.getByRole("heading", { name: "Rollback history", level: 3 })).toBeVisible();
  // The recall button is disabled until a to_release_id is entered (admin-gated).
  const recall = page.getByRole("button", { name: "Recall", exact: true });
  await expect(recall).toBeDisabled();
  await page.getByPlaceholder("rel_…").fill("rel_good");
  await expect(recall).toBeEnabled();
});

test("Deployment detail error panel surfaces the live 404 for the unknown deployment", async ({
  page,
}) => {
  await openDeployment(page, "dep_e2e_unknown_2");
  // The status card hits GET /deployments/{id} which 404s -> at least one ErrorPanel.
  await expect(page.getByRole("alert").first()).toBeVisible();
});
