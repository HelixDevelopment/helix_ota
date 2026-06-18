// Helix OTA dashboard — POPULATED Release + Deployment detail e2e (real SPA + real
// control plane). Closes the documented gap: previously the detail screens were
// covered only at the empty/error level because seeding a release needs an uploaded
// AND verified (ed25519-signed) artifact the harness did not mint. seedArtifact now
// mints a GENUINELY signed artifact (verified by the real server), so these specs
// drive a real release/deployment/rollout through to a populated UI render.
//
// Navigation is CLIENT-SIDE (Releases-list "view" link + the Deployments "Open by id"
// lookup form) so the in-memory session survives (AuthContext §7.1). No mocks (§11.4):
// every rendered value traces to genuine server state seeded via the same /api/v1
// endpoints the SPA uses.

import { expect, test } from "@playwright/test";
import {
  adminToken,
  login,
  seedArtifact,
  seedDeployment,
  seedRelease,
  seedRollout,
} from "./helpers";

test("Release detail renders a populated release seeded from a real signed artifact", async ({
  page,
  request,
}) => {
  const token = await adminToken(request);
  // A monotonic version that won't collide with sibling seeded releases for this target.
  const version = `7.${Date.now() % 100000}.0`;
  const targetModel = `rk-pop-rel-${Date.now()}`;
  const artifact = await seedArtifact(request, token, { version, targetModel });
  const releaseId = await seedRelease(request, token, artifact);

  await login(page);

  // The seeded release appears in the Releases list; click its "view" link
  // (client-side nav) to land on the populated detail.
  await page.getByRole("link", { name: "Releases" }).click();
  await expect(page.getByRole("heading", { name: "Releases", level: 1 })).toBeVisible();
  const row = page.getByRole("row").filter({ hasText: version });
  await expect(row).toBeVisible();
  await row.getByRole("link", { name: "view" }).click();

  // Populated Release detail: the success path (data present, no error/empty panel).
  await expect(page.getByRole("heading", { name: "Release", level: 1 })).toBeVisible();
  // Every seeded field renders.
  await expect(page.getByText(releaseId, { exact: true })).toBeVisible();
  await expect(page.getByText(version, { exact: true })).toBeVisible();
  await expect(page.getByText(targetModel, { exact: true })).toBeVisible();
  await expect(page.getByText(artifact.artifactId, { exact: true })).toBeVisible();
  // The operator-gated "Deploy this release" action is present on the populated detail.
  await expect(page.getByRole("button", { name: "Deploy this release" })).toBeVisible();
  // The success path renders NO error alert.
  await expect(page.getByRole("alert")).toHaveCount(0);
});

test("Deployment detail renders a populated deployment with a real staged-rollout panel", async ({
  page,
  request,
}) => {
  const token = await adminToken(request);
  const version = `8.${Date.now() % 100000}.0`;
  const targetModel = `rk-pop-dep-${Date.now()}`;
  const artifact = await seedArtifact(request, token, { version, targetModel });
  const releaseId = await seedRelease(request, token, artifact);
  const deploymentId = await seedDeployment(request, token, releaseId);
  await seedRollout(request, token, deploymentId);

  await login(page);

  // Reach the populated Deployment detail via the in-app "Open by id" lookup form.
  await page.getByRole("link", { name: "Deployments" }).click();
  await expect(page.getByRole("heading", { name: "Deployments", level: 1 })).toBeVisible();
  await page.getByPlaceholder("dep_…").fill(deploymentId);
  await page.getByRole("button", { name: "Open", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Deployment", level: 1 })).toBeVisible();

  // Populated status card: the real deployment id + an active status badge render.
  await expect(page.getByText(deploymentId, { exact: true })).toBeVisible();
  await expect(page.getByText("all-targets")).toBeVisible();
  // The progress counts row (succeeded/failed/etc.) is present for the live deployment.
  await expect(page.getByText("succeeded", { exact: true })).toBeVisible();

  // Staged rollout panel shows REAL phase data (not the "no rollout yet" empty state).
  await expect(page.getByRole("heading", { name: "Staged rollout" })).toBeVisible();
  await expect(page.getByText("No staged rollout for this deployment yet.")).toHaveCount(0);
  // The three seeded phases render in the rollout table.
  await expect(page.getByText("10%", { exact: true })).toBeVisible();
  await expect(page.getByText("50%", { exact: true })).toBeVisible();
  await expect(page.getByText("100%", { exact: true })).toBeVisible();
  // The "Evaluate current phase" controls are gated-in for admin once a rollout exists.
  await expect(page.getByRole("button", { name: "Evaluate" })).toBeVisible();
});

test("seeded signed artifact is retrievable + verified via the real artifact detail API path", async ({
  request,
}) => {
  // Sink-side proof (§11.4.69): the server itself confirms the seeded artifact is
  // stored AND verified — the upload was genuinely signed, not faked.
  const token = await adminToken(request);
  const version = `9.${Date.now() % 100000}.0`;
  const artifact = await seedArtifact(request, token, {
    version,
    targetModel: `rk-pop-art-${Date.now()}`,
  });

  const res = await request.get(
    `http://localhost:8095/api/v1/artifacts/${encodeURIComponent(artifact.artifactId)}`,
    { headers: { Authorization: `Bearer ${token}` } },
  );
  expect(res.status()).toBe(200);
  const body = (await res.json()) as { artifact_id: string; verified: boolean; sha256: string };
  expect(body.artifact_id).toBe(artifact.artifactId);
  expect(body.verified).toBe(true);
  expect(body.sha256).toBe(artifact.sha256);
});
