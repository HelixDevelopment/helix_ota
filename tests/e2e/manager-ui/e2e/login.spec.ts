// tests/e2e/manager-ui/e2e/login.spec.ts
//
// Login flow E2E tests for the OTA Manager.
//
// These tests verify that:
//   - The login page renders correctly
//   - A user can log in with valid admin credentials
//   - Invalid credentials show an error
//   - The session persists across page reloads
//
// Prerequisites:
//   - ota-server running on port 8080 with HELIX_ADMIN_PASSWORD=admin
//   - SPA served at the configured baseURL

import { test, expect } from "@playwright/test";

const API_URL = process.env.API_URL || "http://localhost:8080/api/v1";
const ADMIN_USERNAME = process.env.ADMIN_USERNAME || "admin@helix.example";
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || "admin";

test.describe("Login flow", () => {
  test("renders the login page", async ({ page }) => {
    await page.goto("/login");

    // The page should display a login form with username/email and password fields.
    await expect(page.locator('input[type="email"], input[name="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();

    // The submit button should be present.
    await expect(
      page.getByRole("button", { name: /sign in|log in|login|submit/i })
    ).toBeVisible();
  });

  test("logs in with valid admin credentials", async ({ page }) => {
    // Intercept the login API call.
    const loginResponsePromise = page.waitForResponse(
      (res) =>
        res.url().includes("/auth/login") && res.request().method() === "POST"
    );

    await page.goto("/login");

    // Fill in credentials.
    await page.locator('input[type="email"], input[name="email"]').fill(ADMIN_USERNAME);
    await page.locator('input[type="password"]').fill(ADMIN_PASSWORD);

    // Submit the form.
    await page.getByRole("button", { name: /sign in|log in|login|submit/i }).click();

    // Wait for the API response.
    const loginResponse = await loginResponsePromise;
    expect(loginResponse.ok()).toBeTruthy();

    // Should redirect to the dashboard or home page.
    await page.waitForURL(/\/(dashboard|home|$)/);
    await expect(page).not.toHaveURL(/\/login/);
  });

  test("shows an error for invalid credentials", async ({ page }) => {
    await page.goto("/login");

    // Fill in invalid credentials.
    await page.locator('input[type="email"], input[name="email"]').fill("bad@example.com");
    await page.locator('input[type="password"]').fill("wrongpassword");

    // Submit.
    await page.getByRole("button", { name: /sign in|log in|login|submit/i }).click();

    // Should show an error message.
    await expect(
      page.locator("text=/invalid|error|failed|incorrect/i")
    ).toBeVisible({ timeout: 10_000 });

    // Should stay on the login page.
    await expect(page).toHaveURL(/\/login/);
  });

  test("persists session across page reload", async ({ page }) => {
    // Log in first.
    await page.goto("/login");
    await page.locator('input[type="email"], input[name="email"]').fill(ADMIN_USERNAME);
    await page.locator('input[type="password"]').fill(ADMIN_PASSWORD);
    await page.getByRole("button", { name: /sign in|log in|login|submit/i }).click();
    await page.waitForURL(/\/(dashboard|home|$)/);

    // Reload the page.
    await page.reload();

    // Should still be logged in (no redirect to /login).
    await expect(page).not.toHaveURL(/\/login/);
    await expect(page.getByText(/devices|dashboard|artifacts|deployments/i)).toBeVisible();
  });

  test("redirects to login when accessing protected route while unauthenticated", async ({
    page,
  }) => {
    // Try to access a protected page directly.
    await page.goto("/devices");

    // Should be redirected to the login page.
    await expect(page).toHaveURL(/\/login/);
  });
});
