// Helix OTA dashboard — accessibility e2e via @axe-core/playwright (real browser).
// Runs the full axe WCAG 2.0/2.1 A+AA ruleset against the rendered Login screen
// and an authenticated main panel (Overview). Color-contrast IS evaluated here
// (real browser layout), unlike the jsdom component-level a11y check.

import { expect, test } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { login } from "./helpers";

const TAGS = ["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"];

test("Login screen has no critical/serious accessibility violations", async ({ page }) => {
  await page.goto("/login");
  await expect(page.getByText("Helix OTA — operator login")).toBeVisible();

  const results = await new AxeBuilder({ page }).withTags(TAGS).analyze();
  const blocking = results.violations.filter(
    (v) => v.impact === "critical" || v.impact === "serious",
  );
  // Attach detail to the report when a violation is found (anti-bluff: real ids).
  expect(
    blocking,
    `axe violations: ${blocking.map((v) => `${v.id}(${v.impact})`).join(", ")}`,
  ).toEqual([]);
});

test("Overview panel has no critical/serious accessibility violations", async ({ page }) => {
  await login(page);
  await expect(page.getByText("Recent releases")).toBeVisible();

  const results = await new AxeBuilder({ page }).withTags(TAGS).analyze();
  const blocking = results.violations.filter(
    (v) => v.impact === "critical" || v.impact === "serious",
  );
  expect(
    blocking,
    `axe violations: ${blocking.map((v) => `${v.id}(${v.impact})`).join(", ")}`,
  ).toEqual([]);
});
