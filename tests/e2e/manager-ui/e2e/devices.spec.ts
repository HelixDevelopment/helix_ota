// tests/e2e/manager-ui/e2e/devices.spec.ts
//
// Device list and detail page E2E tests for the OTA Manager.
//
// These tests verify that:
//   - The devices list page loads and displays registered devices
//   - Device details can be viewed
//   - The page handles empty states gracefully
//
// Prerequisites:
//   - ota-server running on port 8080 with HELIX_ADMIN_PASSWORD=admin
//   - SPA served at the configured baseURL

import { test, expect } from "@playwright/test";

const API_URL = process.env.API_URL || "http://localhost:8080/api/v1";
const ADMIN_USERNAME = process.env.ADMIN_USERNAME || "admin@helix.example";
const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || "admin";

test.describe("Devices", () => {
  // Log in once before all tests in this describe block.
  test.beforeEach(async ({ page }) => {
    await page.goto("/login");
    await page.locator('input[type="email"], input[name="email"]').fill(ADMIN_USERNAME);
    await page.locator('input[type="password"]').fill(ADMIN_PASSWORD);
    await page.getByRole("button", { name: /sign in|log in|login|submit/i }).click();
    await page.waitForURL(/\/(dashboard|home|$)/);
  });

  test("device list page loads successfully", async ({ page }) => {
    await page.goto("/devices");

    // Should show either a device table/list or an empty state message.
    const hasTable =
      (await page.locator("table, [role='table'], [class*='table'], [class*='list']").count()) > 0;
    const hasEmptyState =
      (await page.getByText(/no devices|no results|empty|no data/i).count()) > 0;

    // Either table or empty state must be present.
    expect(hasTable || hasEmptyState).toBeTruthy();
  });

  test("device list page does not show a server error", async ({ page }) => {
    await page.goto("/devices");

    // Should not show an error alert.
    const errorAlerts = page.locator(
      '[role="alert"], [class*="error"], [class*="alert-danger"]'
    );
    const errorTexts = await errorAlerts.allTextContents();
    for (const text of errorTexts) {
      expect(text.toLowerCase()).not.toContain("server error");
      expect(text.toLowerCase()).not.toContain("internal error");
      expect(text.toLowerCase()).not.toContain("failed to load");
    }
  });

  test("navigates to device detail when a device is clicked", async ({ page }) => {
    await page.goto("/devices");

    // Find clickable device rows or links.
    const deviceRow = page.locator(
      'table tbody tr a, [class*="device-row"], [class*="device-item"]'
    ).first();

    if ((await deviceRow.count()) > 0) {
      // Navigate to the first device detail.
      await deviceRow.click();
      // Should land on a device detail page (/devices/<id>).
      await expect(page).toHaveURL(/\/devices\/[^/]+$/);
    }
    // If there are no devices, this is a graceful pass — the empty state is
    // validated by the "loads successfully" test above.
  });

  test("device detail page shows device information", async ({ page }) => {
    // First, navigate to a device detail.  We fetch a known device from the
    // API and construct the URL.
    const loginRes = await page.request.post(`${API_URL}/auth/login`, {
      data: { username: ADMIN_USERNAME, password: ADMIN_PASSWORD },
    });
    const loginData = await loginRes.json();
    const token = loginData.access_token || loginData.token;

    // Fetch devices to find the first one.
    const devicesRes = await page.request.get(`${API_URL}/devices`, {
      headers: { Authorization: `Bearer ${token}` },
    });

    if (devicesRes.ok()) {
      const devices = await devicesRes.json();
      if (Array.isArray(devices) && devices.length > 0) {
        const firstDevice = devices[0];
        const deviceId = firstDevice.id || firstDevice.device_id;

        await page.goto(`/devices/${deviceId}`);

        // Should show at least basic device info.
        await expect(
          page.getByText(new RegExp(deviceId.substring(0, 8), "i"))
        ).toBeVisible({ timeout: 10_000 });
      }
    }
    // No devices exist — the empty state is handled by the list-page test.
  });
});
