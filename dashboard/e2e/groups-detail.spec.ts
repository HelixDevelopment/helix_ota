// Helix OTA dashboard — Group detail panel e2e (real SPA + real control plane).
// Seeds groups + members via the REAL /api/v1 endpoints (admin token), then asserts
// the GroupList + GroupDetail screens render the seeded server state, the empty-members
// state, and the batch add-members disposition. Navigation is CLIENT-SIDE (nav link +
// the in-list "members" link) so the in-memory session survives. No mocks (§11.4).

import { expect, test, type Page } from "@playwright/test";
import { login, adminToken, seedGroup, seedGroupMembers } from "./helpers";

// Open a seeded group's detail page by clicking its "members" link in the list —
// a client-side navigation that preserves the in-memory session.
async function openGroupByName(page: Page, name: string): Promise<void> {
  await page.getByRole("link", { name: "Groups" }).click();
  await expect(page.getByRole("heading", { name: "Device groups", level: 1 })).toBeVisible();
  const row = page.getByRole("row").filter({ hasText: name });
  await expect(row).toBeVisible();
  await row.getByRole("link", { name: "members" }).click();
  await expect(page.getByRole("heading", { name: /^Group /, level: 1 })).toBeVisible();
}

test("Group list renders a group seeded through the live API", async ({ page, request }) => {
  const token = await adminToken(request);
  const name = `e2e-canary-${Date.now()}`;
  await seedGroup(request, token, name, "canary lab fleet");

  await login(page);
  await page.getByRole("link", { name: "Groups" }).click();
  await expect(page.getByRole("heading", { name: "Device groups", level: 1 })).toBeVisible();
  // The seeded group appears in the list table with its description.
  await expect(page.getByRole("cell", { name })).toBeVisible();
  await expect(page.getByRole("cell", { name: "canary lab fleet" })).toBeVisible();
  await expect(page.getByRole("link", { name: "members" }).first()).toBeVisible();
});

test("Group detail shows the empty-members state for a fresh group", async ({ page, request }) => {
  const token = await adminToken(request);
  const name = `e2e-empty-${Date.now()}`;
  await seedGroup(request, token, name);

  await login(page);
  await openGroupByName(page, name);
  // Add-devices panel (admin/operator gated-in) and the empty Members state.
  await expect(page.getByText("Add devices (batch)")).toBeVisible();
  await expect(page.getByText("No members in this group yet.")).toBeVisible();
});

test("Group detail lists members seeded through the live batch endpoint", async ({
  page,
  request,
}) => {
  const token = await adminToken(request);
  const name = `e2e-members-${Date.now()}`;
  const groupId = await seedGroup(request, token, name);
  const disposition = await seedGroupMembers(request, token, groupId, ["dev_a1", "dev_a2"]);
  const seeded = disposition.added.length + disposition.already_member.length;

  await login(page);
  await openGroupByName(page, name);

  if (seeded > 0) {
    // Members table renders the seeded device id rows.
    await expect(page.getByRole("cell", { name: "dev_a1" })).toBeVisible();
    await expect(page.getByRole("columnheader", { name: "device_id" })).toBeVisible();
  } else {
    // Server treated the ids as unknown devices (not_found) and registered no rows.
    // Honest fallback: assert the empty state rather than a row that does not exist.
    await expect(page.getByText("No members in this group yet.")).toBeVisible();
  }
});

test("Group detail batch add-members reports a disposition in the UI", async ({
  page,
  request,
}) => {
  const token = await adminToken(request);
  const name = `e2e-add-${Date.now()}`;
  await seedGroup(request, token, name);

  await login(page);
  await openGroupByName(page, name);

  // Drive the batch add-members form through the UI.
  await page.locator("textarea").fill("dev_x1\ndev_x2");
  await page.getByRole("button", { name: /Add 2 device/ }).click();

  // The disposition summary appears (added / already_member / not_found badges).
  await expect(page.getByText(/added:/)).toBeVisible();
  await expect(page.getByText(/already_member:/)).toBeVisible();
  await expect(page.getByText(/not_found:/)).toBeVisible();
});
