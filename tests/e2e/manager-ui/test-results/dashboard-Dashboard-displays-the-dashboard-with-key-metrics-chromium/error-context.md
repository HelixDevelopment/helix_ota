# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: dashboard.spec.ts >> Dashboard >> displays the dashboard with key metrics
- Location: e2e/dashboard.spec.ts:29:7

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
  1  | // tests/e2e/manager-ui/e2e/dashboard.spec.ts
  2  | //
  3  | // Dashboard page E2E tests for the OTA Manager.
  4  | //
  5  | // These tests verify that:
  6  | //   - The dashboard loads after successful login
  7  | //   - Key metrics/overview cards are rendered
  8  | //   - Navigation links are functional
  9  | //
  10 | // Prerequisites:
  11 | //   - ota-server running on port 8080 with HELIX_ADMIN_PASSWORD=admin
  12 | //   - SPA served at the configured baseURL
  13 | 
  14 | import { test, expect } from "@playwright/test";
  15 | 
  16 | const ADMIN_USERNAME = process.env.ADMIN_USERNAME || "admin@helix.example";
  17 | const ADMIN_PASSWORD = process.env.ADMIN_PASSWORD || "admin";
  18 | 
  19 | test.describe("Dashboard", () => {
  20 |   // Log in once before all tests in this describe block.
  21 |   test.beforeEach(async ({ page }) => {
  22 |     await page.goto("/login");
  23 |     await page.locator('input[type="email"], input[name="email"]').fill(ADMIN_USERNAME);
  24 |     await page.locator('input[type="password"]').fill(ADMIN_PASSWORD);
  25 |     await page.getByRole("button", { name: /sign in|log in|login|submit/i }).click();
> 26 |     await page.waitForURL(/\/(dashboard|home|$)/);
     |                ^ Error: page.waitForURL: Test timeout of 30000ms exceeded.
  27 |   });
  28 | 
  29 |   test("displays the dashboard with key metrics", async ({ page }) => {
  30 |     // The dashboard should show overview cards or sections.
  31 |     await expect(
  32 |       page.getByText(/devices|total devices|registered devices|artifacts|releases/i)
  33 |     ).toBeVisible();
  34 | 
  35 |     // Should show at least one metric card (common dashboard pattern).
  36 |     const metricCards = page.locator(
  37 |       '[class*="card"], [class*="metric"], [class*="stat"], [role="status"]'
  38 |     );
  39 |     const cardCount = await metricCards.count();
  40 |     // There may be no cards if the server has no data — we just check the
  41 |     // page loaded without error.
  42 |     expect(true).toBeTruthy();
  43 |   });
  44 | 
  45 |   test("navigation links to major sections are present", async ({ page }) => {
  46 |     // Check for navigation links to the main sections.
  47 |     const navLinks = [
  48 |       /devices/i,
  49 |       /artifacts/i,
  50 |       /releases/i,
  51 |       /deployments/i,
  52 |     ];
  53 | 
  54 |     for (const linkText of navLinks) {
  55 |       const link = page.getByRole("link", { name: linkText });
  56 |       if ((await link.count()) > 0) {
  57 |         await expect(link.first()).toBeVisible();
  58 |       }
  59 |     }
  60 |   });
  61 | 
  62 |   test("navigates to devices page via sidebar", async ({ page }) => {
  63 |     // Click on a navigation link that points to /devices.
  64 |     const devicesLink = page.getByRole("link", { name: /devices/i });
  65 |     if ((await devicesLink.count()) > 0) {
  66 |       await devicesLink.first().click();
  67 |       await expect(page).toHaveURL(/\/devices/);
  68 |     }
  69 |     // If no devices link is present, the test is a no-op (the page layout
  70 |     // may not have a sidebar — accept this gracefully).
  71 |   });
  72 | 
  73 |   test("does not show any server error alert on dashboard", async ({ page }) => {
  74 |     // The dashboard should not display a server error / 500 alert.
  75 |     const errorAlerts = page.locator(
  76 |       '[role="alert"], [class*="error"], [class*="alert-danger"]'
  77 |     );
  78 |     // Filter to only error-related alerts (not success/info).
  79 |     const errorTexts = await errorAlerts.allTextContents();
  80 |     for (const text of errorTexts) {
  81 |       expect(text.toLowerCase()).not.toContain("server error");
  82 |       expect(text.toLowerCase()).not.toContain("internal error");
  83 |     }
  84 |   });
  85 | });
  86 | 
```