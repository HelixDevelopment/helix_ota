# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: devices.spec.ts >> Devices >> device list page does not show a server error
- Location: e2e/devices.spec.ts:43:7

# Error details

```
Test timeout of 30000ms exceeded while running "beforeEach" hook.
```

```
Error: page.waitForURL: Test timeout of 30000ms exceeded.
=========================== logs ===========================
waiting for navigation until "load"
  navigated to "http://localhost:5173/login?email=admin%40helix.example&password=admin"
============================================================
```

# Page snapshot

```yaml
- generic [ref=e4]:
  - generic [ref=e5]:
    - generic [ref=e6]: OTA Manager
    - generic [ref=e7]: Sign in to access the OTA management dashboard.
  - generic [ref=e10]:
    - generic [ref=e11]:
      - text: Email
      - textbox "you@example.com" [ref=e13]
    - generic [ref=e14]:
      - text: Password
      - textbox "Enter your password" [ref=e16]
    - button "Sign in" [ref=e17]
```

# Test source

```ts
  1   | // tests/e2e/manager-ui/e2e/devices.spec.ts
  2   | //
  3   | // Device list and detail page E2E tests for the OTA Manager.
  4   | //
  5   | // These tests verify that:
  6   | //   - The devices list page loads and displays registered devices
  7   | //   - Device details can be viewed
  8   | //   - The page handles empty states gracefully
  9   | //
  10  | // Prerequisites:
  11  | //   - ota-server running on port 8080 with HELIX_ADMIN_PASSWORD=admin
  12  | //   - SPA served at the configured baseURL
  13  | 
  14  | import { test, expect } from "@playwright/test";
  15  | 
  16  | const API_URL = process.env.API_URL || "http://localhost:8080/api/v1";
  17  | const ADMIN_USERNAME = process.env.ADMIN_USERNAME || "admin@helix.example";
  18  | const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || "admin";
  19  | 
  20  | test.describe("Devices", () => {
  21  |   // Log in once before all tests in this describe block.
  22  |   test.beforeEach(async ({ page }) => {
  23  |     await page.goto("/login");
  24  |     await page.locator('input[type="email"], input[name="email"]').fill(ADMIN_USERNAME);
  25  |     await page.locator('input[type="password"]').fill(ADMIN_PASSWORD);
  26  |     await page.getByRole("button", { name: /sign in|log in|login|submit/i }).click();
> 27  |     await page.waitForURL(/\/(dashboard|home|$)/);
      |                ^ Error: page.waitForURL: Test timeout of 30000ms exceeded.
  28  |   });
  29  | 
  30  |   test("device list page loads successfully", async ({ page }) => {
  31  |     await page.goto("/devices");
  32  | 
  33  |     // Should show either a device table/list or an empty state message.
  34  |     const hasTable =
  35  |       (await page.locator("table, [role='table'], [class*='table'], [class*='list']").count()) > 0;
  36  |     const hasEmptyState =
  37  |       (await page.getByText(/no devices|no results|empty|no data/i).count()) > 0;
  38  | 
  39  |     // Either table or empty state must be present.
  40  |     expect(hasTable || hasEmptyState).toBeTruthy();
  41  |   });
  42  | 
  43  |   test("device list page does not show a server error", async ({ page }) => {
  44  |     await page.goto("/devices");
  45  | 
  46  |     // Should not show an error alert.
  47  |     const errorAlerts = page.locator(
  48  |       '[role="alert"], [class*="error"], [class*="alert-danger"]'
  49  |     );
  50  |     const errorTexts = await errorAlerts.allTextContents();
  51  |     for (const text of errorTexts) {
  52  |       expect(text.toLowerCase()).not.toContain("server error");
  53  |       expect(text.toLowerCase()).not.toContain("internal error");
  54  |       expect(text.toLowerCase()).not.toContain("failed to load");
  55  |     }
  56  |   });
  57  | 
  58  |   test("navigates to device detail when a device is clicked", async ({ page }) => {
  59  |     await page.goto("/devices");
  60  | 
  61  |     // Find clickable device rows or links.
  62  |     const deviceRow = page.locator(
  63  |       'table tbody tr a, [class*="device-row"], [class*="device-item"]'
  64  |     ).first();
  65  | 
  66  |     if ((await deviceRow.count()) > 0) {
  67  |       // Navigate to the first device detail.
  68  |       await deviceRow.click();
  69  |       // Should land on a device detail page (/devices/<id>).
  70  |       await expect(page).toHaveURL(/\/devices\/[^/]+$/);
  71  |     }
  72  |     // If there are no devices, this is a graceful pass — the empty state is
  73  |     // validated by the "loads successfully" test above.
  74  |   });
  75  | 
  76  |   test("device detail page shows device information", async ({ page }) => {
  77  |     // First, navigate to a device detail.  We fetch a known device from the
  78  |     // API and construct the URL.
  79  |     const loginRes = await page.request.post(`${API_URL}/auth/login`, {
  80  |       data: { username: ADMIN_USERNAME, password: ADMIN_PASSWORD },
  81  |     });
  82  |     const loginData = await loginRes.json();
  83  |     const token = loginData.access_token || loginData.token;
  84  | 
  85  |     // Fetch devices to find the first one.
  86  |     const devicesRes = await page.request.get(`${API_URL}/devices`, {
  87  |       headers: { Authorization: `Bearer ${token}` },
  88  |     });
  89  | 
  90  |     if (devicesRes.ok()) {
  91  |       const devices = await devicesRes.json();
  92  |       if (Array.isArray(devices) && devices.length > 0) {
  93  |         const firstDevice = devices[0];
  94  |         const deviceId = firstDevice.id || firstDevice.device_id;
  95  | 
  96  |         await page.goto(`/devices/${deviceId}`);
  97  | 
  98  |         // Should show at least basic device info.
  99  |         await expect(
  100 |           page.getByText(new RegExp(deviceId.substring(0, 8), "i"))
  101 |         ).toBeVisible({ timeout: 10_000 });
  102 |       }
  103 |     }
  104 |     // No devices exist — the empty state is handled by the list-page test.
  105 |   });
  106 | });
  107 | 
```